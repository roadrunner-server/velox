package builder

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// upstreamDir copies a go.mod fixture into a fresh temp directory and returns it.
func upstreamDir(t *testing.T, fixture string) string {
	t.Helper()

	dir := t.TempDir()
	dst := filepath.Join(dir, "go.mod")

	src, err := os.ReadFile(filepath.Join("testdata", fixture))
	require.NoError(t, err)
	//nolint:gosec // G703: dst is t.TempDir() joined with a constant base name
	require.NoError(t, os.WriteFile(dst, src, 0o600))
	return dir
}

func TestUpstreamModule(t *testing.T) {
	// The fixture lists informer on a single-line require and resetter inside a require block.
	b := NewBuilder(upstreamDir(t, "upstream_go.mod"))

	up, err := b.upstreamModule(t.Context())
	require.NoError(t, err)
	require.Equal(t, "github.com/roadrunner-server/roadrunner/v2025", up.Path)
	require.Equal(t, "github.com/roadrunner-server/informer/v6", up.Informer)
	require.Equal(t, "github.com/roadrunner-server/resetter/v6", up.Resetter)
}

func TestUpstreamModule_MissingInformer(t *testing.T) {
	b := NewBuilder(upstreamDir(t, "upstream_missing_informer.mod"))

	_, err := b.upstreamModule(t.Context())
	require.Error(t, err)
	require.Contains(t, err.Error(), "github.com/roadrunner-server/informer")
	require.Contains(t, err.Error(), "github.com/roadrunner-server/roadrunner/v2025")
}

func TestUpstreamModule_IndirectInformer(t *testing.T) {
	b := NewBuilder(upstreamDir(t, "upstream_indirect_informer.mod"))

	_, err := b.upstreamModule(t.Context())
	require.Error(t, err)
	require.Contains(t, err.Error(), "does not directly require github.com/roadrunner-server/informer")
}

func TestLdflags(t *testing.T) {
	got := ldflags("github.com/roadrunner-server/roadrunner/v2025", "master", "2026-01-02T03:04:05Z")
	require.Equal(t,
		"-X github.com/roadrunner-server/roadrunner/v2025/internal/meta.version=master"+
			" -X github.com/roadrunner-server/roadrunner/v2025/internal/meta.buildTime=2026-01-02T03:04:05Z",
		got,
	)
}

func TestMajorVersion(t *testing.T) {
	cases := []struct {
		name    string
		modPath string
		want    string
	}{
		{"year major", "github.com/roadrunner-server/roadrunner/v2025", "v2025"},
		{"v3", "github.com/roadrunner-server/velox/v3", "v3"},
		{"no suffix", "github.com/roadrunner-server/informer", "v1"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, majorVersion(tc.modPath))
		})
	}
}

func TestIsModuleUnder(t *testing.T) {
	require.True(t, isModuleUnder("github.com/roadrunner-server/informer", "github.com/roadrunner-server/informer"))
	require.True(t, isModuleUnder("github.com/roadrunner-server/informer/v6", "github.com/roadrunner-server/informer"))
	require.False(t, isModuleUnder("github.com/roadrunner-server/informer-x", "github.com/roadrunner-server/informer"))
}
