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

// BindCommand returns the cobra command for `vx build`; the root logger is a pointer because PersistentPreRunE replaces its pointee after wiring.
func BindCommand(cfg *velox.Config, out *string, rootLog *slog.Logger) *cobra.Command {
	return &cobra.Command{
		Use:   "build",
		Short: "Build a custom RoadRunner binary using velox.toml",
		RunE: func(cmd *cobra.Command, _ []string) error {
			log := rootLog.With("component", "builder")
			ref := cfg.Roadrunner[velox.RefKey]

			plugins := make([]*plugin.Plugin, 0, len(cfg.Plugins))
			for _, p := range cfg.Plugins {
				plugins = append(plugins, plugin.NewPlugin(p.ModuleName, p.Tag))
			}

			token := ""
			if cfg.GitHub.Token != nil {
				token = cfg.GitHub.Token.Token
			}

			ctx := cmd.Context()
			gh := github.NewClient(cfg.GitHub.BaseURL, token, log.With("component", "github"))

			// The download dir holds the source tree and the zip, so remove it once the build finishes.
			dlDir, err := os.MkdirTemp("", "velox-build-*")
			if err != nil {
				return err
			}
			defer func() { _ = os.RemoveAll(dlDir) }()

			rrPath, err := gh.DownloadTemplate(ctx, dlDir, ref)
			if err != nil {
				log.Error("downloading template", "error", err)
				return err
			}

			debug := cfg.Debug != nil && cfg.Debug.Enabled
			race := cfg.Debug != nil && cfg.Debug.Race
			binaryPath, err := builder.NewBuilder(rrPath,
				builder.WithLogger(log.With("component", "build")),
				builder.WithPlugins(plugins...),
				builder.WithReplaces(cfg.Replaces),
				builder.WithExcludes(cfg.Excludes),
				builder.WithOutputDir(*out),
				builder.WithRRVersion(ref),
				builder.WithGOOS(cfg.TargetPlatform.OS),
				builder.WithGOARCH(cfg.TargetPlatform.Arch),
				builder.WithDebug(debug),
				builder.WithRace(race),
			).Build(ctx)
			if err != nil {
				log.Error("build failed", "error", err)
				return err
			}

			log.Info("build finished", "path", binaryPath)
			return nil
		},
	}
}
