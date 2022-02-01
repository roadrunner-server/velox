package build

import (
	"github.com/roadrunner-server/velox"
	"github.com/roadrunner-server/velox/github"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func BindCommand(cfg *velox.Config, out string, zlog *zap.Logger) *cobra.Command {
	return &cobra.Command{
		Use:   "build",
		Short: "Build RR",
		RunE: func(_ *cobra.Command, _ []string) error {
			rp := github.NewRepoInfo(cfg, zlog)
			path, err := rp.DownloadTemplate(cfg.Roadrunner["ref"])
			if err != nil {
				zlog.Fatal("[DOWNLOAD TEMPLATE]", zap.Error(err))
			}

			pMod, err := rp.GetPluginsModData()
			if err != nil {
				zlog.Fatal("[PLUGINS GET MOD INFO]", zap.Error(err))
			}

			builder := NewBuilder(path, pMod, out, zlog, cfg.Velox["build_args"])

			err = builder.Build()
			if err != nil {
				zlog.Fatal("[BUILD FAILED]", zap.Error(err))
			}

			zlog.Info("[BUILD]", zap.String("build finished, path", out))
			return nil
		},
	}
}
