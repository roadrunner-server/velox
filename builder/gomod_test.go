package builder

import (
	"context"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// envValue returns the value of key in env, or "" when key is absent.
func envValue(env []string, key string) string {
	prefix := key + "="
	value := ""
	for _, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			value = strings.TrimPrefix(kv, prefix)
		}
	}
	return value
}

func TestRunGo_Success(t *testing.T) {
	b := NewBuilder(t.TempDir())

	res, err := b.runGo(t.Context(), "env", "GOOS")
	require.NoError(t, err)
	require.Equal(t, runtime.GOOS, strings.TrimSpace(string(res.Stdout)))
}

func TestRunGo_FailureIncludesStderr(t *testing.T) {
	b := NewBuilder(t.TempDir())

	_, err := b.runGo(t.Context(), "help", "no-such-help-topic")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown help topic")
	require.Contains(t, err.Error(), "stderr")
}

func TestRunGo_ContextCancel(t *testing.T) {
	b := NewBuilder(t.TempDir())

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := b.runGo(ctx, "env", "GOOS")
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)
}

func TestNewEnv_NativeKeepsGOPATH(t *testing.T) {
	env := newEnv(runtime.GOOS, runtime.GOARCH, false)

	require.Equal(t, runtime.GOOS, envValue(env, "GOOS"))
	require.Equal(t, runtime.GOARCH, envValue(env, "GOARCH"))
	require.Equal(t, "0", envValue(env, "CGO_ENABLED"))
	require.Equal(t, os.Getenv("GOPATH"), envValue(env, "GOPATH"))
	require.Equal(t, os.Getenv("GOCACHE"), envValue(env, "GOCACHE"))
}

func TestNewEnv_CrossKeepsCallerCaches(t *testing.T) {
	goos, goarch := "linux", "arm64"
	if goos == runtime.GOOS && goarch == runtime.GOARCH {
		goos, goarch = "darwin", "amd64"
	}

	env := newEnv(goos, goarch, true)

	require.Equal(t, goos, envValue(env, "GOOS"))
	require.Equal(t, goarch, envValue(env, "GOARCH"))
	require.Equal(t, "1", envValue(env, "CGO_ENABLED"))
	require.Equal(t, os.Getenv("GOPATH"), envValue(env, "GOPATH"))
	require.Equal(t, os.Getenv("GOCACHE"), envValue(env, "GOCACHE"))
}
