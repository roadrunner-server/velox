// Package cli provides the CLI for the Velox build system
package cli

import (
	"fmt"
	"runtime"

	"github.com/pkg/errors"
	"github.com/roadrunner-server/velox/v2025"
	"github.com/roadrunner-server/velox/v2025/internal/cli/build"
	"github.com/roadrunner-server/velox/v2025/internal/cli/server"
	"github.com/roadrunner-server/velox/v2025/internal/version"
	"github.com/roadrunner-server/velox/v2025/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// NewCommand creates the root cobra command with build and server subcommands, config loading, and logger setup.
func NewCommand(executableName string) *cobra.Command {
	lg, _ := zap.NewDevelopment()

	var (
		pathToConfig string // path to the velox configuration
		cfgPath      = p("")
		outputFile   string // output file (optionally with directory)
		address      string
		config       = &velox.Config{} // velox configuration
	)

	cmd := &cobra.Command{
		Use:           executableName,
		Short:         "Automated build system for the RR and roadrunner-plugins",
		SilenceErrors: true,
		SilenceUsage:  true,
		Version:       fmt.Sprintf("%s (build time: %s, %s)", version.Version(), version.BuildTime(), runtime.Version()),
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if cmd.Use == "server" {
				return nil
			}
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

			*lg = *zlog

			return nil
		},
	}

	flag := cmd.PersistentFlags()
	flag.StringVarP(&pathToConfig, "config", "c", "velox.toml", "Path to the velox configuration file: -c velox.toml")
	flag.StringVarP(&outputFile, "out", "o", ".", "Output path: -o /usr/local/bin")
	flag.StringVarP(&address, "address", "a", "127.0.0.1:8080", "Address to bind server: -a 127.0.0.1:8080")

	cmd.AddCommand(
		build.BindCommand(config, cfgPath, lg.Named("builder")),
		server.BindCommand(&address, lg.Named("server")),
	)
	return cmd
}

// p is a generic helper function that returns a pointer to the given value.
func p[T any](val T) *T {
	return &val
}
