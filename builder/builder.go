// Package builder orchestrates the assembly of a custom RoadRunner binary:
// download the upstream template, inject user plugins, apply go.mod
// replace/exclude directives, run `go mod tidy`, and run `go build`.
package builder

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-version"

	"github.com/roadrunner-server/velox/v3"
	"github.com/roadrunner-server/velox/v3/builder/templates"
	"github.com/roadrunner-server/velox/v3/logger"
	"github.com/roadrunner-server/velox/v3/plugin"
)

const (
	executableName = "rr"
	pluginsRelPath = "container/plugins.go"
	goModFile      = "go.mod"
	rrMainGo       = "cmd/rr/main.go"
	cleanupPattern = "roadrunner-server*"
	smokeTimeout   = 5 * time.Second

	// ldflagsFmt injects build metadata into the produced binary. The format
	// uses v3 paths to match the post-bump upstream RoadRunner repository.
	ldflagsFmt = "-X github.com/roadrunner-server/roadrunner/v3/internal/meta.version=%s" +
		" -X github.com/roadrunner-server/roadrunner/v3/internal/meta.buildTime=%s"
)

// Builder produces a custom RoadRunner binary from a downloaded RR source
// directory plus a user-supplied plugin set, replace/exclude directives, and
// build flags. Construct via NewBuilder + functional options.
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
}

