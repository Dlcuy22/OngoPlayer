// This package is reused from by previous project StellePlayer

package lyrics

import (
	"encoding/json"
	"fmt"
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
	TrackName    string `json:"trackName"`
	ArtistName   string `json:"artistName"`
	AlbumName    string `json:"albumName"`
	SyncedLyrics string `json:"syncedLyrics"`
}

var (
	timestampRegexp = regexp.MustCompile(`\[(\d+):(\d+)\.(\d+)\]\s*(.*)`)
	cleanNameRegexp = regexp.MustCompile(`^\d+\s*-?\s*`)
)

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

func FetchFromAPI(artist, title, album string) (string, error) {
	baseURL := "https://lrclib.net/api/search"
	params := url.Values{}
	params.Add("artist_name", artist)
	params.Add("track_name", title)
	if album != "" && album != "Unknown Album" {
		params.Add("album_name", album)
	}

	content, err := performRequest(baseURL, params)
	if err == nil {
		return content, nil
	}

	// fallback search using q param
	params = url.Values{}
	params.Add("q", title)
	return performRequest(baseURL, params)
}

func performRequest(baseURL string, params url.Values) (string, error) {
	resp, err := http.Get(baseURL + "?" + params.Encode())
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status: %s", resp.Status)
	}

	var results []lrclibResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return "", err
	}

	for _, result := range results {
		if result.SyncedLyrics != "" {
			return result.SyncedLyrics, nil
		}
	}

	return "", fmt.Errorf("no synced lyrics found")
}
