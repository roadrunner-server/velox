// Package build implements the `vx build` subcommand: read velox.toml, download
// the upstream RoadRunner source, and produce a custom binary via the Builder.
package build

import (
	"os"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/roadrunner-server/velox/v3"
	"github.com/roadrunner-server/velox/v3/builder"
	"github.com/roadrunner-server/velox/v3/github"
	"github.com/roadrunner-server/velox/v3/plugin"
)

const refKey = "ref"

// BindCommand returns the cobra.Command for `vx build`.
func BindCommand(cfg *velox.Config, out *string, zlog *zap.Logger) *cobra.Command {
	return &cobra.Command{
		Use:   "build",
		Short: "Build a custom RoadRunner binary using velox.toml",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if *out == "." {
				wd, err := os.Getwd()
				if err != nil {
					return err
				}
				*out = wd
			}

			plugins := make([]*plugin.Plugin, 0, len(cfg.Plugins))
			for name, p := range cfg.Plugins {
				if p == nil {
					zlog.Warn("plugin info is nil", zap.String("name", name))
					continue
				}
				plugins = append(plugins, plugin.NewPlugin(p.ModuleName, p.Tag))
			}

			token := ""
			if cfg.GitHub != nil && cfg.GitHub.Token != nil {
				token = cfg.GitHub.Token.Token
			}
			baseURL := ""
			if cfg.GitHub != nil {
				baseURL = cfg.GitHub.BaseURL
			}

			ctx := cmd.Context()
			gh := github.NewClient(baseURL, token, github.NewLRUCache(0), zlog.Named("GitHub"))
			rrPath, err := gh.DownloadTemplate(ctx, os.TempDir(), uuid.NewString(), cfg.Roadrunner[refKey])
			if err != nil {
				zlog.Error("downloading template", zap.Error(err))
				return err
			}

			debug := cfg.Debug != nil && cfg.Debug.Enabled
			binaryPath, err := builder.NewBuilder(rrPath,
				builder.WithLogger(zlog.Named("Builder")),
				builder.WithPlugins(plugins...),
				builder.WithReplaces(cfg.Replaces),
				builder.WithExcludes(cfg.Excludes),
				builder.WithOutputDir(*out),
				builder.WithRRVersion(cfg.Roadrunner[refKey]),
				builder.WithGOOS(cfg.TargetPlatform.OS),
				builder.WithGOARCH(cfg.TargetPlatform.Arch),
				builder.WithDebug(debug),
			).Build(ctx, cfg.Roadrunner[refKey])
			if err != nil {
				zlog.Error("build failed", zap.Error(err))
				return err
			}

			zlog.Info("build finished", zap.String("path", binaryPath))
			return nil
		},
	}
}
