package server

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/bufbuild/connect-go"
	lru "github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/roadrunner-server/velox/v2025"
	"go.uber.org/zap"

	"buf.build/go/protovalidate"
	"github.com/roadrunner-server/velox/v2025/builder"
	requestV1 "github.com/roadrunner-server/velox/v2025/gen/go/api/request/v1"
	responseV1 "github.com/roadrunner-server/velox/v2025/gen/go/api/response/v1"
	"github.com/roadrunner-server/velox/v2025/github"
)

type BuildServer struct {
	log *zap.Logger
	lru *lru.LRU[string, any]
}

func NewBuildServer(log *zap.Logger) *BuildServer {
	return &BuildServer{
		log: log,
		lru: lru.NewLRU(100, func(key string, _ any) {
			log.Info("evicting cache key", zap.String("key", key))
		}, time.Minute*30),
	}
}

func (b *BuildServer) Build(_ context.Context, req *connect.Request[requestV1.BuildRequest]) (*connect.Response[responseV1.BuildResponse], error) {
	// validate the request
	err := protovalidate.Validate(req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("validating request: %w", err))
	}

	cfg := velox.DefaultConfig
	if req.Msg.GetRrVersion() != "" {
		cfg.Roadrunner[velox.DefaultRRRef] = req.Msg.GetRrVersion()
	}

	sb := new(strings.Builder)

	for pi, p := range req.Msg.GetPluginsInfo() {
		if p == nil {
			b.log.Warn("plugin info is nil", zap.String("plugin", pi))
			continue
		}

		switch strings.ToLower(pi) {
		case "github":
			if cfg.GitHub == nil {
				cfg.GitHub = &velox.CodeHosting{
					Token: &velox.Token{
						Token: os.Getenv("GITHUB_TOKEN"),
					},
				}
			}

			if cfg.GitHub.Plugins == nil {
				cfg.GitHub.Plugins = make(map[string]*velox.PluginConfig)
			}

			for _, m := range p.GetPlugins() {
				b.log.Debug("adding plugin", zap.String("plugin", m.GetName()))
				name := m.GetName()
				if m.GetName() == "" {
					name = fmt.Sprintf("%s/%s/%s", m.GetOwner(), m.GetRepository(), m.GetRef())
				}

				cfg.GitHub.Plugins[name] = &velox.PluginConfig{
					Ref:   m.GetRef(),
					Owner: m.GetOwner(),
					Repo:  m.GetRepository(),
				}
			}

			err := cfg.Validate()
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("github plugin config: %w", err))
			}
		case "gitlab":
		default:
		}

		rp := github.NewGHRepoInfo(cfg, b.log.Named("GitHub"))
		path, err := rp.DownloadTemplate(os.TempDir(), cfg.Roadrunner[velox.DefaultRRRef])
		if err != nil {
			b.log.Error("downloading template", zap.Error(err))
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("downloading template: %w", err))
		}

		pMod, err := rp.GetPluginsModData()
		if err != nil {
			b.log.Error("get plugins mod data", zap.Error(err))
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get plugins mod data: %w", err))
		}

		err = builder.NewBuilder(path, pMod,
			builder.WithOutputDir(os.TempDir()),
			builder.WithRRVersion(cfg.Roadrunner[velox.DefaultRRRef]),
			builder.WithDebug(cfg.Debug.Enabled),
			builder.WithLogs(sb),
			builder.WithLogger(b.log.Named("Builder")),
		).Build(cfg.Roadrunner[velox.DefaultRRRef])
		if err != nil {
			b.log.Error("fatal", zap.Error(err))
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("building plugins: %w", err))
		}
	}

	resp := &responseV1.BuildResponse{
		Path: "foo/bar",
		Logs: sb.String(),
	}

	return connect.NewResponse(resp), nil
}
