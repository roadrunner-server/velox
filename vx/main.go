package main

import (
	"flag"
	"log"
	"strings"

	"github.com/fatih/color"
	"github.com/roadrunner-server/velox"
	"github.com/roadrunner-server/velox/build"
	"github.com/roadrunner-server/velox/github"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Mode represents available logger modes
const (
	none        string = "none"
	off         string = "off"
	production  string = "production"
	development string = "development"
	raw         string = "raw"
)

func main() {
	var cfg *velox.Config

	pathToConfig := flag.String("config", "plugins.toml", "Path to the velox configuration file with plugins")
	out := flag.String("out", "rr", "Output filename (might be with the path)")

	flag.Parse()

	// the user doesn't provide a path to the config
	if pathToConfig == nil {
		log.Fatalf("path to the config should be provided")
	}

	v := viper.New()
	v.SetConfigFile(*pathToConfig)
	err := v.ReadInConfig()
	if err != nil {
		log.Fatal(err)
	}

	err = v.Unmarshal(&cfg)
	if err != nil {
		log.Fatal(err)
	}

	err = cfg.Validate()
	if err != nil {
		log.Fatal(err)
	}

	// [log]
	// level = "debug"
	// mode = "development"
	zlog, err := BuildLogger(cfg.Log["level"], cfg.Log["mode"])
	if err != nil {
		log.Fatal(err)
	}

	defer func() {
		_ = zlog.Sync()
	}()

	rp := github.NewRepoInfo(cfg, zlog)
	path, err := rp.DownloadTemplate(cfg.Roadrunner["ref"])
	if err != nil {
		zlog.Fatal("[DOWNLOAD TEMPLATE]", zap.Error(err))
	}

	pMod, err := rp.GetPluginsModData()
	if err != nil {
		zlog.Fatal("[PLUGINS GET MOD INFO]", zap.Error(err))
	}

	builder := build.NewBuilder(path, pMod, *out, zlog, cfg.Velox["build_args"])

	err = builder.Build()
	if err != nil {
		zlog.Fatal("[BUILD FAILED]", zap.Error(err))
	}

	zlog.Info("[BUILD]", zap.String("build finished, path", *out))
}

// BuildLogger converts config into Zap configuration.
func BuildLogger(level, mode string) (*zap.Logger, error) {
	var zCfg zap.Config
	switch mode {
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
				EncodeTime:     zapcore.EpochTimeEncoder,
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
				EncodeTime:     zapcore.ISO8601TimeEncoder,
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
				MessageKey:  "message",
				EncodeLevel: ColoredLevelEncoder,
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
				EncodeTime:     zapcore.ISO8601TimeEncoder,
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
	case zapcore.PanicLevel, zapcore.FatalLevel:
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
