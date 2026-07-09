package logging

import (
	"io"
	"log/slog"
	"os"
)

var level = slog.LevelInfo

func init() {
	if v := os.Getenv("ONGO_DEBUG"); v != "" {
		level = slog.LevelDebug
	}
}

func NewLogger(subsystem string) *slog.Logger {
	opts := &slog.HandlerOptions{Level: level}
	h := slog.NewTextHandler(os.Stderr, opts)
	if subsystem != "" {
		return slog.New(h).With("sub", subsystem)
	}
	return slog.New(h)
}

func NewLoggerWithWriter(w io.Writer, subsystem string) *slog.Logger {
	opts := &slog.HandlerOptions{Level: level}
	h := slog.NewTextHandler(w, opts)
	if subsystem != "" {
		return slog.New(h).With("sub", subsystem)
	}
	return slog.New(h)
}

func DebugEnabled() bool {
	return level.Level() <= slog.LevelDebug
}

var Default = NewLogger("")
