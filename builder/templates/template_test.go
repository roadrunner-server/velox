package templates_test

import (
	"go/format"
	"go/parser"
	"go/token"
	"testing"

	"github.com/roadrunner-server/velox/v3/builder/templates"
	"github.com/roadrunner-server/velox/v3/plugin"
	"github.com/stretchr/testify/require"
)

const (
	informerImport = "github.com/roadrunner-server/informer/v6"
	resetterImport = "github.com/roadrunner-server/resetter/v6"
)

func TestRender_v6(t *testing.T) {
	plugins := []*plugin.Plugin{
		plugin.NewPlugin("github.com/roadrunner-server/some_plugin", "latest"),
		plugin.NewPlugin("github.com/roadrunner-server/some_plugin/v2", "v2.1.0"),
		plugin.NewPlugin("github.com/roadrunner-server/prometheus/v6", "v6.1.1"),
		plugin.NewPlugin("github.com/roadrunner-server/temporal/v6", "latest"),
	}
	plugin.ResolvePrefixCollisions(plugins)

	src, err := templates.Render(templates.NewTemplate(informerImport, resetterImport, plugins))
	require.NoError(t, err)
	result := string(src)

	require.Contains(t, result, "package container")
	require.Contains(t, result, `informer "`+informerImport+`"`)
	require.Contains(t, result, `resetter "`+resetterImport+`"`)
	require.Contains(t, result, "&informer.Plugin{}")
	require.Contains(t, result, "&resetter.Plugin{}")
	for _, p := range plugins {
		require.Contains(t, result, p.Imports())
		require.Contains(t, result, "&"+p.Code())
	}
}

func TestRender_SinglePlugin(t *testing.T) {
	// Minimal case: one user plugin, to catch an off-by-one in the range over .Code.
	plugins := []*plugin.Plugin{
		plugin.NewPlugin("github.com/roadrunner-server/http/v6", "v6.0.0"),
	}
	plugin.ResolvePrefixCollisions(plugins)

	src, err := templates.Render(templates.NewTemplate(informerImport, resetterImport, plugins))
	require.NoError(t, err)
	result := string(src)

	require.Contains(t, result, `informer "`+informerImport+`"`)
	require.Contains(t, result, `resetter "`+resetterImport+`"`)
	require.Contains(t, result, plugins[0].Imports())
	require.Contains(t, result, "&"+plugins[0].Code())
}

func TestRender_RejectsMissingBundledImports(t *testing.T) {
	tt := templates.NewTemplate("", "", []*plugin.Plugin{
		plugin.NewPlugin("github.com/x/y", "v1.0.0"),
	})
	_, err := templates.Render(tt)
	require.Error(t, err)
	require.Contains(t, err.Error(), "InformerImport")
}

func TestRender_RejectsNilTemplate(t *testing.T) {
	_, err := templates.Render(nil)
	require.Error(t, err)
}

func TestRender_OutputParses(t *testing.T) {
	plugins := []*plugin.Plugin{
		plugin.NewPlugin("github.com/roadrunner-server/http/v6", "v6.0.0"),
		plugin.NewPlugin("github.com/roadrunner-server/jobs/v6", "latest"),
		plugin.NewPlugin("github.com/temporalio/roadrunner-temporal/v6", "latest"),
	}
	plugin.ResolvePrefixCollisions(plugins)

	src, err := templates.Render(templates.NewTemplate(informerImport, resetterImport, plugins))
	require.NoError(t, err)

	_, err = parser.ParseFile(token.NewFileSet(), "plugins.go", src, parser.AllErrors)
	require.NoError(t, err)

	// Render already formats, so a second pass must be a no-op.
	formatted, err := format.Source(src)
	require.NoError(t, err)
	require.Equal(t, string(src), string(formatted))
}

// TestRender_Golden pins the exact output bytes, which stay stable because the prefixes come from sha256(moduleName).
func TestRender_Golden(t *testing.T) {
	plugins := []*plugin.Plugin{
		plugin.NewPlugin("github.com/roadrunner-server/http/v6", "v6.0.0"),
		plugin.NewPlugin("github.com/roadrunner-server/logger/v6", "v6.0.0"),
		plugin.NewPlugin("github.com/roadrunner-server/rpc/v6", "v6.0.0"),
	}
	plugin.ResolvePrefixCollisions(plugins)

	src, err := templates.Render(templates.NewTemplate(informerImport, resetterImport, plugins))
	require.NoError(t, err)
	require.Equal(t, goldenPluginsGo, string(src))
}

const goldenPluginsGo = `package container

import (
	ukleg "github.com/roadrunner-server/http/v6"
	informer "github.com/roadrunner-server/informer/v6"
	yvmxt "github.com/roadrunner-server/logger/v6"
	resetter "github.com/roadrunner-server/resetter/v6"
	xxfns "github.com/roadrunner-server/rpc/v6"
)

// Plugins returns the static plugin list compiled into this RoadRunner binary.
func Plugins() []any {
	return []any{
		// bundled
		&informer.Plugin{},
		&resetter.Plugin{},
		// user-supplied (matches order in velox.toml)
		&ukleg.Plugin{},
		&yvmxt.Plugin{},
		&xxfns.Plugin{},
	}
}
`
