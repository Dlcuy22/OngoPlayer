// Package logging provides custom console logging handlers with support for
// unified time formatting, ANSI colors, and multiline log readability.
//
// Key Components:
//   - ConsoleHandler: Custom slog.Handler that formats logs with ANSI colors
//   - NewLogger(): Creates a new scoped logger
//
// Dependencies:
//   - log/slog: Structured logging standard library
//
// Example:
//   log := logging.NewLogger("my-subsystem")
//   log.Info("Hello, world")
package logging

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
)

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
	colorGray   = "\033[90m"
)

var level = slog.LevelInfo

type ConsoleHandler struct {
	w      io.Writer
	level  slog.Level
	attrs  []slog.Attr
	group  string
	subsys string
	mu     *sync.Mutex
}

func NewConsoleHandler(w io.Writer, lvl slog.Level, subsys string) *ConsoleHandler {
	return &ConsoleHandler{
		w:      w,
		level:  lvl,
		subsys: subsys,
		mu:     &sync.Mutex{},
	}
}

func (h *ConsoleHandler) Enabled(_ context.Context, l slog.Level) bool {
	return l >= h.level
}

func (h *ConsoleHandler) Handle(_ context.Context, r slog.Record) error {
	var buf bytes.Buffer

	// 1. Time (unified format)
	timeStr := r.Time.Format("2006-01-02 15:04:05.000")
	buf.WriteString(colorGray)
	buf.WriteString(timeStr)
	buf.WriteString(" ")

	// 2. Level with color
	var levelStr string
	var levelColor string
	switch r.Level {
	case slog.LevelDebug:
		levelStr = "DEBUG"
		levelColor = colorCyan
	case slog.LevelInfo:
		levelStr = "INFO"
		levelColor = colorGreen
	case slog.LevelWarn:
		levelStr = "WARN"
		levelColor = colorYellow
	case slog.LevelError:
		levelStr = "ERROR"
		levelColor = colorRed
	default:
		levelStr = r.Level.String()
		levelColor = colorReset
	}
	buf.WriteString(levelColor)
	buf.WriteString(levelStr)
	buf.WriteString(colorReset)
	buf.WriteString(" ")

	// 3. Subsystem
	sub := h.subsys
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "sub" {
			sub = a.Value.String()
		}
		return true
	})
	if sub != "" {
		buf.WriteString(colorPurple)
		buf.WriteString("[")
		buf.WriteString(sub)
		buf.WriteString("]")
		buf.WriteString(colorReset)
		buf.WriteString(" ")
	}

	// 4. Message (Format multiline strings like yt-dlp warnings/errors beautifully)
	msg := r.Message
	if strings.Contains(msg, "\n") {
		buf.WriteString("\n")
		lines := strings.Split(msg, "\n")
		for _, line := range lines {
			trimmed := strings.TrimRight(line, "\r\n")
			if strings.HasPrefix(trimmed, "WARNING:") {
				buf.WriteString(colorYellow)
				buf.WriteString("  | ")
				buf.WriteString(trimmed)
				buf.WriteString(colorReset)
			} else if strings.HasPrefix(trimmed, "ERROR:") {
				buf.WriteString(colorRed)
				buf.WriteString("  | ")
				buf.WriteString(trimmed)
				buf.WriteString(colorReset)
			} else if trimmed != "" {
				buf.WriteString(colorGray)
				buf.WriteString("  | ")
				buf.WriteString(colorReset)
				buf.WriteString(trimmed)
			} else {
				buf.WriteString("  | ")
			}
			buf.WriteString("\n")
		}
		if buf.Len() > 0 {
			buf.Truncate(buf.Len() - 1)
		}
	} else {
		buf.WriteString(msg)
	}

	// 5. Attributes (excluding "sub")
	firstAttr := true
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "sub" {
			return true
		}
		if firstAttr {
			buf.WriteString(" ")
			firstAttr = false
		}
		buf.WriteString(colorGray)
		buf.WriteString(a.Key)
		buf.WriteString("=")
		buf.WriteString(colorReset)
		buf.WriteString(a.Value.String())
		buf.WriteString(" ")
		return true
	})

	for _, a := range h.attrs {
		if a.Key == "sub" {
			continue
		}
		if firstAttr {
			buf.WriteString(" ")
			firstAttr = false
		}
		buf.WriteString(colorGray)
		buf.WriteString(a.Key)
		buf.WriteString("=")
		buf.WriteString(colorReset)
		buf.WriteString(a.Value.String())
		buf.WriteString(" ")
	}

	buf.WriteString("\n")

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.w.Write(buf.Bytes())
	return err
}

func (h *ConsoleHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := append([]slog.Attr(nil), h.attrs...)
	newAttrs = append(newAttrs, attrs...)
	subsys := h.subsys
	for _, attr := range attrs {
		if attr.Key == "sub" {
			subsys = attr.Value.String()
		}
	}
	return &ConsoleHandler{
		w:      h.w,
		level:  h.level,
		attrs:  newAttrs,
		group:  h.group,
		subsys: subsys,
		mu:     h.mu,
	}
}

func (h *ConsoleHandler) WithGroup(name string) slog.Handler {
	return &ConsoleHandler{
		w:      h.w,
		level:  h.level,
		attrs:  h.attrs,
		group:  name,
		subsys: h.subsys,
		mu:     h.mu,
	}
}

func init() {
	if v := os.Getenv("ONGO_DEBUG"); v != "" {
		level = slog.LevelDebug
	}
	slog.SetDefault(NewLogger(""))
}

func NewLogger(subsystem string) *slog.Logger {
	h := NewConsoleHandler(os.Stderr, level, subsystem)
	return slog.New(h)
}

func NewLoggerWithWriter(w io.Writer, subsystem string) *slog.Logger {
	h := NewConsoleHandler(w, level, subsystem)
	return slog.New(h)
}

func DebugEnabled() bool {
	return level.Level() <= slog.LevelDebug
}

var Default = NewLogger("")
