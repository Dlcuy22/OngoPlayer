// internal/service/lyrics/lyrics.go
// Lyrics fetching, parsing, and caching service.
// Supports local .lrc file loading and remote fetching from lrclib.net API.
//
// Fetch Strategy (FetchFromAPI):
//   1. Try /api/get with exact artist, title, album, duration for a precise match.
//   2. Fallback to /api/search with artist_name + track_name, then score results
//      by artist/title match and duration proximity. Only accept results that
//      match both artist and title (case-insensitive).
//
// Dependencies:
//   - encoding/json, net/http, net/url: HTTP + JSON for lrclib.net
//   - math: duration proximity scoring
//   - regexp, strconv, strings: LRC timestamp parsing

package lyrics

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type Line struct {
	Time float64
	Text string
}

type Lyrics struct {
	Lines  []Line
	Loaded bool
}

type lrclibResult struct {
	TrackName    string  `json:"trackName"`
	ArtistName   string  `json:"artistName"`
	AlbumName    string  `json:"albumName"`
	Duration     float64 `json:"duration"`
	SyncedLyrics string  `json:"syncedLyrics"`
}

type lrclibGetResult struct {
	SyncedLyrics string `json:"syncedLyrics"`
}

var (
	timestampRegexp = regexp.MustCompile(`\[(\d+):(\d+)\.(\d+)\]\s*(.*)`)
	cleanNameRegexp = regexp.MustCompile(`^\d+\s*-?\s*`)
)

/*
Parse converts raw LRC content into a sorted slice of timestamped lines.

	params:
	      content: raw LRC string with [mm:ss.cs] timestamps
	returns:
	      []Line: sorted by time
*/
func Parse(content string) []Line {
	var lines []Line

	for _, line := range strings.Split(content, "\n") {
		matches := timestampRegexp.FindStringSubmatch(line)
		if len(matches) != 5 {
			continue
		}

		min, _ := strconv.Atoi(matches[1])
		sec, _ := strconv.Atoi(matches[2])
		ms, _ := strconv.Atoi(matches[3])

		timeInSeconds := float64(min*60) + float64(sec) + float64(ms)/100.0
		text := strings.TrimSpace(matches[4])

		if text == "" {
			continue
		}

		lines = append(lines, Line{
			Time: timeInSeconds,
			Text: text,
		})
	}

	sort.Slice(lines, func(i, j int) bool {
		return lines[i].Time < lines[j].Time
	})

	return lines
}

/*
LoadFromFile attempts to load lyrics from a local .lrc file.
Searches in a "lyrics" subdirectory of musicDir with multiple
filename patterns (clean name, base name, lowercase variants).

	params:
	      songPath: full path to the audio file
	      musicDir: root music directory containing the "lyrics" folder
	returns:
	      Lyrics: parsed lyrics struct
	      bool:   true if lyrics were found and parsed
*/
func LoadFromFile(songPath, musicDir string) (Lyrics, bool) {
	baseName := strings.TrimSuffix(filepath.Base(songPath), filepath.Ext(songPath))
	cleanName := cleanNameRegexp.ReplaceAllString(baseName, "")

	lyricsDir := filepath.Join(musicDir, "lyrics")
	if err := os.MkdirAll(lyricsDir, 0755); err != nil {
		return Lyrics{}, false
	}

	patterns := []string{
		filepath.Join(lyricsDir, cleanName+".lrc"),
		filepath.Join(lyricsDir, baseName+".lrc"),
		filepath.Join(lyricsDir, strings.ToLower(cleanName)+".lrc"),
		filepath.Join(lyricsDir, strings.ToLower(baseName)+".lrc"),
	}

	for _, candidate := range patterns {
		content, err := os.ReadFile(candidate)
		if err != nil {
			continue
		}

		lines := Parse(string(content))
		if len(lines) == 0 {
			continue
		}

		return Lyrics{Lines: lines, Loaded: true}, true
	}

	return Lyrics{}, false
}

/*
SaveToFile persists raw LRC content to the "lyrics" subdirectory.

	params:
	      songPath: full path to the audio file (used for filename derivation)
	      musicDir: root music directory
	      content:  raw LRC string to save
	returns:
	      string: path where the file was saved
	      error
*/
func SaveToFile(songPath, musicDir, content string) (string, error) {
	baseName := strings.TrimSuffix(filepath.Base(songPath), filepath.Ext(songPath))
	cleanName := cleanNameRegexp.ReplaceAllString(baseName, "")

	lyricsDir := filepath.Join(musicDir, "lyrics")
	if err := os.MkdirAll(lyricsDir, 0755); err != nil {
		return "", err
	}

	path := filepath.Join(lyricsDir, cleanName+".lrc")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", err
	}

	return path, nil
}

/*
cleanMetadataStrings cleans metadata strings (artist, title) from common
junk suffixes like " - Topic" and splits dual-language naming (e.g. "なとり / natori") and more
It returns a slice of potential search aliases.

	params:
	      s: raw string
	returns:
	      []string: list of cleaned aliases
*/
func cleanMetadataStrings(s string) []string {
	// 1. Remove common YouTube/Auto-gen suffixes
	s = strings.ReplaceAll(s, " - Topic", "")
	s = strings.ReplaceAll(s, " official YouTube channel", "")
	s = strings.ReplaceAll(s, "VEVO", "")
	s = strings.ReplaceAll(s, " official", "")
	s = strings.TrimSpace(s)

	if s == "" {
		return nil
	}

	// 2. If it contains a slash or double-space, it's often dual language (e.g. "なとり / natori", "Kenshi Yonezu  米津玄師")
	normalized := strings.ReplaceAll(s, "  ", "/")
	if strings.Contains(normalized, "/") {
		var aliases []string
		parts := strings.Split(normalized, "/")
		for _, p := range parts {
			cleaned := strings.TrimSpace(p)
			if cleaned != "" {
				aliases = append(aliases, cleaned)
			}
		}
		if len(aliases) > 0 {
			return aliases
		}
	}

	return []string{s}
}

