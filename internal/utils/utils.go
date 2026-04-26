// Package utils provides common utility functions used across the OngoPlayer application.
//
// Key Functions:
//   - FormatTime(): Converts seconds into a formatted MM:SS string.
//   - RenderBar(): Generates an ASCII progress bar for TUI usage.
//   - ScanFolder(): Scans a directory for supported audio files.
//   - Hex(): Parses a hex color string into a Gio-compatible color.NRGBA.
//
// Dependencies:
//   - image/color: Used for color representation in Gio.
//   - os, path/filepath: Used for directory traversal and file path manipulation.
//
// Example:
//   color := utils.Hex("#ff0000")
//   files, err := utils.ScanFolder("/music", map[string]bool{".mp3": true})
package utils

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

/*
FormatTime converts a duration in seconds into a formatted MM:SS string.

	params:
	      secs: The duration in seconds.
	returns:
	      string: Formatted time string (e.g., "3:05").
*/
func FormatTime(secs float64) string {
	s := int(secs)
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

/*
RenderBar generates an ASCII progress bar representing playback position.
Primarily used for CLI or TUI output.

	params:
	      pos: Current playback position in seconds.
	      dur: Total track duration in seconds.
	      barWidth: The character width of the progress bar.
	returns:
	      string: The rendered ASCII progress bar.
*/
func RenderBar(pos, dur float64, barWidth int) string {
	if dur <= 0 {
		return fmt.Sprintf("\r[%s] %s / %s", strings.Repeat("-", barWidth), FormatTime(pos), "?:??")
	}

	ratio := pos / dur
	if ratio > 1 {
		ratio = 1
	}

	filled := int(ratio * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}

	var bar string
	if filled == barWidth {
		bar = strings.Repeat("=", barWidth)
	} else if filled > 0 {
		bar = strings.Repeat("=", filled-1) + "==>" + strings.Repeat(" ", barWidth-filled)
	} else {
		bar = strings.Repeat(" ", barWidth)
	}

	return fmt.Sprintf("\r[%s] %s / %s   ", bar, FormatTime(pos), FormatTime(dur))
}

/*
ScanFolder recursively scans a directory and extracts files that match supported extensions.

	params:
	      dir: The directory path to scan.
	      supportedExts: A map of valid file extensions (e.g., {".mp3": true}).
	returns:
	      []string: A list of absolute file paths matching the extensions.
	      error: Returns if reading the directory fails.
*/
func ScanFolder(dir string, supportedExts map[string]bool) ([]string, error) {
	var queue []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("cannot read folder: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if supportedExts[strings.ToLower(filepath.Ext(e.Name()))] {
			queue = append(queue, filepath.Join(dir, e.Name()))
		}
	}
	return queue, nil
}

/*
Hex parses a hexadecimal color string into an NRGBA color object used by Gio UI.

	params:
	      hex: A hex string, either 6 characters (#RRGGBB) or 8 characters (#RRGGBBAA).
	           The leading "#" is optional.
	returns:
	      color.NRGBA: The color object representing the hex string.
*/
func Hex(hex string) color.NRGBA {
	hex = strings.TrimPrefix(hex, "#")

	var r, g, b, a uint8
	a = 0xFF

	switch len(hex) {
	case 6: // RRGGBB
		val, _ := strconv.ParseUint(hex, 16, 32)
		r = uint8(val >> 16)
		g = uint8(val >> 8)
		b = uint8(val)
	case 8: // RRGGBBAA
		val, _ := strconv.ParseUint(hex, 16, 32)
		r = uint8(val >> 24)
		g = uint8(val >> 16)
		b = uint8(val >> 8)
		a = uint8(val)
	}

	return color.NRGBA{R: r, G: g, B: b, A: a}
}
