package server

import (
	"bytes"
	"context"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/bufbuild/connect-go"
	lru "github.com/hashicorp/golang-lru/v2/expirable"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"buf.build/go/protovalidate"
	requestV1 "github.com/roadrunner-server/velox/v2025/gen/go/api/request/v1"
	responseV1 "github.com/roadrunner-server/velox/v2025/gen/go/api/response/v1"
	"github.com/roadrunner-server/velox/v2025/v2/builder"
	cacheimpl "github.com/roadrunner-server/velox/v2025/v2/cache"
	"github.com/roadrunner-server/velox/v2025/v2/github"
	"github.com/roadrunner-server/velox/v2025/v2/plugin"
)

type cache interface {
	Get(key string) *bytes.Buffer
	Set(key string, value *bytes.Buffer)
}

type BuildServer struct {
	log *zap.Logger
	// lru cache with the RR builds
	lru                 *lru.LRU[string, string]
	currentlyProcessing *lru.LRU[string, struct{}]
	rrcache             cache
}

func NewBuildServer(log *zap.Logger) *BuildServer {
	return &BuildServer{
		log: log,
		lru: lru.NewLRU(100, func(hash string, rrBinPath string) {
			// key -> hash, value - path to rr binary. On eviction -> delete file
			log.Info("evicting cache key", zap.String("hash", hash), zap.String("path", rrBinPath))
			err := os.RemoveAll(rrBinPath)
			if err != nil {
				log.Error("removing cached file", zap.String("path", rrBinPath), zap.Error(err))
			}
			// remove path
			log.Info("removing cached directory", zap.String("path", filepath.Join(os.TempDir(), hash)))
			err = os.RemoveAll(filepath.Join(os.TempDir(), hash))
			if err != nil {
				log.Error("failed to remove directory", zap.String("path", filepath.Join(os.TempDir(), hash)), zap.Error(err))
			}
		}, time.Minute*30),
		currentlyProcessing: lru.NewLRU(100, func(key string, _ struct{}) {
			// key -> hash, value - struct{} (no data to delete)
			log.Info("evicting currently processing key", zap.String("key", key))
		}, time.Minute*5),
		rrcache: cacheimpl.NewRRCache(),
	}
}

func (b *BuildServer) Build(_ context.Context, req *connect.Request[requestV1.BuildRequest]) (*connect.Response[responseV1.BuildResponse], error) {
	// validate the request
	err := protovalidate.Validate(req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("validating request: %w", err))
	}

	hash := b.generateCacheHash(req)
	b.log.Debug("cache key", zap.String("key", hash))

	// we can't process the same request concurrently, since we use a filesystem and we don't want to corrupt the state
	// b.currentlyProcessing is safe for concurrent use
	if b.currentlyProcessing.Contains(hash) {
		b.log.Debug("currently processing", zap.String("key", hash))
		return nil, connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("build is already in progress"))
	}

	// save currently processing key
	// needed for concurrent requests for the same request_id
	b.currentlyProcessing.Add(hash, struct{}{})
	defer b.currentlyProcessing.Remove(hash)

	if cached, ok := b.lru.Get(hash); ok && !req.Msg.GetForceRebuild() {
		b.log.Debug("cache hit", zap.String("key", hash))
		return connect.NewResponse(&responseV1.BuildResponse{
			Path: cached,
			Logs: "cached output, logs are available only on the first build",
		}), nil
	}

	outputPath := filepath.Join(os.TempDir(), hash)
	sb := new(strings.Builder)
	bplugins := make([]*plugin.Plugin, 0, 5)
	for _, p := range req.Msg.GetPlugins() {
		if p == nil {
			b.log.Warn("plugin info is nil")
			continue
		}

		bplugins = append(bplugins, plugin.NewPlugin(p.GetModuleName(), p.GetTag()))
	}

	rp := github.NewHTTPClient(os.Getenv("GITHUB_TOKEN"), b.rrcache, b.log.Named("GitHub"))
	path, err := rp.DownloadTemplate(os.TempDir(), hash, req.Msg.GetRrVersion())
	if err != nil {
		b.log.Error("downloading template", zap.Error(err))
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("downloading template: %w", err))
	}

	opts := make([]builder.Option, 0)
	opts = append(opts,
		builder.WithPlugins(bplugins...),
		builder.WithOutputDir(outputPath),
		builder.WithRRVersion(req.Msg.GetRrVersion()),
		builder.WithLogs(sb),
		builder.WithLogger(b.log.Named("Builder")),
		builder.WithGOOS(req.Msg.GetTargetPlatform().GetOs()),
		builder.WithGOARCH(req.Msg.GetTargetPlatform().GetArch()),
	)

	err = builder.NewBuilder(path, opts...).Build(req.Msg.GetRrVersion())
	if err != nil {
		b.log.Error("fatal", zap.Error(err))
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("building plugins: %w", err))
	}

	binaryPath := fmt.Sprintf("%s/%s", outputPath, "rr")
	resp := &responseV1.BuildResponse{
		// TODO: replace rr with a requested binary name (proto)
		Path: binaryPath,
		Logs: sb.String(),
	}

	b.lru.Add(hash, binaryPath)
	return connect.NewResponse(resp), nil
}

func (b *BuildServer) generateCacheHash(req *connect.Request[requestV1.BuildRequest]) string {
	cacheReq := &requestV1.BuildRequest{
		RequestId:      req.Msg.GetRequestId(),
		RrVersion:      req.Msg.GetRrVersion(),
		TargetPlatform: req.Msg.GetTargetPlatform(),
		Plugins:        req.Msg.GetPlugins(),
	}

	data, err := proto.MarshalOptions{
		Deterministic: true,
		AllowPartial:  false,
	}.Marshal(cacheReq)
	if err != nil {
		// TODO: might be just fail processing?
		b.log.Error("marshaling cache key error, cache creation would be skipped", zap.Error(err))
		return ""
	}

	h := fnv.New64a()
	h.Write(data)
	return strconv.FormatUint(h.Sum64(), 16)
}
