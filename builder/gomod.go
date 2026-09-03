package builder

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const (
	gracefulKillTimeout = 15 * time.Second
	stderrCaptureLimit  = 8 * 1024
)

// runResult holds the captured output of a single subprocess invocation.
type runResult struct {
	Stdout []byte
	Stderr []byte
}

// runGo runs the go tool with args in dir under env and captures its output.
func runGo(ctx context.Context, log *slog.Logger, dir string, env []string, args ...string) (runResult, error) {
	if log != nil {
		log.Info("executing command",
			"cmd", "go "+strings.Join(args, " "),
			"dir", dir,
		)
	}

	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = dir
	cmd.Env = env
	// Cancellation sends SIGINT and falls back to SIGKILL after the wait delay.
	cmd.Cancel = func() error { return cmd.Process.Signal(os.Interrupt) }
	cmd.WaitDelay = gracefulKillTimeout

	// The runner keeps all of stdout and the last stderrCaptureLimit bytes of stderr.
	stdout := &bytes.Buffer{}
	stderr := newRingBuffer(stderrCaptureLimit)
	cmd.Stdout = stdout
	if log != nil {
		cmd.Stderr = io.MultiWriter(stderr, &slogDebugWriter{log: log})
	} else {
		cmd.Stderr = stderr
	}

	err := cmd.Run()
	res := runResult{Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}

	// go build grandchildren inherit the stderr pipe and can hold it open after the parent exits.
	if errors.Is(err, exec.ErrWaitDelay) && cmd.ProcessState != nil && cmd.ProcessState.Success() {
		err = nil
	}
	if err == nil {
		return res, nil
	}

	failed := fmt.Errorf("go failed: %w\n--- stderr (last %d bytes) ---\n%s",
		err, len(res.Stderr), res.Stderr)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return res, errors.Join(ctxErr, failed)
	}
	return res, failed
}

// runGo runs the go tool in the RoadRunner source tree with the builder environment.
func (b *Builder) runGo(ctx context.Context, args ...string) (runResult, error) {
	return runGo(ctx, b.log, b.rrTempPath, b.env, args...)
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

// slogDebugWriter forwards writes to a slog logger at debug level.
type slogDebugWriter struct{ log *slog.Logger }

func (w *slogDebugWriter) Write(p []byte) (int, error) {
	w.log.Debug("[stderr]", "data", string(p))
	return len(p), nil
}

// goModEdit runs `go mod edit args...` inside b.rrTempPath.
func (b *Builder) goModEdit(ctx context.Context, args ...string) error {
	_, err := b.runGo(ctx, append([]string{"mod", "edit"}, args...)...)
	return err
}

// goModTidy runs `go mod tidy -e` so replace directives for uncached modules do not stop the build.
func (b *Builder) goModTidy(ctx context.Context) error {
	_, err := b.runGo(ctx, "mod", "tidy", "-e")
	return err
}

// applyRequires passes one `-require=<module>@<tag>` operand per plugin to a single `go mod edit` call.
func (b *Builder) applyRequires(ctx context.Context) error {
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
