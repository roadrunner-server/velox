// Package build provides a command to build the RoadRunner binary
package build

import (
	"os"
	"runtime"
	"syscall"

	"github.com/google/uuid"
	"github.com/roadrunner-server/velox/v2025"
	"github.com/roadrunner-server/velox/v2025/builder"
	cacheimpl "github.com/roadrunner-server/velox/v2025/cache"
	"github.com/roadrunner-server/velox/v2025/github"
	"github.com/roadrunner-server/velox/v2025/plugin"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

const (
	ref string = "ref"
)

func BindCommand(cfg *velox.Config, out *string, zlog *zap.Logger) *cobra.Command {
	return &cobra.Command{
		Use:   "build",
		Short: "Build RR",
		RunE: func(_ *cobra.Command, _ []string) error {
			if *out == "." {
				wd, err := syscall.Getwd()
				if err != nil {
					return err
				}
				*out = wd
			}

			bplugins := make([]*plugin.Plugin, 0, len(cfg.Plugins))
			for _, p := range cfg.Plugins {
				if p == nil {
					zlog.Warn("plugin info is nil")
					continue
				}
				bplugins = append(bplugins, plugin.NewPlugin(p.ModuleName, p.Tag))
			}

			rrcache := cacheimpl.NewRRCache()
			rp := github.NewHTTPClient(os.Getenv("GITHUB_TOKEN"), rrcache, zlog.Named("GitHub"))
			path, err := rp.DownloadTemplate(os.TempDir(), uuid.NewString(), cfg.Roadrunner[ref])
			if err != nil {
				zlog.Error("downloading template", zap.Error(err))
				os.Exit(1)
			}

			// Use target platform or host platform as default
			targetOS := runtime.GOOS
			targetArch := runtime.GOARCH
			if cfg.TargetPlatform != nil {
				if cfg.TargetPlatform.OS != "native" {
					targetOS = cfg.TargetPlatform.OS
				}
				if cfg.TargetPlatform.Arch != "native" {
					targetArch = cfg.TargetPlatform.Arch
				}
			}

			opts := make([]builder.Option, 0)
			opts = append(opts,
				builder.WithPlugins(bplugins...),
				builder.WithOutputDir(*out),
				builder.WithRRVersion(cfg.Roadrunner[ref]),
				builder.WithLogger(zlog.Named("Builder")),
				builder.WithGOOS(targetOS),
				builder.WithGOARCH(targetArch),
			)

			binaryPath, err := builder.NewBuilder(path, opts...).Build(cfg.Roadrunner[ref])
			if err != nil {
				zlog.Error("fatal", zap.Error(err))
				os.Exit(1)
			}

			zlog.Info("build finished successfully", zap.String("path", binaryPath))
			return nil
		},
	}
}
