package server

import (
	"errors"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/grpcreflect"
	"connectrpc.com/validate"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	servicev1 "github.com/roadrunner-server/velox/v2025/gen/go/api/service/v1/serviceV1connect"
)

// BindCommand returns a cobra.Command that starts the Velox server.
// The command listens on the provided address and uses zlog for logging.
// It registers the BuildService handler (with a request validation interceptor) and gRPC reflection,
// serves HTTP/2 over cleartext (h2c) with a one-minute header timeout, and runs until the server is closed.
// If the server stops due to an orderly shutdown, the command exits with nil; other listen errors are returned.
func BindCommand(address *string, zlog *zap.Logger) *cobra.Command {
	return &cobra.Command{
		Use:   "server",
		Short: "Run Velox server",
		RunE: func(_ *cobra.Command, _ []string) error {
			zlog.Debug("starting velox server", zap.String("address", *address))
			reflector := grpcreflect.NewStaticReflector(
				"/api.service.v1.BuildService/",
			)

			mux := http.NewServeMux()
			// build server
			client := NewBuildServer(zlog)
			path, handler := servicev1.NewBuildServiceHandler(client, connect.WithInterceptors(validate.NewInterceptor()))

			// handlers
			mux.Handle(path, handler)
			mux.Handle(grpcreflect.NewHandlerV1(reflector))

			server := &http.Server{
				Addr: *address,
				Handler: h2c.NewHandler(mux, &http2.Server{
					MaxConcurrentStreams: 256,
				}),
				ReadHeaderTimeout: time.Minute,
			}
			err := server.ListenAndServe()
			if err != nil {
				if errors.Is(err, http.ErrServerClosed) {
					return nil
				}

				return err
			}
			return nil
		},
	}
}