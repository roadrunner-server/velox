// Package server implements the Connect/gRPC build service that produces
// custom RoadRunner binaries on demand.
package server

import (
	"context"
	"fmt"
	"hash/fnv"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"time"

	"connectrpc.com/connect"
	lru "github.com/hashicorp/golang-lru/v2/expirable"
	"google.golang.org/protobuf/proto"

	"github.com/roadrunner-server/velox/v3"
	"github.com/roadrunner-server/velox/v3/builder"
	requestV1 "github.com/roadrunner-server/velox/v3/gen/go/api/request/v1"
	responseV1 "github.com/roadrunner-server/velox/v3/gen/go/api/response/v1"
	"github.com/roadrunner-server/velox/v3/github"
	"github.com/roadrunner-server/velox/v3/plugin"
)

const (
	binaryCacheSize    = 100
	binaryCacheTTL     = 30 * time.Minute
	processingLockSize = 100
	processingLockTTL  = 5 * time.Minute
)

// BuildServer is the Connect/gRPC handler for BuildService.
type BuildServer struct {
	log                 *slog.Logger
	lru                 *lru.LRU[string, string]
	currentlyProcessing *lru.LRU[string, struct{}]
	// inflightMu serializes the Contains/Add pair on currentlyProcessing so
	// two concurrent identical requests can't both pass the dedupe check.
	inflightMu sync.Mutex
	rrCache    github.Cache
}

// NewBuildServer constructs the server with bounded caches and per-eviction
// cleanup of on-disk artifacts.
func NewBuildServer(log *slog.Logger) *BuildServer {
	return &BuildServer{
		log: log,
		lru: lru.NewLRU(binaryCacheSize, func(hash, rrBinPath string) {
			log.Info("evicting binary cache entry",
				"hash", hash, "path", rrBinPath)
			if err := os.RemoveAll(rrBinPath); err != nil {
				log.Error("removing cached binary", "path", rrBinPath, "error", err)
			}
			tempDir := filepath.Join(os.TempDir(), hash)
			if err := os.RemoveAll(tempDir); err != nil {
				log.Error("removing temp dir", "path", tempDir, "error", err)
			}
		}, binaryCacheTTL),
		currentlyProcessing: lru.NewLRU(processingLockSize, func(key string, _ struct{}) {
			log.Info("releasing in-flight lock", "key", key)
		}, processingLockTTL),
		rrCache: github.NewLRUCache(0),
	}
}

// Build handles a single BuildRequest: deduplicates concurrent identical
// requests, serves cached results when possible, and otherwise drives the
// Builder pipeline end-to-end.
func (b *BuildServer) Build(ctx context.Context, req *connect.Request[requestV1.BuildRequest]) (*connect.Response[responseV1.BuildResponse], error) {
	// Default a missing target_platform to the host BEFORE hashing so that
	// `{platform: nil}` and `{platform: <host>}` produce the same cache key —
	// they describe the same build.
	if req.Msg.GetTargetPlatform() == nil {
		b.log.Info("target platform unspecified; using host platform")
		req.Msg.TargetPlatform = &requestV1.Platform{Os: runtime.GOOS, Arch: runtime.GOARCH}
	}

	hash, err := b.generateCacheHash(req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("generating cache hash: %w", err))
	}
	b.log.Debug("cache key computed", "hash", hash)

	b.inflightMu.Lock()
	if b.currentlyProcessing.Contains(hash) {
		b.inflightMu.Unlock()
		return nil, connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("build %s is already in progress", hash))
	}
	b.currentlyProcessing.Add(hash, struct{}{})
	b.inflightMu.Unlock()
	defer b.currentlyProcessing.Remove(hash)

	if cached, ok := b.lru.Get(hash); ok && !req.Msg.GetForceRebuild() {
		b.log.Debug("cache hit", "hash", hash)
		return connect.NewResponse(&responseV1.BuildResponse{
			Path: cached,
			Logs: "cached output, logs are available only on the first build",
		}), nil
	}

	plugins := make([]*plugin.Plugin, 0, len(req.Msg.GetPlugins()))
	for _, p := range req.Msg.GetPlugins() {
		if p == nil {
			continue
		}
		plugins = append(plugins, plugin.NewPlugin(p.GetModuleName(), p.GetTag()))
	}
	replaces := toReplaces(req.Msg.GetReplaces())
	excludes := toExcludes(req.Msg.GetExcludes())

	gh := github.NewClient("", os.Getenv("GITHUB_TOKEN"), b.rrCache, b.log.With("component", "github"))
	rrPath, err := gh.DownloadTemplate(ctx, os.TempDir(), hash, req.Msg.GetRrVersion())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("downloading template: %w", err))
	}

	outputPath := filepath.Join(os.TempDir(), hash)
	binaryPath, err := builder.NewBuilder(rrPath,
		builder.WithLogger(b.log.With("component", "build")),
		builder.WithPlugins(plugins...),
		builder.WithReplaces(replaces),
		builder.WithExcludes(excludes),
		builder.WithOutputDir(outputPath),
		builder.WithRRVersion(req.Msg.GetRrVersion()),
		builder.WithGOOS(req.Msg.GetTargetPlatform().GetOs()),
		builder.WithGOARCH(req.Msg.GetTargetPlatform().GetArch()),
		builder.WithDebug(req.Msg.GetDebug()),
		builder.WithRace(req.Msg.GetRace()),
	).Build(ctx, req.Msg.GetRrVersion())
	if err != nil {
		b.log.Error("build failed", "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("building plugins: %w", err))
	}

	b.lru.Add(hash, binaryPath)
	return connect.NewResponse(&responseV1.BuildResponse{Path: binaryPath}), nil
}

