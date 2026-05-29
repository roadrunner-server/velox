// Package build implements the `vx build` subcommand: read velox.toml, download
// the upstream RoadRunner source, and produce a custom binary via the Builder.
package build

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/roadrunner-server/velox/v3"
	"github.com/roadrunner-server/velox/v3/builder"
	"github.com/roadrunner-server/velox/v3/github"
	"github.com/roadrunner-server/velox/v3/plugin"
)

const refKey = "ref"

// BindCommand returns the cobra.Command for `vx build`. The root *slog.Logger
// is passed by pointer because the root command's PersistentPreRunE rewrites
// its pointee with the config-driven logger after construction; child loggers
// are therefore derived inside RunE, not at wiring time.
func BindCommand(cfg *velox.Config, out *string, rootLog *slog.Logger) *cobra.Command {
	return &cobra.Command{
		Use:   "build",
		Short: "Build a custom RoadRunner binary using velox.toml",
		RunE: func(cmd *cobra.Command, _ []string) error {
			log := rootLog.With("component", "builder")

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
					log.Warn("plugin info is nil", "name", name)
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
			gh := github.NewClient(baseURL, token, github.NewLRUCache(0), log.With("component", "github"))

			// Download into a unique per-build temp dir and remove it once the
			// build finishes. The builder's own cleanup only sweeps the output
			// dir, which differs from this download dir in CLI mode — without
			// this defer the RR source tree + zip would leak into TempDir.
			dlDir, err := os.MkdirTemp("", "velox-build-*")
			if err != nil {
				return err
			}
			defer func() { _ = os.RemoveAll(dlDir) }()

			rrPath, err := gh.DownloadTemplate(ctx, dlDir, "", cfg.Roadrunner[refKey])
			if err != nil {
				log.Error("downloading template", "error", err)
				return err
			}

			debug := cfg.Debug != nil && cfg.Debug.Enabled
			binaryPath, err := builder.NewBuilder(rrPath,
				builder.WithLogger(log.With("component", "build")),
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
				log.Error("build failed", "error", err)
				return err
			}

			log.Info("build finished", "path", binaryPath)
			return nil
		},
	}
}
