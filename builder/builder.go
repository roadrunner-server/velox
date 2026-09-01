package builder

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/roadrunner-server/velox/v3"
	"github.com/roadrunner-server/velox/v3/builder/templates"
	"github.com/roadrunner-server/velox/v3/logger"
	"github.com/roadrunner-server/velox/v3/plugin"
)

const (
	executableName = "rr"
	pluginsRelPath = "container/plugins.go"
	rrMainGo       = "cmd/rr/main.go"
	smokeTimeout   = 5 * time.Second
)

// Builder turns a downloaded RoadRunner source tree and a user plugin set into a custom binary.
type Builder struct {
	rrTempPath string
	outputDir  string
	log        *slog.Logger
	plugins    []*plugin.Plugin
	replaces   []velox.Replace
	excludes   []velox.Exclude
	debug      bool
	race       bool
	rrVersion  string
	goos       string
	goarch     string
	env        []string
}

// NewBuilder creates a Builder rooted at the directory holding the downloaded RoadRunner source tree.
func NewBuilder(rrTmpPath string, opts ...Option) *Builder {
	b := &Builder{rrTempPath: rrTmpPath, log: logger.Discard()}
	for _, opt := range opts {
		opt(b)
	}
	b.env = newEnv(b.goos, b.goarch, b.race)
	return b
}

// Build runs the whole pipeline and returns the path of the binary in the output directory.
func (b *Builder) Build(ctx context.Context, rrRef string) (string, error) {
	if err := b.validateInputs(); err != nil {
		return "", err
	}
	b.log.Info("building RoadRunner", "ref", rrRef)

	plugin.ResolvePrefixCollisions(b.plugins)

	// Read the upstream go.mod before applyRequires adds the user plugins to it.
	up, err := b.upstreamModule(ctx)
	if err != nil {
		return "", fmt.Errorf("upstreamModule: %w", err)
	}
	b.log.Info("RoadRunner module", "ref", rrRef, "module", up.Path, "major", majorVersion(up.Path))

	if err := b.writePluginsGo(up); err != nil {
		return "", fmt.Errorf("writePluginsGo: %w", err)
	}
	if err := b.applyRequires(ctx); err != nil {
		return "", fmt.Errorf("applyRequires: %w", err)
	}
	if err := b.applyReplaces(ctx); err != nil {
		return "", fmt.Errorf("applyReplaces: %w", err)
	}
	if err := b.applyExcludes(ctx); err != nil {
		return "", fmt.Errorf("applyExcludes: %w", err)
	}
	if err := b.goModTidy(ctx); err != nil {
		return "", fmt.Errorf("go mod tidy: %w", err)
	}
	if err := b.verifyResolvedVersions(ctx); err != nil {
		return "", fmt.Errorf("verifyResolvedVersions: %w", err)
	}
	builtPath, err := b.compile(ctx, up)
	if err != nil {
		return "", fmt.Errorf("compile: %w", err)
	}
	finalPath, err := b.relocate(builtPath)
	if err != nil {
		return "", fmt.Errorf("relocate: %w", err)
	}
	if err := b.smokeTest(ctx, finalPath); err != nil {
		return "", fmt.Errorf("smokeTest: %w", err)
	}
	return finalPath, nil
}

func (b *Builder) validateInputs() error {
	if len(b.plugins) == 0 {
		return errors.New("no plugins provided; use WithPlugins to add at least one")
	}
	if b.rrTempPath == "" {
		return errors.New("RR source path is empty")
	}
	if b.outputDir == "" {
		return errors.New("output directory is empty; use WithOutputDir")
	}
	if err := velox.ValidateTargetOS(b.goos); err != nil {
		return err
	}
	return b.ensureOutputDir()
}

func (b *Builder) ensureOutputDir() error {
	if info, err := os.Stat(b.outputDir); err == nil {
		if !info.IsDir() {
			return fmt.Errorf("%s is not a directory", b.outputDir)
		}
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.MkdirAll(b.outputDir, 0o755)
}

// writePluginsGo renders container/plugins.go using the parameterized template.
func (b *Builder) writePluginsGo(up upstreamModule) error {
	kept := make([]*plugin.Plugin, 0, len(b.plugins))
	for _, p := range b.plugins {
		// The template hardcodes informer and resetter; a user entry registers them twice.
		if isModuleUnder(p.ModuleName(), informerModule) || isModuleUnder(p.ModuleName(), resetterModule) {
			b.log.Warn("skipping bundled plugin listed in velox.toml", "module", p.ModuleName())
			continue
		}
		kept = append(kept, p)
	}

	src, err := templates.Render(templates.NewTemplate(up.Informer, up.Resetter, kept))
	if err != nil {
		return fmt.Errorf("render plugins.go template: %w", err)
	}

	pluginsPath := filepath.Join(b.rrTempPath, pluginsRelPath)
	if err := os.WriteFile(pluginsPath, src, 0o600); err != nil {
		return fmt.Errorf("write plugins.go: %w", err)
	}

	b.log.Debug("wrote container/plugins.go",
		"informer", up.Informer,
		"resetter", up.Resetter,
		"user_plugins", len(kept),
	)
	return nil
}

// verifyResolvedVersions checks the resolved version of every pinned plugin; a "latest" tag stays unchecked.
func (b *Builder) verifyResolvedVersions(ctx context.Context) error {
	want := make(map[string]string, len(b.plugins))
	for _, p := range b.plugins {
		if p.Tag() == "" || p.Tag() == "latest" {
			continue
		}
		want[p.ModuleName()] = p.Tag()
	}
	// An empty operand list makes `go list -m` describe the main module.
	if len(want) == 0 {
		return nil
	}

	args := append([]string{"list", "-m", "-e", "-json"}, slices.Sorted(maps.Keys(want))...)
	res, err := b.runGo(ctx, args...)
	if err != nil {
		return fmt.Errorf("go list -m: %w", err)
	}

	dec := jsontext.NewDecoder(bytes.NewReader(res.Stdout))
	for {
		var mod struct {
			Path    string
			Version string
			Replace *struct {
				Path    string
				Version string
			}
			Error *struct {
				Err string
			}
		}
		if err := json.UnmarshalDecode(dec, &mod); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("parse go list output: %w", err)
		}
		// `go list -e` exits 0 and reports per-module failures in the payload.
		if mod.Error != nil {
			return fmt.Errorf("go list -m %s: %s", mod.Path, mod.Error.Err)
		}

		tag, pinned := want[mod.Path]
		if !pinned {
			continue
		}
		version := mod.Version
		if mod.Replace != nil {
			// A different module or a local directory carries no comparable version.
			if mod.Replace.Path != mod.Path || mod.Replace.Version == "" {
				continue
			}
			version = mod.Replace.Version
		}
		if version != "" && version != tag {
			return fmt.Errorf(
				"plugin %s resolved to %s (you requested %s); use a [[replaces]] entry to force this version",
				mod.Path, version, tag,
			)
		}
	}
}

