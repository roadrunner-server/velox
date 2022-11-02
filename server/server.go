package server

import (
	"net"
	"sync"

	"github.com/pkg/errors"
	"github.com/roadrunner-server/velox"
	veloxv1 "go.buf.build/grpc/go/roadrunner-server/api/velox/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type Server struct {
	srv *grpc.Server
	log *zap.Logger
}

func NewServer(log *zap.Logger) (*Server, error) {
	return &Server{
		log: log,
		srv: grpc.NewServer(),
	}, nil
}

func (s *Server) Start() error {
	l, err := net.Listen("tcp", "127.0.0.1:10000")
	if err != nil {
		return err
	}

	veloxv1.RegisterBuilderServiceServer(s.srv, &Builder{
		log: s.log,
		cfgPool: sync.Pool{
			New: func() any {
				return new(velox.Config)
			},
		},
	},
	)
	err = s.srv.Serve(l)
	if err != nil {
		if errors.Is(err, grpc.ErrServerStopped) {
			return nil
		}

		return err
	}

	return nil
}

func (s *Server) Stop() {
	s.srv.GracefulStop()
}
