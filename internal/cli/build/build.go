package build

import (
	"log/slog"
	"os"
	"syscall"

	"github.com/roadrunner-server/velox/v2024"
	"github.com/roadrunner-server/velox/v2024/builder"
	"github.com/roadrunner-server/velox/v2024/github"
	"github.com/roadrunner-server/velox/v2024/gitlab"
	"github.com/spf13/cobra"
)

const (
	ref string = "ref"
)

func BindCommand(cfg *velox.Config, out *string, zlog *slog.Logger) *cobra.Command {
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
				rp, err := gitlab.NewGLRepoInfo(cfg, zlog.WithGroup("GITLAB"))
				if err != nil {
					return err
				}

				mi, err = rp.GetPluginsModData()
				if err != nil {
					return err
				}
			}

			// roadrunner located on the github
			rp := github.NewGHRepoInfo(cfg, zlog.WithGroup("GITHUB"))
			path, err := rp.DownloadTemplate(os.TempDir(), cfg.Roadrunner[ref])
			if err != nil {
				zlog.Error("downloading template", slog.Any("error", err))
				os.Exit(1)
			}

			pMod, err := rp.GetPluginsModData()
			if err != nil {
				zlog.Error("get plugins mod data", slog.Any("error", err))
				os.Exit(1)
			}

			// append data from gitlab
			if mi != nil {
				pMod = append(pMod, mi...)
			}

			err = builder.NewBuilder(path, pMod, *out, cfg.Roadrunner[ref], cfg.Debug.Enabled, zlog.WithGroup("BUILDER")).Build(cfg.Roadrunner[ref])
			if err != nil {
				zlog.Error("fatal", slog.Any("error", err))
				os.Exit(1)
			}

			zlog.Info("========= build finished successfully =========", slog.Any("RoadRunner binary can be found at", *out))
			return nil
		},
	}
}
