package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func FormatTime(secs float64) string {
	s := int(secs)
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

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
