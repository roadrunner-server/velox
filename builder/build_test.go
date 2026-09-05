package builder

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/roadrunner-server/velox/v3"
	"github.com/roadrunner-server/velox/v3/plugin"
)

func TestApplyGoModEdits_LatestWithReplaceAndExclude(t *testing.T) {
	useLocalProxy(t, t.TempDir())
	dir := newMainModule(t, "require "+demoModule+" "+demoOld+"\n")

	b := NewBuilder(dir,
		WithPlugins(plugin.NewPlugin("example.com/http", "latest")),
		WithReplaces([]velox.Replace{{Old: "github.com/foo/bar", New: "github.com/me/fork@v1.2.3"}}),
		WithExcludes([]velox.Exclude{{Module: "github.com/redis/go-redis/v9", Version: "v9.15.0"}}),
	)
	require.NoError(t, b.applyGoModEdits(t.Context()))

	gomod, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	require.NoError(t, err)
	require.Contains(t, string(gomod), "example.com/http latest")
	require.Contains(t, string(gomod), "replace github.com/foo/bar => github.com/me/fork v1.2.3")
	require.Contains(t, string(gomod), "exclude github.com/redis/go-redis/v9 v9.15.0")

	// The literal "latest" makes any further go mod edit call fail, which is why a single call carries every directive.
	_, err = b.runGo(t.Context(), "mod", "edit", "-json")
	require.ErrorContains(t, err, "latest")
}

func TestUserPlugins_SortedAndWithoutBundled(t *testing.T) {
	http := plugin.NewPlugin("github.com/roadrunner-server/http/v6", "latest")
	logger := plugin.NewPlugin("github.com/roadrunner-server/logger/v6", "latest")
	rpc := plugin.NewPlugin("github.com/roadrunner-server/rpc/v6", "latest")
	informer := plugin.NewPlugin("github.com/roadrunner-server/informer/v6", "v6.0.0")
	resetter := plugin.NewPlugin("github.com/roadrunner-server/resetter/v6", "v6.0.0")

	want := []*plugin.Plugin{http, logger, rpc}
	for _, in := range [][]*plugin.Plugin{
		{rpc, informer, http, resetter, logger},
		{logger, http, resetter, rpc, informer},
	} {
		b := NewBuilder(t.TempDir(), WithPlugins(in...))
		require.Equal(t, want, b.plugins)
	}
}

func TestWritePluginsGo_RendersEachModuleOnce(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "container"), 0o750))
	b := NewBuilder(dir, WithPlugins(
		plugin.NewPlugin("github.com/roadrunner-server/informer/v6", "v6.0.0"),
		plugin.NewPlugin("github.com/roadrunner-server/http/v6", "latest"),
	))
	up := upstreamModule{
		Path:     "github.com/roadrunner-server/roadrunner/v2025",
		Informer: "github.com/roadrunner-server/informer/v6",
		Resetter: "github.com/roadrunner-server/resetter/v6",
	}

	require.NoError(t, b.writePluginsGo(up))

	src, err := os.ReadFile(filepath.Join(dir, pluginsRelPath))
	require.NoError(t, err)
	require.Equal(t, 1, strings.Count(string(src), `"github.com/roadrunner-server/informer/v6"`))
	require.Equal(t, 1, strings.Count(string(src), `"github.com/roadrunner-server/http/v6"`))
}

func TestBuildArgs(t *testing.T) {
	up := upstreamModule{Path: "github.com/roadrunner-server/roadrunner/v2025"}
	const out = "/out/rr.tmp"

	t.Run("release", func(t *testing.T) {
		b := NewBuilder(t.TempDir(), WithRRVersion("master"))
		args := b.buildArgs(up, out)
		require.Equal(t, []string{"build", "-trimpath"}, args[:2])
		require.Equal(t, []string{"-o", out, rrMainGo}, args[len(args)-3:])
		require.NotContains(t, args, "-v")
		require.NotContains(t, args, "-gcflags")
		require.NotContains(t, args, "-race")
		ld := ldflagsValue(t, args)
		require.Contains(t, ld, "-X github.com/roadrunner-server/roadrunner/v2025/internal/meta.version=master")
		require.Contains(t, ld, "-X github.com/roadrunner-server/roadrunner/v2025/internal/meta.buildTime=")
		require.Contains(t, ld, "-s -w")
	})

	t.Run("debug and race", func(t *testing.T) {
		b := NewBuilder(t.TempDir(), WithRRVersion("master"), WithDebug(true), WithRace(true))
		args := b.buildArgs(up, out)
		require.Contains(t, args, "-v")
		require.Contains(t, args, "-race")
		require.Contains(t, args, "all=-N -l")
		require.Contains(t, args, "debug")
		require.NotContains(t, ldflagsValue(t, args), "-s -w")
	})
}

