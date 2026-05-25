// Package logger builds the *slog.Logger used by the CLI and build server.
//
// Four modes are recognized; "none" / "off" are no-ops returning a logger that
// discards every record:
//
//	production  → JSON, default level INFO  (machine-readable)
//	development → text, default level DEBUG (human-readable)
//	raw         → text with only the message (no time/level/source)
//	none/off    → discard everything
//
// `level` (if non-empty) overrides the per-mode default. Accepts the standard
// slog spellings: "debug" / "info" / "warn" / "error" (case-insensitive).
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

// BuildLogger constructs a *slog.Logger for the given mode and level. An empty
// level uses the mode's default. Unrecognized modes fall back to development.
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

// parseLevel converts a slog-style level string ("debug", "info", "warn",
// "error", case-insensitive) to a slog.Level. Empty input returns the default.
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

// dropEverythingButMessage strips time, level, and source attributes so "raw"
// mode prints just the message text — analogous to the original raw zap config.
func dropEverythingButMessage(_ []string, a slog.Attr) slog.Attr {
	switch a.Key {
	case slog.TimeKey, slog.LevelKey, slog.SourceKey:
		return slog.Attr{}
	}
	return a
}

// Discard is a convenience for callers that want a no-op logger without going
// through BuildLogger("", "none").
func Discard() *slog.Logger { return slog.New(slog.DiscardHandler) }
