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

// runGo runs the go tool with args in the RoadRunner source tree under the builder environment and captures its output.
func (b *Builder) runGo(ctx context.Context, args ...string) (runResult, error) {
	b.log.Info("executing command",
		"cmd", "go "+strings.Join(args, " "),
		"dir", b.rrTempPath,
	)

	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = b.rrTempPath
	cmd.Env = b.env
	// Cancellation sends SIGINT and falls back to SIGKILL after the wait delay.
	cmd.Cancel = func() error { return cmd.Process.Signal(os.Interrupt) }
	cmd.WaitDelay = gracefulKillTimeout

	// The runner keeps all of stdout and the last stderrCaptureLimit bytes of stderr.
	stdout := &bytes.Buffer{}
	stderr := newRingBuffer(stderrCaptureLimit)
	cmd.Stdout = stdout
	cmd.Stderr = io.MultiWriter(stderr, &slogDebugWriter{log: b.log})

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

// goModTidy runs `go mod tidy -e` so replace directives for uncached modules do not stop the build.
func (b *Builder) goModTidy(ctx context.Context) error {
	_, err := b.runGo(ctx, "mod", "tidy", "-e")
	return err
}

// applyGoModEdits applies every require, replace, and exclude directive in one `go mod edit` call. The go tool writes a non-semver version such as "latest" as given, and a later `go mod edit` call refuses to parse it.
func (b *Builder) applyGoModEdits(ctx context.Context) error {
	edits := make([]string, 0, len(b.plugins)+len(b.replaces)+len(b.excludes))
	for _, p := range b.plugins {
		edits = append(edits, "-require="+p.RequireArg())
	}
	for _, r := range b.replaces {
		edits = append(edits, "-replace="+r.Old+"="+r.New)
	}
	for _, e := range b.excludes {
		edits = append(edits, "-exclude="+e.Module+"@"+e.Version)
	}
	if len(edits) == 0 {
		return nil
	}
	_, err := b.runGo(ctx, append([]string{"mod", "edit"}, edits...)...)
	return err
}