// generateCacheHash produces a deterministic key for the request. RequestId is
// excluded (UUID per call) and all repeated fields are sorted so two
// semantically equal requests with reordered lists produce the same hash.
func (b *BuildServer) generateCacheHash(req *requestV1.BuildRequest) (string, error) {
	keyed := &requestV1.BuildRequest{
		RrVersion:      req.GetRrVersion(),
		TargetPlatform: req.GetTargetPlatform(),
		Plugins:        sortedPlugins(req.GetPlugins()),
		Replaces:       sortedReplaces(req.GetReplaces()),
		Excludes:       sortedExcludes(req.GetExcludes()),
		Race:           req.GetRace(),
		Debug:          req.GetDebug(),
	}
	data, err := proto.MarshalOptions{Deterministic: true}.Marshal(keyed)
	if err != nil {
		return "", err
	}
	h := fnv.New64a()
	_, _ = h.Write(data)
	return strconv.FormatUint(h.Sum64(), 16), nil
}

func sortedPlugins(in []*requestV1.Plugin) []*requestV1.Plugin {
	out := append([]*requestV1.Plugin(nil), in...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].GetModuleName() != out[j].GetModuleName() {
			return out[i].GetModuleName() < out[j].GetModuleName()
		}
		return out[i].GetTag() < out[j].GetTag()
	})
	return out
}

func sortedReplaces(in []*requestV1.Replace) []*requestV1.Replace {
	out := append([]*requestV1.Replace(nil), in...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].GetOld() < out[j].GetOld() })
	return out
}

func sortedExcludes(in []*requestV1.Exclude) []*requestV1.Exclude {
	out := append([]*requestV1.Exclude(nil), in...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].GetModule() != out[j].GetModule() {
			return out[i].GetModule() < out[j].GetModule()
		}
		return out[i].GetVersion() < out[j].GetVersion()
	})
	return out
}

func toReplaces(in []*requestV1.Replace) []velox.Replace {
	if len(in) == 0 {
		return nil
	}
	out := make([]velox.Replace, 0, len(in))
	for _, r := range in {
		if r == nil {
			continue
		}
		out = append(out, velox.Replace{New: r.GetNew(), Old: r.GetOld()})
	}
	return out
}

func toExcludes(in []*requestV1.Exclude) []velox.Exclude {
	if len(in) == 0 {
		return nil
	}
	out := make([]velox.Exclude, 0, len(in))
	for _, e := range in {
		if e == nil {
			continue
		}
		out = append(out, velox.Exclude{Module: e.GetModule(), Version: e.GetVersion()})
	}
	return out
}
