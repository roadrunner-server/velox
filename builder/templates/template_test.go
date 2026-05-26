package templates_test

import (
	"bytes"
	"testing"

	"github.com/roadrunner-server/velox/v3/builder/templates"
	"github.com/roadrunner-server/velox/v3/plugin"
	"github.com/stretchr/testify/require"
)

func TestCompilePlugins_v6(t *testing.T) {
	plugins := []*plugin.Plugin{
		plugin.NewPlugin("github.com/roadrunner-server/some_plugin", "latest"),
		plugin.NewPlugin("github.com/roadrunner-server/some_plugin/v2", "v2.1.0"),
		plugin.NewPlugin("github.com/roadrunner-server/prometheus/v6", "v6.1.1"),
		plugin.NewPlugin("github.com/roadrunner-server/temporal/v6", "latest"),
	}
	plugin.ResolvePrefixCollisions(plugins)

	tt := templates.NewTemplate(plugins)
	tt.InformerImport = "github.com/roadrunner-server/informer/v6"
	tt.ResetterImport = "github.com/roadrunner-server/resetter/v6"

	var buf bytes.Buffer
	require.NoError(t, templates.CompilePlugins(&buf, tt))
	result := buf.String()

	require.Contains(t, result, "package container")
	require.Contains(t, result, `informer "github.com/roadrunner-server/informer/v6"`)
	require.Contains(t, result, `resetter "github.com/roadrunner-server/resetter/v6"`)
	require.Contains(t, result, "&informer.Plugin{}")
	require.Contains(t, result, "&resetter.Plugin{}")
	for _, p := range plugins {
		require.Contains(t, result, p.Imports())
		require.Contains(t, result, "&"+p.Code())
	}
}

func TestCompilePlugins_SinglePlugin(t *testing.T) {
	// Minimal case: one user plugin. Verifies the template handles a small
	// input gracefully (no off-by-one in the range over .Code).
	plugins := []*plugin.Plugin{
		plugin.NewPlugin("github.com/roadrunner-server/http/v6", "v6.0.0"),
	}
	plugin.ResolvePrefixCollisions(plugins)

	tt := templates.NewTemplate(plugins)
	tt.InformerImport = "github.com/roadrunner-server/informer/v6"
	tt.ResetterImport = "github.com/roadrunner-server/resetter/v6"

	var buf bytes.Buffer
	require.NoError(t, templates.CompilePlugins(&buf, tt))
	result := buf.String()

	require.Contains(t, result, `informer "github.com/roadrunner-server/informer/v6"`)
	require.Contains(t, result, `resetter "github.com/roadrunner-server/resetter/v6"`)
	require.Contains(t, result, plugins[0].Imports())
	require.Contains(t, result, "&"+plugins[0].Code())
}

func TestCompilePlugins_RejectsMissingBundledImports(t *testing.T) {
	tt := templates.NewTemplate([]*plugin.Plugin{
		plugin.NewPlugin("github.com/x/y", "v1.0.0"),
	})
	// Deliberately leave InformerImport / ResetterImport empty.
	err := templates.CompilePlugins(&bytes.Buffer{}, tt)
	require.Error(t, err)
	require.Contains(t, err.Error(), "InformerImport")
}

func TestParseUpstreamModules(t *testing.T) {
	const goMod = `module github.com/roadrunner-server/roadrunner/v3

go 1.26

require (
	github.com/roadrunner-server/informer/v6 v6.1.0
	github.com/roadrunner-server/resetter/v6 v6.1.0
	github.com/fatih/color v1.18.0
)
`
	informer, resetter, err := templates.ParseUpstreamModules([]byte(goMod))
	require.NoError(t, err)
	require.Equal(t, "github.com/roadrunner-server/informer/v6", informer)
	require.Equal(t, "github.com/roadrunner-server/resetter/v6", resetter)
}

func TestParseUpstreamModules_MissingInformer(t *testing.T) {
	const goMod = `module foo
go 1.26
require github.com/roadrunner-server/resetter/v6 v6.1.0
`
	_, _, err := templates.ParseUpstreamModules([]byte(goMod))
	require.Error(t, err)
	require.Contains(t, err.Error(), "informer")
}
