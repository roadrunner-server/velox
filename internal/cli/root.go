package cli

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"runtime"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/roadrunner-server/velox/v3"
	"github.com/roadrunner-server/velox/v3/internal/cli/build"
	"github.com/roadrunner-server/velox/v3/internal/version"
	"github.com/roadrunner-server/velox/v3/logger"
)

// NewCommand returns the root cobra command; PersistentPreRunE replaces the pointee of the shared logger after wiring.
func NewCommand(executableName string) *cobra.Command {
	lg := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	var (
		pathToConfig string
		outputFile   string
		config       = &velox.Config{}
	)

	cmd := &cobra.Command{
		Use:           executableName,
		Short:         "Automated build system for the RoadRunner server and its plugins",
		SilenceErrors: true,
		SilenceUsage:  true,
		Version:       fmt.Sprintf("%s (build time: %s, %s)", version.Version(), version.BuildTime(), runtime.Version()),
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			if pathToConfig == "" {
				return errors.New("path to the config should be provided")
			}

			v := viper.New()
			v.SetConfigFile(pathToConfig)
			if err := v.ReadInConfig(); err != nil {
				return err
			}
			var cfg velox.Config
			if err := v.Unmarshal(&cfg); err != nil {
				return err
			}
			if err := cfg.Validate(); err != nil {
				return err
			}
			*config = cfg

			zlog, err := logger.BuildLogger(config.Log[velox.LogLevelKey], config.Log[velox.LogModeKey])
			if err != nil {
				return err
			}
			*lg = *zlog
			return nil
		},
	}

	flag := cmd.PersistentFlags()
	flag.StringVarP(&pathToConfig, "config", "c", "velox.toml", "Path to the velox configuration file")
	flag.StringVarP(&outputFile, "out", "o", ".", "Output directory for the produced RoadRunner binary")

	cmd.AddCommand(build.BindCommand(config, &outputFile, lg))
	return cmd
}
