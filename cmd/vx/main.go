// Package main provides the vx CLI entrypoint.
package main

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/fatih/color"
	"github.com/roadrunner-server/velox/v3/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)

	cmd := cli.NewCommand(filepath.Base(os.Args[0]))
	err := cmd.ExecuteContext(ctx)
	stop() // release the signal handler explicitly; os.Exit below would skip defers
	if err != nil {
		_, _ = color.New(color.FgHiRed, color.Bold).Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
