package main

import (
	"os"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/roadrunner-server/velox/internal/cli"
)

func main() {
	// os.Args[0] always contains a path to the executable, like foo/bar/rr -> rr
	cmd := cli.NewCommand(filepath.Base(os.Args[0]))
	err := cmd.Execute()
	if err != nil {
		_, _ = color.New(color.FgHiRed, color.Bold).Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
