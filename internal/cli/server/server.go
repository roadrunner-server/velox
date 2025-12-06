package server

import (
	"bytes"
	"context"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	lru "github.com/hashicorp/golang-lru/v2/expirable"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/roadrunner-server/velox/v2025/builder"
	cacheimpl "github.com/roadrunner-server/velox/v2025/cache"
	requestV1 "github.com/roadrunner-server/velox/v2025/gen/go/api/request/v1"
	responseV1 "github.com/roadrunner-server/velox/v2025/gen/go/api/response/v1"
	"github.com/roadrunner-server/velox/v2025/github"
	"github.com/roadrunner-server/velox/v2025/plugin"
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

// NewBuildServer creates and returns a BuildServer configured to orchestrate plugin builds,
// cache built binaries, and prevent concurrent builds for the same request.
//
// The returned BuildServer uses an LRU cache (capacity 100, 30-minute TTL) that maps request
// hashes to built binary paths and removes both the binary and its temporary directory on eviction.
// It also maintains a "currently processing" LRU (capacity 100, 5-minute TTL) to track in-flight
// builds and avoid duplicate concurrent work. An RR cache instance is created for template retrieval.
// The provided logger is used for operational logging.
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

// Build handles gRPC build requests, caches results, and prevents duplicate concurrent builds.
// It generates a hash from the request, checks caches, and builds RoadRunner with the specified plugins.
func (b *BuildServer) Build(_ context.Context, req *connect.Request[requestV1.BuildRequest]) (*connect.Response[responseV1.BuildResponse], error) {
	hash, err := b.generateCacheHash(req)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("generating cache hash: %w", err))
	}
	b.log.Debug("cache key", zap.String("key", hash))

	// we can't process the same request concurrently, since we use a filesystem, and we don't want to corrupt the state
	// b.currentlyProcessing is safe for concurrent use
	if b.currentlyProcessing.Contains(hash) {
		b.log.Debug("currently processing", zap.String("key", hash))
		return nil, connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("build is already in progress"))
	}

	// save the currently processing key
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

	// if a target platform is not specified,
	// use a host platform
	if req.Msg.GetTargetPlatform() == nil {
		b.log.Info("target platform is not specified, using host platform")
		req.Msg.TargetPlatform = &requestV1.Platform{
			Os:   runtime.GOOS,
			Arch: runtime.GOARCH,
		}
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

	binaryPath, err := builder.NewBuilder(path, opts...).Build(req.Msg.GetRrVersion())
	if err != nil {
		b.log.Error("fatal", zap.Error(err))
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("building plugins: %w", err))
	}

	resp := &responseV1.BuildResponse{
		Path: binaryPath,
		Logs: sb.String(),
	}

	b.lru.Add(hash, binaryPath)
	return connect.NewResponse(resp), nil
}

// generateCacheHash generates a deterministic FNV-64a hash from the build request for caching.
func (b *BuildServer) generateCacheHash(req *connect.Request[requestV1.BuildRequest]) (string, error) {
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
		b.log.Error("marshaling cache key error, cache creation would be skipped", zap.Error(err))
		return "", err
	}

	h := fnv.New64a()
	h.Write(data)
	return strconv.FormatUint(h.Sum64(), 16), nil
}