// ldflagsValue returns the value of the single -ldflags flag in args.
func ldflagsValue(t *testing.T, args []string) string {
	t.Helper()

	i := slices.Index(args, "-ldflags")
	require.NotEqual(t, -1, i, "no -ldflags in %q", args)
	require.Equal(t, -1, slices.Index(args[i+1:], "-ldflags"), "-ldflags given twice in %q", args)
	return args[i+1]
}

func TestValidateInputs_AbsoluteOutputDir(t *testing.T) {
	t.Chdir(t.TempDir())
	cwd, err := os.Getwd()
	require.NoError(t, err)

	b := NewBuilder("/rr/src",
		WithPlugins(plugin.NewPlugin("example.com/demo", "latest")),
		WithOutputDir("./out"),
	)
	require.NoError(t, b.validateInputs())
	require.Equal(t, filepath.Join(cwd, "out"), b.outputDir)
	require.DirExists(t, b.outputDir)
}

func TestValidateInputs_RejectsUnsafeRef(t *testing.T) {
	b := NewBuilder("/rr/src",
		WithPlugins(plugin.NewPlugin("example.com/demo", "latest")),
		WithOutputDir(t.TempDir()),
		WithRRVersion("master; rm -rf /"),
	)
	require.Error(t, b.validateInputs())
}

func TestBuildTimestamp(t *testing.T) {
	var logs bytes.Buffer
	b := NewBuilder(t.TempDir(), WithLogger(slog.New(slog.NewTextHandler(&logs, nil))))

	t.Setenv("SOURCE_DATE_EPOCH", "1700000000")
	require.Equal(t, "2023-11-14T22:13:20Z", b.buildTimestamp())
	require.Empty(t, logs.String())

	t.Setenv("SOURCE_DATE_EPOCH", "not-a-number")
	_, err := time.Parse(time.RFC3339, b.buildTimestamp())
	require.NoError(t, err)
	require.Contains(t, logs.String(), "SOURCE_DATE_EPOCH")

	t.Setenv("SOURCE_DATE_EPOCH", "")
	_, err = time.Parse(time.RFC3339, b.buildTimestamp())
	require.NoError(t, err)
}

// fakeBinary writes an executable script that prints output, standing in for the built rr.
func fakeBinary(t *testing.T, output string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), executableName)
	//nolint:gosec // G306: the smoke test runs the file, so it must be executable
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\necho '"+output+"'\n"), 0o755))
	return path
}

func TestSmokeTest_ChecksInjectedVersion(t *testing.T) {
	b := NewBuilder(t.TempDir(), WithRRVersion("v2025.1.2"))

	require.NoError(t, b.smokeTest(t.Context(), fakeBinary(t, "rr version 2025.1.2 (build time: x, go1.27.0)")))

	err := b.smokeTest(t.Context(), fakeBinary(t, "rr version local (build time: development, go1.27.0)"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "2025.1.2")
}

func TestSmokeTest_SkipsCrossCompile(t *testing.T) {
	goos := "linux"
	if runtime.GOOS == goos {
		goos = "darwin"
	}
	goarch := "arm64"
	if runtime.GOARCH == goarch {
		goarch = "amd64"
	}

	b := NewBuilder(t.TempDir(), WithRRVersion("v2025.1.2"), WithGOOS(goos))
	require.NoError(t, b.smokeTest(t.Context(), "/nonexistent"))

	b = NewBuilder(t.TempDir(), WithRRVersion("v2025.1.2"), WithGOARCH(goarch))
	require.NoError(t, b.smokeTest(t.Context(), "/nonexistent"))
}

func TestPublish(t *testing.T) {
	out := t.TempDir()
	b := NewBuilder(t.TempDir(), WithOutputDir(out))
	tmp := filepath.Join(out, executableName+".tmp")
	require.NoError(t, os.WriteFile(tmp, []byte("bin"), 0o600))

	final, err := b.publish(tmp)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(out, executableName), final)
	require.FileExists(t, final)
	require.NoFileExists(t, tmp)
}

