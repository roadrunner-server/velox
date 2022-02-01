package run

import (
	"github.com/roadrunner-server/velox"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func BindCommand(cfg *velox.Config, log *zap.Logger) *cobra.Command {
	return &cobra.Command{}
}
