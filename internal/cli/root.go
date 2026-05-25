// Package cli wires the root cobra command and the build / server subcommands.
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
	"github.com/roadrunner-server/velox/v3/internal/cli/server"
	"github.com/roadrunner-server/velox/v3/internal/version"
	"github.com/roadrunner-server/velox/v3/logger"
)

// NewCommand returns the root cobra command. The CLI uses cmd.Context() (set
// by the caller in main.go) so SIGINT/SIGTERM propagates through the whole
// build pipeline.
//
// The root *slog.Logger is shared with subcommands by pointer. PersistentPreRunE
// rewrites the pointee with the config-driven logger after the subcommand
// callbacks have been wired, so each subcommand calls .With(...) inside its
// RunE to derive a child logger from the *current* state.
func NewCommand(executableName string) *cobra.Command {
	lg := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	var (
		pathToConfig string
		outputFile   string
		address      string
		config       = &velox.Config{}
	)

	cmd := &cobra.Command{
		Use:           executableName,
		Short:         "Automated build system for the RoadRunner server and its plugins",
		SilenceErrors: true,
		SilenceUsage:  true,
		Version:       fmt.Sprintf("%s (build time: %s, %s)", version.Version(), version.BuildTime(), runtime.Version()),
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if cmd.Use == "server" {
				return nil
			}
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
	flag.StringVarP(&address, "address", "a", "127.0.0.1:8080", "Bind address for the build server")

	cmd.AddCommand(
		build.BindCommand(config, &outputFile, lg),
		server.BindCommand(&address, lg),
	)
	return cmd
}
