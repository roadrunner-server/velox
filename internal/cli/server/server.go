package server

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/roadrunner-server/velox/server"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func ServeCommand(zlog *zap.Logger) *cobra.Command {
	return &cobra.Command{
		Use:   "server",
		Short: "start an gRPC server",
		RunE: func(_ *cobra.Command, _ []string) error {
			srv, err := server.NewServer(zlog)
			if err != nil {
				return err
			}

			errCh := make(chan error, 1)
			oss, stop := make(chan os.Signal, 5), make(chan struct{}, 1) //nolint:gomnd
			signal.Notify(oss, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

			go func() {
				err = srv.Start()
				if err != nil {
					errCh <- err
					return
				}
			}()

			go func() {
				// first catch - stop the server
				<-oss
				// send signal to stop execution
				stop <- struct{}{}
				// second catch - exit from the process
				<-oss
				zlog.Info("exit forced")
				os.Exit(1)
			}()

			for {
				select {
				case e := <-errCh:
					zlog.Error("server error", zap.Error(e))
				case <-stop: // stop the container after first signal
					zlog.Info("stop signal received, stopping the gRPC server")

					srv.Stop()

					return nil
				}
			}
		},
	}
}
