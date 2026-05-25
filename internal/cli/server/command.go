package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/grpcreflect"
	"connectrpc.com/validate"
	"github.com/spf13/cobra"

	servicev1 "github.com/roadrunner-server/velox/v3/gen/go/api/service/v1/serviceV1connect"
)

const shutdownTimeout = 30 * time.Second

// BindCommand returns the cobra.Command that runs the build server. The root
// *slog.Logger is passed by pointer because the root command's PersistentPreRunE
// rewrites its pointee with the config-driven logger after construction; child
// loggers are therefore derived inside RunE, not at wiring time.
//
// The server honors the inherited cobra context for graceful shutdown: on
// SIGINT/SIGTERM, in-flight HTTP/2 streams get up to shutdownTimeout to
// finish before forced close.
func BindCommand(address *string, rootLog *slog.Logger) *cobra.Command {
	return &cobra.Command{
		Use:   "server",
		Short: "Run the Velox build server (Connect / gRPC over h2c)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			log := rootLog.With("component", "server")
			log.Debug("starting velox server", "address", *address)

			reflector := grpcreflect.NewStaticReflector("/api.service.v1.BuildService/")
			mux := http.NewServeMux()
			path, handler := servicev1.NewBuildServiceHandler(
				NewBuildServer(log),
				connect.WithInterceptors(validate.NewInterceptor()),
			)
			mux.Handle(path, handler)
			mux.Handle(grpcreflect.NewHandlerV1(reflector))

			protocols := &http.Protocols{}
			protocols.SetHTTP1(true)
			protocols.SetUnencryptedHTTP2(true)
			srv := &http.Server{
				Addr:              *address,
				Handler:           mux,
				ReadHeaderTimeout: time.Minute,
				Protocols:         protocols,
				HTTP2:             &http.HTTP2Config{MaxConcurrentStreams: 256},
			}

			errCh := make(chan error, 1)
			go func() { errCh <- srv.ListenAndServe() }()

			select {
			case <-cmd.Context().Done():
				log.Info("shutdown signal received")
				ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
				defer cancel()
				return srv.Shutdown(ctx)
			case err := <-errCh:
				if errors.Is(err, http.ErrServerClosed) {
					return nil
				}
				return err
			}
		},
	}
}