// compile runs go build; all -ldflags values go in one argument because a repeated -ldflags flag overwrites the earlier value.
func (b *Builder) compile(ctx context.Context, up upstreamModule) (string, error) {
	args := []string{"build", "-v", "-trimpath"}
	if b.debug {
		args = append(args, "-gcflags", "all=-N -l", "-tags", "debug")
	}
	if b.race {
		args = append(args, "-race")
	}

	ldParts := []string{ldflags(up.Path, b.rrVersion, buildTimestamp())}
	if !b.debug {
		ldParts = append(ldParts, "-s", "-w")
	}
	args = append(args, "-ldflags", strings.Join(ldParts, " "))

	outPath := filepath.Join(b.rrTempPath, executableName)
	args = append(args, "-o", outPath, rrMainGo)

	if _, err := b.runGo(ctx, args...); err != nil {
		return "", err
	}
	return outPath, nil
}

func (b *Builder) relocate(srcBin string) (string, error) {
	dst := filepath.Join(b.outputDir, executableName)
	b.log.Info("moving binary", "from", srcBin, "to", dst)
	if err := os.Rename(srcBin, dst); err != nil {
		return "", fmt.Errorf("move binary: %w", err)
	}
	return dst, nil
}

// smokeTest runs `rr --version` on the new binary when the host platform matches the target.
func (b *Builder) smokeTest(ctx context.Context, binPath string) error {
	hostOS, hostArch := runtime.GOOS, runtime.GOARCH
	if b.goos != "" && b.goos != hostOS {
		b.log.Info("skipping smoke test (cross-compiled)",
			"target_os", b.goos, "host_os", hostOS)
		return nil
	}
	if b.goarch != "" && b.goarch != hostArch {
		b.log.Info("skipping smoke test (cross-compiled)",
			"target_arch", b.goarch, "host_arch", hostArch)
		return nil
	}

	smokeCtx, cancel := context.WithTimeout(ctx, smokeTimeout)
	defer cancel()
	out, err := exec.CommandContext(smokeCtx, binPath, "--version").CombinedOutput()
	if err != nil {
		return fmt.Errorf("`%s --version` failed: %w\n%s", binPath, err, out)
	}
	b.log.Info("smoke test passed", "version", string(out))
	return nil
}

// newEnv overlays the target platform settings on the parent environment.
func newEnv(goos, goarch string, race bool) []string {
	env := slices.Clone(os.Environ())
	if goos != "" {
		env = setKV(env, "GOOS", goos)
	}
	if goarch != "" {
		env = setKV(env, "GOARCH", goarch)
	}
	if race {
		env = setKV(env, "CGO_ENABLED", "1")
	} else {
		env = setKV(env, "CGO_ENABLED", "0")
	}

	// A cross build gets its own GOPATH tree; a native build keeps the caller caches.
	cross := goos != "" && goarch != "" && (goos != runtime.GOOS || goarch != runtime.GOARCH)
	if home, err := os.UserHomeDir(); err == nil && cross {
		gopath := filepath.Join(home, "go", goos, goarch)
		env = setKV(env, "GOPATH", gopath)
		env = setKV(env, "GOCACHE", filepath.Join(gopath, "go-build"))
	}
	return env
}

// setKV replaces (or appends) "KEY=value" in env.
func setKV(env []string, key, value string) []string {
	prefix := key + "="
	for i, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}

// buildTimestamp returns the RFC3339 timestamp for ldflags and honors SOURCE_DATE_EPOCH for reproducible builds.
func buildTimestamp() string {
	if s := os.Getenv("SOURCE_DATE_EPOCH"); s != "" {
		if secs, err := strconv.ParseInt(s, 10, 64); err == nil {
			return time.Unix(secs, 0).UTC().Format(time.RFC3339)
		}
	}
	return time.Now().UTC().Format(time.RFC3339)
}