// NewBuilder creates a Builder rooted at the directory containing the
// downloaded RoadRunner source tree.
func NewBuilder(rrTmpPath string, opts ...Option) *Builder {
	b := &Builder{rrTempPath: rrTmpPath, log: logger.Discard()}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// Build orchestrates the full produce-binary pipeline. It returns the path to
// the final binary in the configured output directory, or an error wrapping
// the failing stage and (when available) the last 8 KB of stderr.
func (b *Builder) Build(ctx context.Context, rrRef string) (string, error) {
	if err := b.validateInputs(); err != nil {
		return "", err
	}

	module, err := parseRRMajor(rrRef)
	if err != nil {
		return "", err
	}
	b.log.Info("RoadRunner major version", "ref", rrRef, "major", module)

	plugin.ResolvePrefixCollisions(b.plugins)

	defer b.cleanupOutputDir()

	if err := b.writePluginsGo(); err != nil {
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
	builtPath, err := b.compile(ctx)
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
	if strings.EqualFold(b.goos, "windows") {
		return errors.New("velox v3 does not support Windows targets")
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
// The bundled informer/resetter import paths come from the downloaded RR's own
// go.mod, so the same template works for every RR major version.
func (b *Builder) writePluginsGo() error {
	goModBytes, err := os.ReadFile(filepath.Join(b.rrTempPath, goModFile))
	if err != nil {
		return fmt.Errorf("read upstream go.mod: %w", err)
	}
	informer, resetter, err := templates.ParseUpstreamModules(goModBytes)
	if err != nil {
		return err
	}

	t := templates.NewTemplate(b.plugins)
	t.InformerImport = informer
	t.ResetterImport = resetter

	pluginsPath := filepath.Join(b.rrTempPath, pluginsRelPath)
	if err := os.Remove(pluginsPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove old plugins.go: %w", err)
	}

	f, err := os.OpenFile(pluginsPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("open plugins.go: %w", err)
	}
	defer func() { _ = f.Close() }()

	if err := templates.CompilePlugins(f, t); err != nil {
		return fmt.Errorf("render plugins.go template: %w", err)
	}
	b.log.Debug("wrote container/plugins.go",
		"informer", informer,
		"resetter", resetter,
		"user_plugins", len(b.plugins),
	)
	return nil
}

// verifyResolvedVersions asks the Go toolchain for the post-tidy version of
// every user-requested plugin. If `go mod tidy` upgraded any plugin past the
// requested tag (typically because upstream RR transitively pins a newer
// version), we surface an actionable error instead of building a binary that
// silently uses a different plugin version than the user asked for.
//
// `tag = "latest"` is treated as "whatever tidy resolves" — no check.
func (b *Builder) verifyResolvedVersions(ctx context.Context) error {
	for _, p := range b.plugins {
		if p.Tag() == "" || p.Tag() == "latest" {
			continue
		}
		res, err := runCmd(ctx, b.log, b.rrTempPath, b.env(),
			"go", "list", "-m", "-json", p.ModuleName())
		if err != nil {
			return fmt.Errorf("go list -m %s: %w", p.ModuleName(), err)
		}
		var mod struct {
			Path    string
			Version string
		}
		if err := json.Unmarshal(res.Stdout, &mod); err != nil {
			return fmt.Errorf("parse go list output for %s: %w", p.ModuleName(), err)
		}
		if mod.Version != "" && mod.Version != p.Tag() {
			return fmt.Errorf(
				"plugin %s resolved to %s (you requested %s); use a [[replaces]] entry to force this version",
				p.ModuleName(), mod.Version, p.Tag(),
			)
		}
	}
	return nil
}

// compile runs `go build` in the RR source tree. Returns the path of the
// produced binary (still inside the temp dir).
//
// All `-ldflags` values are concatenated into a single argument: passing
// `-ldflags` twice would let the later invocation silently overwrite the
// earlier one, so the release-mode `-s -w` strip flags must be folded into
// the same flag value as the version-injection symbols.
func (b *Builder) compile(ctx context.Context) (string, error) {
	args := []string{"build", "-v", "-trimpath"}
	if b.debug {
		args = append(args, "-gcflags", "all=-N -l", "-tags", "debug")
	}
	if b.race {
		args = append(args, "-race")
	}

	ldParts := []string{fmt.Sprintf(ldflagsFmt, b.rrVersion, buildTimestamp())}
	if !b.debug {
		ldParts = append(ldParts, "-s", "-w")
	}
	args = append(args, "-ldflags", strings.Join(ldParts, " "))

	outPath := filepath.Join(b.rrTempPath, executableName)
	args = append(args, "-o", outPath, rrMainGo)

	if _, err := runCmd(ctx, b.log, b.rrTempPath, b.env(), "go", args...); err != nil {
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

// smokeTest invokes `./rr --version` on the freshly-built binary when the host
// platform matches the target. Cross-compiled binaries are not exercised.
func (b *Builder) smokeTest(ctx context.Context, binPath string) error {
	hostOS, hostArch := goosFromRuntime(), goarchFromRuntime()
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

// cleanupOutputDir removes leftover roadrunner-server* dirs in the output
// directory so the next build starts from a clean slate.
func (b *Builder) cleanupOutputDir() {
	files, err := filepath.Glob(filepath.Join(b.outputDir, cleanupPattern))
	if err != nil {
		return
	}
	for _, f := range files {
		b.log.Info("cleaning temporary folder", "path", f)
		_ = os.RemoveAll(f)
	}
}

// env composes the subprocess environment, inheriting from the parent (so
// GOPROXY, GOPRIVATE, GOFLAGS, etc. are preserved) and overlaying our
// target-platform / cgo / GOPATH settings.
func (b *Builder) env() []string {
	env := append([]string(nil), os.Environ()...)
	if b.goos != "" {
		env = setKV(env, "GOOS", b.goos)
	}
	if b.goarch != "" {
		env = setKV(env, "GOARCH", b.goarch)
	}
	if b.race {
		env = setKV(env, "CGO_ENABLED", "1")
	} else {
		env = setKV(env, "CGO_ENABLED", "0")
	}
	if home, err := os.UserHomeDir(); err == nil && b.goos != "" && b.goarch != "" {
		gopath := filepath.Join(home, "go", b.goos, b.goarch)
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

// buildTimestamp returns the RFC3339 timestamp used in ldflags. Honors
// SOURCE_DATE_EPOCH (https://reproducible-builds.org/specs/source-date-epoch/)
// so distros / CI can produce bit-identical binaries.
func buildTimestamp() string {
	if s := os.Getenv("SOURCE_DATE_EPOCH"); s != "" {
		if secs, err := strconv.ParseInt(s, 10, 64); err == nil {
			return time.Unix(secs, 0).UTC().Format(time.RFC3339)
		}
	}
	return time.Now().UTC().Format(time.RFC3339)
}

// parseRRMajor returns the major-version identifier (vN or vYYYY) for an RR
// ref. "master" maps to the current default V3. Legacy year-based refs
// (v2025.x.y, v2024.x.y) keep their year identifier so older RR releases
// continue to build.
func parseRRMajor(ref string) (string, error) {
	if ref == "master" {
		return velox.V3, nil
	}
	v, err := version.NewVersion(ref)
	if err != nil {
		return "", fmt.Errorf("invalid RR ref %q: %w", ref, err)
	}
	return fmt.Sprintf("v%d", v.Segments()[0]), nil
}
