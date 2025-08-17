package templates_test

import (
	"bytes"
	"testing"

	"github.com/roadrunner-server/velox/v2025/builder/templates"
	"github.com/roadrunner-server/velox/v2025/plugin"
	"github.com/stretchr/testify/require"
)

func TestCompileV2025(t *testing.T) {
	plugins := []*plugin.Plugin{
		plugin.NewPlugin("github.com/roadrunner-server/some_plugin", "latest"),
		plugin.NewPlugin("github.com/roadrunner-server/some_plugin/v2", "v2.1.0"),
		plugin.NewPlugin("github.com/roadrunner-server/some_plugin/v22234", "v22234.5.1"),
		plugin.NewPlugin("github.com/roadrunner-server/rpc/v4", "v4.2.1"),
		plugin.NewPlugin("github.com/roadrunner-server/http/v4", "v4.5.3"),
		plugin.NewPlugin("github.com/roadrunner-server/grpc/v4", "v4.1.8"),
		plugin.NewPlugin("github.com/roadrunner-server/logger/v4", "v4.3.2"),
		plugin.NewPlugin("github.com/roadrunner-server/redis/v4", "v4.0.9"),
		plugin.NewPlugin("github.com/roadrunner-server/prometheus/v5", "v5.1.1"),
		plugin.NewPlugin("github.com/roadrunner-server/temporal/v5", "latest"),
		plugin.NewPlugin("github.com/roadrunner-server/jobs/v4", "v4.7.0"),
		plugin.NewPlugin("github.com/roadrunner-server/centrifuge/v4", "v4.2.3"),
	}

	tt := templates.NewTemplate(plugins)

	buf := new(bytes.Buffer)
	err := templates.CompileTemplateV2025(buf, tt)
	require.NoError(t, err)

	result := buf.String()

	// Verify the structure and content
	require.Contains(t, result, "package container")
	require.Contains(t, result, `"github.com/roadrunner-server/informer/v5"`)
	require.Contains(t, result, `"github.com/roadrunner-server/resetter/v5"`)
	require.Contains(t, result, "&informer.Plugin{}")
	require.Contains(t, result, "&resetter.Plugin{}")

	// Verify all plugin modules are imported and used
	for _, p := range plugins {
		require.Contains(t, result, p.Imports())
		require.Contains(t, result, "&"+p.Code())
	}
}
