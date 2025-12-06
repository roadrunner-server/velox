package logger

import (
	"strings"
	"time"

	"github.com/fatih/color"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Mode represents available logger modes
type Mode string

const (
	none        Mode = "none"
	off         Mode = "off"
	production  Mode = "production"
	development Mode = "development"
	raw         Mode = "raw"
)

// BuildLogger constructs a zap logger with the specified level and mode.
// Supported modes: production (JSON), development (console with colors), raw (message only), none/off (no-op).
// If level is specified, it overrides the default level for the mode.
func BuildLogger(level, mode string) (*zap.Logger, error) {
	var zCfg zap.Config
	switch Mode(strings.ToLower(mode)) {
	case off, none:
		return zap.NewNop(), nil
	case production:
		zCfg = zap.Config{
			Level:       zap.NewAtomicLevelAt(zap.InfoLevel),
			Development: false,
			Encoding:    "json",
			EncoderConfig: zapcore.EncoderConfig{
				TimeKey:        "ts",
				LevelKey:       "level",
				NameKey:        "logger",
				CallerKey:      zapcore.OmitKey,
				FunctionKey:    zapcore.OmitKey,
				MessageKey:     "msg",
				StacktraceKey:  zapcore.OmitKey,
				EncodeLevel:    zapcore.LowercaseLevelEncoder,
				EncodeTime:     utcEpochTimeEncoder,
				EncodeDuration: zapcore.SecondsDurationEncoder,
				EncodeCaller:   zapcore.ShortCallerEncoder,
			},
			OutputPaths:      []string{"stderr"},
			ErrorOutputPaths: []string{"stderr"},
		}
	case development:
		zCfg = zap.Config{
			Level:       zap.NewAtomicLevelAt(zap.DebugLevel),
			Development: true,
			Encoding:    "console",
			EncoderConfig: zapcore.EncoderConfig{
				TimeKey:        "ts",
				LevelKey:       "level",
				NameKey:        "logger",
				CallerKey:      zapcore.OmitKey,
				FunctionKey:    zapcore.OmitKey,
				MessageKey:     "msg",
				StacktraceKey:  zapcore.OmitKey,
				EncodeLevel:    ColoredLevelEncoder,
				EncodeName:     ColoredNameEncoder,
				EncodeTime:     utcISO8601TimeEncoder,
				EncodeDuration: zapcore.StringDurationEncoder,
				EncodeCaller:   zapcore.ShortCallerEncoder,
			},
			OutputPaths:      []string{"stderr"},
			ErrorOutputPaths: []string{"stderr"},
		}
	case raw:
		zCfg = zap.Config{
			Level:    zap.NewAtomicLevelAt(zap.InfoLevel),
			Encoding: "console",
			EncoderConfig: zapcore.EncoderConfig{
				MessageKey: "message",
			},
			OutputPaths:      []string{"stderr"},
			ErrorOutputPaths: []string{"stderr"},
		}
	default:
		zCfg = zap.Config{
			Level:    zap.NewAtomicLevelAt(zap.DebugLevel),
			Encoding: "console",
			EncoderConfig: zapcore.EncoderConfig{
				TimeKey:        "T",
				LevelKey:       "L",
				NameKey:        "N",
				CallerKey:      zapcore.OmitKey,
				FunctionKey:    zapcore.OmitKey,
				MessageKey:     "M",
				StacktraceKey:  zapcore.OmitKey,
				EncodeLevel:    ColoredLevelEncoder,
				EncodeName:     ColoredNameEncoder,
				EncodeTime:     utcISO8601TimeEncoder,
				EncodeDuration: zapcore.StringDurationEncoder,
				EncodeCaller:   zapcore.ShortCallerEncoder,
			},
			OutputPaths:      []string{"stderr"},
			ErrorOutputPaths: []string{"stderr"},
		}
	}

	if level != "" {
		lvl := zap.NewAtomicLevel()
		if err := lvl.UnmarshalText([]byte(level)); err == nil {
			zCfg.Level = lvl
		}
	}

	return zCfg.Build()
}

// ColoredLevelEncoder colorizes log levels.
func ColoredLevelEncoder(level zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
	switch level {
	case zapcore.DebugLevel:
		enc.AppendString(color.HiWhiteString(level.CapitalString()))
	case zapcore.InfoLevel:
		enc.AppendString(color.HiCyanString(level.CapitalString()))
	case zapcore.WarnLevel:
		enc.AppendString(color.HiYellowString(level.CapitalString()))
	case zapcore.ErrorLevel, zapcore.DPanicLevel:
		enc.AppendString(color.HiRedString(level.CapitalString()))
	case zapcore.PanicLevel, zapcore.FatalLevel, zapcore.InvalidLevel:
		enc.AppendString(color.HiMagentaString(level.CapitalString()))
	}
}

// ColoredNameEncoder colorizes service names.
func ColoredNameEncoder(s string, enc zapcore.PrimitiveArrayEncoder) {
	if len(s) < 12 {
		s += strings.Repeat(" ", 12-len(s))
	}

	enc.AppendString(color.HiGreenString(s))
}

// utcEpochTimeEncoder encodes timestamps as UTC Unix epoch time in nanoseconds for structured logging.
func utcEpochTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendInt64(t.UTC().UnixNano())
}

// utcISO8601TimeEncoder encodes timestamps in UTC ISO8601 format for human-readable logs.
func utcISO8601TimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.UTC().Format("2006-01-02T15:04:05-0700"))
}