// useFakeGo stops compilation while the source directory contains a wait file.
func useFakeGo(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	const script = `#!/bin/sh
set -eu
trap 'exit 130' INT
case "$1 $2" in
  "mod edit")
    if [ "$3" = "-json" ]; then
      printf '%s\n' '{"Module":{"Path":"example.com/rr"},"Require":[{"Path":"github.com/roadrunner-server/informer"},{"Path":"github.com/roadrunner-server/resetter"}]}'
    fi
    ;;
  "mod tidy") ;;
  "build -trimpath")
    while [ "$1" != "-o" ]; do shift; done
    cp binary "$2"
    printf '%s\n' "$2" > output-path
    while [ -f wait ]; do sleep 0.01; done
    if [ -f compile-error ]; then exit 1; fi
    ;;
  *) exit 1 ;;
esac
`
	//nolint:gosec // G306: runGo must execute the test command.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go"), []byte(script), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func newBuildFixture(t *testing.T, out, version string) *Builder {
	t.Helper()

	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "container"), 0o750))
	require.NoError(t, os.Rename(fakeBinary(t, version), filepath.Join(dir, "binary")))
	return NewBuilder(dir,
		WithPlugins(plugin.NewPlugin(demoModule, "latest")),
		WithOutputDir(out),
		WithRRVersion(version),
	)
}

func TestBuild_ConcurrentOutputDirectory(t *testing.T) {
	useFakeGo(t)
	out := t.TempDir()
	final := filepath.Join(out, executableName)
	writeFixture(t, final, "previous")
	builders := []*Builder{
		newBuildFixture(t, out, "first"),
		newBuildFixture(t, out, "second"),
	}
	type result struct {
		path string
		err  error
	}
	results := make([]chan result, len(builders))
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	var workers sync.WaitGroup
	defer func() {
		cancel()
		workers.Wait()
	}()

	for i, b := range builders {
		writeFixture(t, filepath.Join(b.rrTempPath, "wait"), "")
		results[i] = make(chan result, 1)
		workers.Go(func() {
			path, err := b.Build(ctx)
			results[i] <- result{path: path, err: err}
		})
	}

	tmpPaths := make([]string, len(builders))
	for i, b := range builders {
		pathFile := filepath.Join(b.rrTempPath, "output-path")
		require.Eventually(t, func() bool {
			data, err := os.ReadFile(pathFile)
			return err == nil && len(data) > 0
		}, 5*time.Second, 10*time.Millisecond)
		data, err := os.ReadFile(pathFile)
		require.NoError(t, err)
		tmpPaths[i] = strings.TrimSpace(string(data))
		require.FileExists(t, tmpPaths[i])
	}
	require.NotEqual(t, tmpPaths[0], tmpPaths[1])
	data, err := os.ReadFile(final)
	require.NoError(t, err)
	require.Equal(t, "previous", string(data))

	for i, b := range builders {
		require.Equal(t, out, filepath.Dir(filepath.Dir(tmpPaths[i])))
		require.FileExists(t, tmpPaths[i])
		require.NoError(t, os.Remove(filepath.Join(b.rrTempPath, "wait")))
		res := <-results[i]
		require.NoError(t, res.err)
		require.Equal(t, final, res.path)
		data, err := os.ReadFile(final)
		require.NoError(t, err)
		require.Contains(t, string(data), b.rrVersion)
		require.NoDirExists(t, filepath.Dir(tmpPaths[i]))
	}
}

func TestBuild_CleansTemporaryOutput(t *testing.T) {
	useFakeGo(t)
	for _, stage := range []string{"success", "compile", "smokeTest", "publish"} {
		t.Run(stage, func(t *testing.T) {
			out := t.TempDir()
			final := filepath.Join(out, executableName)
			previous := final
			if stage == "publish" {
				previous = filepath.Join(final, "previous")
			}
			writeFixture(t, previous, "previous")
			b := newBuildFixture(t, out, "master")
			switch stage {
			case "compile":
				writeFixture(t, filepath.Join(b.rrTempPath, "compile-error"), "")
			case "smokeTest":
				b.rrVersion = "missing"
			}

			path, err := b.Build(t.Context())
			if stage == "success" {
				require.NoError(t, err)
				require.Equal(t, final, path)
			} else {
				require.ErrorContains(t, err, stage+":")
				require.Empty(t, path)
				data, err := os.ReadFile(previous)
				require.NoError(t, err)
				require.Equal(t, "previous", string(data))
			}
			entries, err := os.ReadDir(out)
			require.NoError(t, err)
			require.Len(t, entries, 1)
			require.Equal(t, executableName, entries[0].Name())
		})
	}
}
