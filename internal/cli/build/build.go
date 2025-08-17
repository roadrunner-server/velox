package build

import (
	"os"
	"syscall"

	"github.com/roadrunner-server/velox/v2025"
	"github.com/roadrunner-server/velox/v2025/builder"
	"github.com/roadrunner-server/velox/v2025/github"
	"github.com/roadrunner-server/velox/v2025/gitlab"
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

			var mi []*velox.ModulesInfo
			if cfg.GitLab != nil {
				rp, err := gitlab.NewGLRepoInfo(cfg, zlog.Named("GITLAB"))
				if err != nil {
					return err
				}

				mi, err = rp.GetPluginsModData()
				if err != nil {
					return err
				}
			}

			// roadrunner located on the github
			rp := github.NewGHRepoInfo(cfg, zlog.Named("GITHUB"))
			path, err := rp.DownloadTemplate(os.TempDir(), cfg.Roadrunner[ref])
			if err != nil {
				zlog.Error("downloading template", zap.Error(err))
				os.Exit(1)
			}

			pMod, err := rp.GetPluginsModData()
			if err != nil {
				zlog.Error("get plugins mod data", zap.Error(err))
				os.Exit(1)
			}

			// append data from gitlab
			if mi != nil {
				pMod = append(pMod, mi...)
			}

			err = builder.NewBuilder(path, pMod,
				builder.WithOutputDir(*out),
				builder.WithRRVersion(cfg.Roadrunner[ref]),
				builder.WithDebug(cfg.Debug.Enabled),
				builder.WithLogger(zlog.Named("Builder")),
			).Build(cfg.Roadrunner[ref])
			if err != nil {
				zlog.Error("fatal", zap.Error(err))
				os.Exit(1)
			}

			zlog.Info("========= build finished successfully =========", zap.String("RoadRunner binary can be found at", *out))
			return nil
		},
	}
}
