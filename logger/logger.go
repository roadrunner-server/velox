package logger

import (
	"log/slog"
	"os"
	"strings"
)

// Mode represents available logger modes
const (
	production  string = "production"
	development string = "development"
)

// BuildLogger converts config into Zap configuration.
func BuildLogger(level, mode string) *slog.Logger {
	switch mode {
	case production:
		lg := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			AddSource: false,
			Level:     stringToSlogLevel(level),
		})

		return slog.New(lg)
	case development:
		lg := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			AddSource: false,
			Level:     stringToSlogLevel(level),
		})
		return slog.New(lg)
	default:
		return slog.Default()
	}
}

func stringToSlogLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	case "fatal":
		return slog.LevelError
	case "panic":
		return slog.LevelError
	default:
		return slog.LevelDebug
	}
}
