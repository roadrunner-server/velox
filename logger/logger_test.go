package logger

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildLogger(t *testing.T) {
	cases := []struct {
		name     string
		level    string
		mode     string
		wantJSON bool
		enabled  slog.Level
		disabled slog.Level
	}{
		{"production defaults to info", "", "production", true, slog.LevelInfo, slog.LevelDebug},
		{"production is case insensitive", "", "PRODUCTION", true, slog.LevelInfo, slog.LevelDebug},
		{"raw defaults to info", "", "raw", false, slog.LevelInfo, slog.LevelDebug},
		{"development defaults to debug", "", "development", false, slog.LevelDebug, slog.LevelDebug - 1},
		{"unknown mode falls back to development", "", "bogus", false, slog.LevelDebug, slog.LevelDebug - 1},
		{"level overrides the mode default", "error", "development", false, slog.LevelError, slog.LevelWarn},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lg, err := BuildLogger(tc.level, tc.mode)
			require.NoError(t, err)

			_, isJSON := lg.Handler().(*slog.JSONHandler)
			require.Equal(t, tc.wantJSON, isJSON)
			require.True(t, lg.Handler().Enabled(t.Context(), tc.enabled))
			require.False(t, lg.Handler().Enabled(t.Context(), tc.disabled))
		})
	}
}

func TestBuildLogger_Off(t *testing.T) {
	for _, mode := range []string{"none", "off", "OFF"} {
		lg, err := BuildLogger("debug", mode)
		require.NoError(t, err)
		require.False(t, lg.Handler().Enabled(t.Context(), slog.LevelError), mode)
	}
}

func TestBuildLogger_InvalidLevel(t *testing.T) {
	_, err := BuildLogger("bogus", "production")
	require.ErrorContains(t, err, "bogus")
}

func TestDropEverythingButMessage(t *testing.T) {
	for _, key := range []string{slog.TimeKey, slog.LevelKey, slog.SourceKey} {
		require.True(t, dropEverythingButMessage(nil, slog.String(key, "x")).Equal(slog.Attr{}), key)
	}
	keep := slog.String(slog.MessageKey, "building rr")
	require.True(t, dropEverythingButMessage(nil, keep).Equal(keep))
}
