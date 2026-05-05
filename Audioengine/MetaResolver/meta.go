// Audioengine/MetaResolver/meta.go
// Provides shared audio track metadata types and directory scanning
// utilities for all OngoPlayer UI frontends (Gio, WebUI, TUI).
//
// Key Types:
//   - TrackMeta: UI-agnostic track metadata with raw cover art bytes
//
// Key Functions:
//   - ScanFolder(): scans a directory for supported audio files
//   - ResolveTrack(): reads metadata from a single audio file
//
// Dependencies:
//   - dhowden/tag: embedded metadata extraction (ID3, Vorbis, FLAC)
//
// Supported Formats:
//   .opus, .mp3, .flac, .ogg, .oga

package MetaResolver

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/dhowden/tag"
)

type TrackMeta struct {
	Path      string `json:"path"`
	Title     string `json:"title"`
	Artist    string `json:"artist"`
	Album     string `json:"album"`
	Format    string `json:"format"`
	CoverData []byte `json:"-"` // raw cover art bytes (JPEG/PNG), excluded from JSON
}

var supportedExts = map[string]bool{
	".opus": true,
	".mp3":  true,
	".ogg":  true,
	".oga":  true,
	".flac": true,
}

/*
ScanFolder reads a directory and returns metadata for all supported audio files.

	params:
	      folder: absolute path to the music directory
	returns:
	      []TrackMeta: metadata for each audio file found
	      error
*/
func ScanFolder(folder string) ([]TrackMeta, error) {
	entries, err := os.ReadDir(folder)
	if err != nil {
		return nil, err
	}

	var tracks []TrackMeta
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if !supportedExts[ext] {
			continue
		}

		fullPath := filepath.Join(folder, e.Name())
		meta := ResolveTrack(fullPath)
		tracks = append(tracks, meta)
	}

	return tracks, nil
}

/*
ResolveTrack reads metadata from a single audio file.

	params:
	      path: absolute path to the audio file
	returns:
	      TrackMeta: populated metadata (falls back to filename if tags are missing)
*/
func ResolveTrack(path string) TrackMeta {
	ext := strings.ToLower(filepath.Ext(path))
	meta := TrackMeta{
		Path:  path,
		Title: filepath.Base(path),
	}

	meta.Format = resolveFormat(ext)

	f, err := os.Open(path)
	if err != nil {
		return meta
	}
	defer f.Close()

	m, err := tag.ReadFrom(f)
	if err != nil {
		return meta
	}

	if t := m.Title(); t != "" {
		meta.Title = t
	}
	meta.Artist = m.Artist()
	meta.Album = m.Album()

	if pic := m.Picture(); pic != nil {
		meta.CoverData = pic.Data
	}

	return meta
}

/*
IsSupportedExt checks if a file extension is a supported audio format.

	params:
	      ext: file extension including the dot (e.g. ".mp3")
	returns:
	      bool
*/
func IsSupportedExt(ext string) bool {
	return supportedExts[strings.ToLower(ext)]
}

func resolveFormat(ext string) string {
	switch ext {
	case ".opus":
		return "OPUS"
	case ".mp3":
		return "MP3"
	case ".flac":
		return "FLAC"
	case ".ogg", ".oga":
		return "OGG"
	default:
		return "unknown"
	}
}
