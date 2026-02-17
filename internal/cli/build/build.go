package build

import (
	"os"
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

// BindCommand creates the cobra command for building RoadRunner binary with configured plugins.
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
			for name, p := range cfg.Plugins {
				if p == nil {
					zlog.Warn("plugin info is nil", zap.String("name", name))
					continue
				}

				bplugins = append(bplugins, plugin.NewPlugin(p.ModuleName, p.Tag))
			}

			// init out simple cache
			rrcache := cacheimpl.NewRRCache()
			// we can use a GITHUB token to download templates, but it's not required
			rp := github.NewHTTPClient(os.Getenv("GITHUB_TOKEN"), rrcache, zlog.Named("GitHub"))
			// Download the template for the specified RoadRunner version
			path, err := rp.DownloadTemplate(os.TempDir(), uuid.NewString(), cfg.Roadrunner[ref])
			if err != nil {
				zlog.Error("downloading template", zap.Error(err))
				return err
			}

			opts := make([]builder.Option, 0, 6)
			opts = append(opts,
				builder.WithPlugins(bplugins...),
				builder.WithOutputDir(*out),
				builder.WithRRVersion(cfg.Roadrunner[ref]),
				builder.WithLogger(zlog.Named("Builder")),
				builder.WithGOOS(cfg.TargetPlatform.OS),
				builder.WithGOARCH(cfg.TargetPlatform.Arch),
			)

			binaryPath, err := builder.NewBuilder(path, opts...).Build(cfg.Roadrunner[ref])
			if err != nil {
				zlog.Error("fatal", zap.Error(err))
				return err
			}

			zlog.Info("build finished successfully", zap.String("path", binaryPath))
			return nil
		},
	}
}
