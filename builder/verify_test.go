package builder

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/roadrunner-server/velox/v3/plugin"
	"github.com/stretchr/testify/require"
)

const (
	demoModule    = "example.com/demo"
	missingModule = "example.com/missing"
	demoOld       = "v1.0.0"
	demoNew       = "v1.1.0"
	demoGoMod     = "module " + demoModule + "\n\ngo 1.20\n"
)

// fileURL renders an absolute directory as the file:// URL GOPROXY accepts.
func fileURL(dir string) string {
	return (&url.URL{Scheme: "file", Path: dir}).String()
}

// writeFixture writes data to path and creates the missing parent directories.
func writeFixture(t *testing.T, path, data string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
	require.NoError(t, os.WriteFile(path, []byte(data), 0o600))
}

// newProxyDir builds a module proxy tree that serves demoModule from local disk.
func newProxyDir(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	modDir := filepath.Join(dir, "example.com", "demo", "@v")
	versions := []string{demoOld, demoNew}

	writeFixture(t, filepath.Join(modDir, "list"), strings.Join(versions, "\n")+"\n")
	for _, v := range versions {
		// `go list -m` reads the .info and .mod files only, so the proxy needs no .zip.
		writeFixture(t, filepath.Join(modDir, v+".info"),
			fmt.Sprintf(`{"Version":%q,"Time":"2020-01-01T00:00:00Z"}`, v))
		writeFixture(t, filepath.Join(modDir, v+".mod"), demoGoMod)
	}
	return dir
}

// useLocalProxy points the go tool at proxyDir and keeps every module lookup off the network.
func useLocalProxy(t *testing.T, proxyDir string) {
	t.Helper()

	// NewBuilder clones os.Environ, so a value set here reaches the go subprocess.
	t.Setenv("GOPROXY", fileURL(proxyDir))
	t.Setenv("GOSUMDB", "off")
	// A host GOPRIVATE, GONOPROXY or GONOSUMDB would send the fixture module straight to its origin.
	t.Setenv("GOPRIVATE", "")
	t.Setenv("GONOPROXY", "")
	t.Setenv("GONOSUMDB", "")
	// The fixtures ship no go.sum, so the go tool needs write access to the module files.
	t.Setenv("GOFLAGS", "-mod=mod")
	t.Setenv("GOWORK", "off")
	t.Setenv("GOTOOLCHAIN", "local")
	t.Setenv("GOMODCACHE", t.TempDir())
}

// newMainModule writes a main module whose go.mod carries the given require and replace directives.
func newMainModule(t *testing.T, directives string) string {
	t.Helper()

	dir := t.TempDir()
	writeFixture(t, filepath.Join(dir, "go.mod"), "module rrfake\n\ngo 1.26\n\n"+directives)
	return dir
}

func TestVerifyResolvedVersions_PinnedTagMatches(t *testing.T) {
	useLocalProxy(t, newProxyDir(t))
	dir := newMainModule(t, "require "+demoModule+" "+demoNew+"\n")

	b := NewBuilder(dir, WithPlugins(plugin.NewPlugin(demoModule, demoNew)))

	require.NoError(t, b.verifyResolvedVersions(t.Context()))
}

func TestVerifyResolvedVersions_PinnedTagOlderThanResolved(t *testing.T) {
	useLocalProxy(t, newProxyDir(t))
	dir := newMainModule(t, "require "+demoModule+" "+demoNew+"\n")

	b := NewBuilder(dir, WithPlugins(plugin.NewPlugin(demoModule, demoOld)))

	err := b.verifyResolvedVersions(t.Context())
	require.Error(t, err)
	require.Contains(t, err.Error(), "resolved to "+demoNew)
	require.Contains(t, err.Error(), "you requested "+demoOld)
}

func TestVerifyResolvedVersions_ReplaceSameModule(t *testing.T) {
	useLocalProxy(t, newProxyDir(t))
	// The replacement version, not the required one, is the version the check compares.
	dir := newMainModule(t, "require "+demoModule+" "+demoNew+"\n\n"+
		"replace "+demoModule+" => "+demoModule+" "+demoOld+"\n")

	b := NewBuilder(dir, WithPlugins(plugin.NewPlugin(demoModule, demoOld)))

	require.NoError(t, b.verifyResolvedVersions(t.Context()))
}

func TestVerifyResolvedVersions_ReplaceLocalDirectory(t *testing.T) {
	useLocalProxy(t, newProxyDir(t))
	dir := newMainModule(t, "require "+demoModule+" "+demoNew+"\n\n"+
		"replace "+demoModule+" => ./local\n")
	writeFixture(t, filepath.Join(dir, "local", "go.mod"), demoGoMod)

	// A directory replacement carries no version, so the pinned tag stays unchecked.
	b := NewBuilder(dir, WithPlugins(plugin.NewPlugin(demoModule, "v9.9.9")))

	require.NoError(t, b.verifyResolvedVersions(t.Context()))
}

func TestVerifyResolvedVersions_UnresolvableModule(t *testing.T) {
	useLocalProxy(t, newProxyDir(t))
	dir := newMainModule(t, "require "+missingModule+" "+demoOld+"\n")

	b := NewBuilder(dir, WithPlugins(plugin.NewPlugin(missingModule, demoOld)))

	// `go list -e` exits 0 here and reports the failure in the per-module payload.
	err := b.verifyResolvedVersions(t.Context())
	require.Error(t, err)
	require.Contains(t, err.Error(), missingModule)
}

func TestVerifyResolvedVersions_LatestSkipsResolution(t *testing.T) {
	useLocalProxy(t, t.TempDir())

	dir := t.TempDir()
	// An unparsable go.mod turns every go invocation in dir into a failure.
	writeFixture(t, filepath.Join(dir, "go.mod"), "not a go.mod directive\n")

	b := NewBuilder(dir, WithPlugins(
		plugin.NewPlugin(demoModule, "latest"),
		plugin.NewPlugin(missingModule, ""),
	))

	_, err := b.runGo(t.Context(), "list", "-m", "-e", "-json", demoModule)
	require.Error(t, err, "the fixture must make any resolution attempt fail")
	require.NoError(t, b.verifyResolvedVersions(t.Context()))
}
