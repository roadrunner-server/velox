package builder

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"
)

const (
	gracefulKillTimeout = 15 * time.Second
	stderrCaptureLimit  = 8 * 1024
)

// runResult holds the captured output of a single subprocess invocation.
type runResult struct {
	Stdout []byte
	// Stderr is the last stderrCaptureLimit bytes of stderr (older bytes dropped).
	Stderr []byte
}

// runCmd executes name with args in dir under env, honoring ctx for cancellation.
// stderr is captured into a bounded buffer (last 8 KB) and embedded in the returned
// error on failure. stdout is captured fully and returned for callers that need it.
//
// On ctx.Done(): SIGINT is sent immediately; if the process hasn't exited within
// gracefulKillTimeout, it is killed.
//
// name is parameterized (rather than hard-coded to "go") so tests can inject a
// fake `go` script via PATH manipulation.
//
//nolint:unparam // name is intentionally pluggable for test fakes
func runCmd(ctx context.Context, log *zap.Logger, dir string, env []string,
	name string, args ...string,
) (runResult, error) {
	if log != nil {
		log.Info("executing command",
			zap.String("cmd", name+" "+strings.Join(args, " ")),
			zap.String("dir", dir),
		)
	}

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = env

	stdout := &bytes.Buffer{}
	stderr := newRingBuffer(stderrCaptureLimit)
	cmd.Stdout = stdout
	if log != nil {
		// also tee stderr to the debug logger so live builds are observable
		cmd.Stderr = io.MultiWriter(stderr, &zapDebugWriter{log: log})
	} else {
		cmd.Stderr = stderr
	}

	if err := cmd.Start(); err != nil {
		return runResult{}, fmt.Errorf("starting %s: %w", name, err)
	}

	doneCh := make(chan error, 1)
	go func() { doneCh <- cmd.Wait() }()

	select {
	case err := <-doneCh:
		res := runResult{Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}
		if err != nil {
			return res, fmt.Errorf("%s failed: %w\n--- stderr (last %d bytes) ---\n%s",
				name, err, len(res.Stderr), res.Stderr)
		}
		return res, nil
	case <-ctx.Done():
		// Send SIGINT first for graceful shutdown.
		_ = cmd.Process.Signal(syscall.SIGINT)
		select {
		case <-doneCh:
		case <-time.After(gracefulKillTimeout):
			_ = cmd.Process.Kill()
			<-doneCh
		}
		return runResult{Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}, ctx.Err()
	}
}

// ringBuffer keeps at most capacity bytes; older bytes are dropped on overflow.
type ringBuffer struct {
	mu       sync.Mutex
	capacity int
	data     []byte
}

func newRingBuffer(capacity int) *ringBuffer { return &ringBuffer{capacity: capacity} }

func (r *ringBuffer) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data = append(r.data, p...)
	if len(r.data) > r.capacity {
		r.data = r.data[len(r.data)-r.capacity:]
	}
	return len(p), nil
}

func (r *ringBuffer) Bytes() []byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]byte, len(r.data))
	copy(out, r.data)
	return out
}

// zapDebugWriter forwards writes to a zap logger at debug level.
type zapDebugWriter struct{ log *zap.Logger }

func (w *zapDebugWriter) Write(p []byte) (int, error) {
	w.log.Debug("[stderr]", zap.ByteString("data", p))
	return len(p), nil
}

// goModEdit runs `go mod edit args...` inside b.rrTempPath.
func (b *Builder) goModEdit(ctx context.Context, args ...string) error {
	_, err := runCmd(ctx, b.log, b.rrTempPath, b.env(),
		"go", append([]string{"mod", "edit"}, args...)...)
	return err
}

// goModTidy runs `go mod tidy -e` inside b.rrTempPath. The -e flag continues on
// errors from replace directives that reference modules not yet present in the
// module cache — important because we apply replaces before tidy.
func (b *Builder) goModTidy(ctx context.Context) error {
	_, err := runCmd(ctx, b.log, b.rrTempPath, b.env(), "go", "mod", "tidy", "-e")
	return err
}

// applyRequires invokes `go mod edit -require=<module>@<tag>` for each plugin.
// Batching all requires in one call avoids spawning N subprocesses.
func (b *Builder) applyRequires(ctx context.Context) error {
	if len(b.plugins) == 0 {
		return errors.New("no plugins provided; use WithPlugins to add at least one")
	}
	args := make([]string, 0, len(b.plugins))
	for _, p := range b.plugins {
		args = append(args, "-require="+p.RequireArg())
	}
	return b.goModEdit(ctx, args...)
}

// applyReplaces invokes `go mod edit -replace=<old>=<new>` for each Replace.
func (b *Builder) applyReplaces(ctx context.Context) error {
	if len(b.replaces) == 0 {
		return nil
	}
	args := make([]string, 0, len(b.replaces))
	for _, r := range b.replaces {
		args = append(args, "-replace="+r.Old+"="+r.New)
	}
	return b.goModEdit(ctx, args...)
}

// applyExcludes invokes `go mod edit -exclude=<module>@<version>` for each Exclude.
func (b *Builder) applyExcludes(ctx context.Context) error {
	if len(b.excludes) == 0 {
		return nil
	}
	args := make([]string, 0, len(b.excludes))
	for _, e := range b.excludes {
		args = append(args, "-exclude="+e.Module+"@"+e.Version)
	}
	return b.goModEdit(ctx, args...)
}