/*
FetchFromAPI fetches synced lyrics from lrclib.net.

Strategy:

 1. Try the /api/get endpoint with exact artist, title, album, and duration
    for a precise single-result match. Iterates over cleaned aliases.

 2. If that fails, fall back to /api/search with artist_name + track_name,
    then score and filter results by artist/title similarity and duration proximity.

    params:
    artist:      artist name from file metadata
    title:       track title from file metadata
    album:       album name from file metadata
    durationSec: track duration in seconds (from the audio engine)
    returns:
    string: raw synced LRC content
    error
*/
func FetchFromAPI(artist, title, album string, durationSec float64) (string, error) {
	artistAliases := cleanMetadataStrings(artist)
	if len(artistAliases) == 0 {
		return "", fmt.Errorf("artist name is empty after cleaning")
	}

	titleAliases := cleanMetadataStrings(title)
	if len(titleAliases) == 0 {
		return "", fmt.Errorf("title is empty after cleaning")
	}

	// Step 1: Try /api/get for exact match over all alias combinations
	if durationSec > 0 {
		for _, cleanArtist := range artistAliases {
			for _, cleanTitle := range titleAliases {
				content, err := tryGetEndpoint(cleanArtist, cleanTitle, album, int(math.Round(durationSec)))
				if err == nil && content != "" {
					return content, nil
				}
			}
		}
	}

	// Step 2: Fallback to /api/search over all alias combinations
	for _, cleanArtist := range artistAliases {
		for _, cleanTitle := range titleAliases {
			content, err := trySearchEndpoint(cleanArtist, cleanTitle, album, durationSec)
			if err == nil && content != "" {
				return content, nil
			}
		}
	}

	return "", fmt.Errorf("no synced lyrics found for %q by %q", title, artist)
}

/*
tryGetEndpoint queries lrclib.net /api/get for an exact match.

	params:
	      artist, title, album: metadata fields
	      duration:             track duration in whole seconds
	returns:
	      string: synced lyrics content, or empty
	      error
*/
func tryGetEndpoint(artist, title, album string, duration int) (string, error) {
	params := url.Values{}
	params.Add("artist_name", artist)
	params.Add("track_name", title)
	if album != "" && album != "Unknown Album" {
		params.Add("album_name", album)
	}
	params.Add("duration", strconv.Itoa(duration))

	reqURL := "https://lrclib.net/api/get?" + params.Encode()
	resp, err := http.Get(reqURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("api/get returned %s", resp.Status)
	}

	var result lrclibGetResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if result.SyncedLyrics != "" {
		return result.SyncedLyrics, nil
	}

	return "", fmt.Errorf("no synced lyrics in /api/get response")
}

/*
trySearchEndpoint queries lrclib.net /api/search and scores results.
Only results where both artist and title match (case-insensitive) are
considered. Among matches, the result with the closest duration wins.

	params:
	      artist, title, album: metadata fields
	      durationSec:          track duration in seconds (0 if unknown)
	returns:
	      string: best matching synced lyrics content
	      error
*/
func trySearchEndpoint(artist, title, album string, durationSec float64) (string, error) {
	params := url.Values{}
	params.Add("artist_name", artist)
	params.Add("track_name", title)
	if album != "" && album != "Unknown Album" {
		params.Add("album_name", album)
	}

	reqURL := "https://lrclib.net/api/search?" + params.Encode()
	resp, err := http.Get(reqURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("api/search returned %s", resp.Status)
	}

	var results []lrclibResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return "", err
	}

	var candidates []lrclibResult
	for _, r := range results {
		if r.SyncedLyrics == "" {
			continue
		}

		// Due to the use of aliases (e.g. querying "natori" but r.ArtistName is "なとり"),
		// it checks if ANY of the aliases are contained within the result's artist name,
		// or vice-versa, to maintain leniency while ensuring safety.
		artistMatch := false
		rArtist := strings.ToLower(r.ArtistName)
		artistAliases := cleanMetadataStrings(artist)
		for _, a := range artistAliases {
			lowerA := strings.ToLower(a)
			if strings.Contains(rArtist, lowerA) || strings.Contains(lowerA, rArtist) {
				artistMatch = true
				break
			}
		}
		if !artistMatch {
			continue
		}

		// Leniency on title match
		titleMatch := false
		rTitle := strings.ToLower(r.TrackName)
		titleAliases := cleanMetadataStrings(title)
		for _, t := range titleAliases {
			lowerT := strings.ToLower(t)
			if strings.Contains(rTitle, lowerT) || strings.Contains(lowerT, rTitle) {
				titleMatch = true
				break
			}
		}
		if !titleMatch {
			continue
		}

		candidates = append(candidates, r)
	}

	if len(candidates) == 0 {
		return "", fmt.Errorf("no matching synced lyrics found")
	}

	// If there is a known duration, pick the candidate with closest match
	if durationSec > 0 && len(candidates) > 1 {
		sort.Slice(candidates, func(i, j int) bool {
			diffI := math.Abs(candidates[i].Duration - durationSec)
			diffJ := math.Abs(candidates[j].Duration - durationSec)
			return diffI < diffJ
		})
	}

	return candidates[0].SyncedLyrics, nil
}
