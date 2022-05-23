package build

import (
	"syscall"

	"github.com/roadrunner-server/velox"
	"github.com/roadrunner-server/velox/builder"
	"github.com/roadrunner-server/velox/github"
	"github.com/roadrunner-server/velox/gitlab"
	"github.com/roadrunner-server/velox/shared"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

const (
	ref       string = "ref"
	buildArgs string = "build_args"
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

			var mi []*shared.ModulesInfo
			if cfg.GitLab != nil {
				rp, err := gitlab.NewGLRepoInfo(cfg, zlog)
				if err != nil {
					return err
				}

				mi, err = rp.GetPluginsModData()
				if err != nil {
					return err
				}
			}

			// roadrunner located on the github
			rp := github.NewGHRepoInfo(cfg, zlog)
			path, err := rp.DownloadTemplate(cfg.Roadrunner[ref])
			if err != nil {
				zlog.Fatal("[DOWNLOAD TEMPLATE]", zap.Error(err))
			}

			pMod, err := rp.GetPluginsModData()
			if err != nil {
				zlog.Fatal("[PLUGINS GET MOD INFO]", zap.Error(err))
			}

			// append data from gitlab
			if mi != nil {
				pMod = append(pMod, mi...)
			}

			err = builder.NewBuilder(path, pMod, *out, zlog, cfg.Velox[buildArgs]).Build()
			if err != nil {
				zlog.Fatal("[BUILD FAILED]", zap.Error(err))
			}

			zlog.Info("[BUILD]", zap.String("build finished, path", *out))
			return nil
		},
	}
}
