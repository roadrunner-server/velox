package commands

import (
	"fmt"
	"runtime"

	"github.com/pkg/errors"
	"github.com/roadrunner-server/velox"
	"github.com/roadrunner-server/velox/internal/commands/build"
	"github.com/roadrunner-server/velox/internal/version"
	"github.com/roadrunner-server/velox/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func NewCommand(executableName string) *cobra.Command {
	var config = &velox.Config{} // velox configuration
	var zapLogger = &zap.Logger{}
	var cfgPath = strPtr("")

	var (
		pathToConfig string // path to the velox configuration
		outputFile   string // output file (optionally with directory)

	)

	cmd := &cobra.Command{
		Use:           executableName,
		Short:         "Automated build system for the RR and roadrunner-plugins",
		SilenceErrors: true,
		SilenceUsage:  true,
		Version:       fmt.Sprintf("%s (build time: %s, %s)", version.Version(), version.BuildTime(), runtime.Version()),
		PersistentPreRunE: func(*cobra.Command, []string) error {
			var cfg *velox.Config
			// the user doesn't provide a path to the config
			if pathToConfig == "" {
				return errors.New("path to the config should be provided")
			}

			v := viper.New()
			v.SetConfigFile(pathToConfig)
			err := v.ReadInConfig()
			if err != nil {
				return err
			}

			err = v.Unmarshal(&cfg)
			if err != nil {
				return err
			}

			err = cfg.Validate()
			if err != nil {
				return err
			}

			*config = *cfg
			*cfgPath = outputFile

			// [log]
			// level = "debug"
			// mode = "development"
			zlog, err := logger.BuildLogger(config.Log["level"], config.Log["mode"])
			if err != nil {
				return err
			}

			*zapLogger = *zlog

			return nil
		},
	}

	flag := cmd.PersistentFlags()
	flag.StringVarP(&pathToConfig, "config", "c", "velox.toml", "Path to the velox configuration file: -c velox.toml")
	flag.StringVarP(&outputFile, "out", "o", ".", "Output path: -o /usr/local/bin")

	cmd.AddCommand(
		build.BindCommand(config, cfgPath, zapLogger),
	)
	return cmd
}

func strPtr(s string) *string {
	return &s
}
