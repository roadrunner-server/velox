package logger

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// Mode names recognized by BuildLogger.
type Mode string

const (
	None        Mode = "none"
	Off         Mode = "off"
	Production  Mode = "production"
	Development Mode = "development"
	Raw         Mode = "raw"
)

// BuildLogger returns a logger for the mode and level; an unknown mode falls back to development.
func BuildLogger(level, mode string) (*slog.Logger, error) {
	switch Mode(strings.ToLower(mode)) {
	case None, Off:
		return slog.New(slog.DiscardHandler), nil

	case Production:
		lvl, err := parseLevel(level, slog.LevelInfo)
		if err != nil {
			return nil, err
		}
		return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})), nil

	case Raw:
		lvl, err := parseLevel(level, slog.LevelInfo)
		if err != nil {
			return nil, err
		}
		return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level:       lvl,
			ReplaceAttr: dropEverythingButMessage,
		})), nil

	case Development:
		fallthrough
	default:
		lvl, err := parseLevel(level, slog.LevelDebug)
		if err != nil {
			return nil, err
		}
		return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})), nil
	}
}

// parseLevel converts a case-insensitive slog level name to a slog.Level and maps empty input to fallback.
func parseLevel(s string, fallback slog.Level) (slog.Level, error) {
	if s == "" {
		return fallback, nil
	}
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(strings.ToUpper(s))); err != nil {
		return 0, fmt.Errorf("invalid log level %q: %w", s, err)
	}
	return lvl, nil
}

// dropEverythingButMessage strips the time, level, and source attributes so raw mode prints the message alone.
func dropEverythingButMessage(_ []string, a slog.Attr) slog.Attr {
	switch a.Key {
	case slog.TimeKey, slog.LevelKey, slog.SourceKey:
		return slog.Attr{}
	}
	return a
}

// Discard returns a no-op logger without a BuildLogger call.
func Discard() *slog.Logger { return slog.New(slog.DiscardHandler) }
