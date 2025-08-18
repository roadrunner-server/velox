package velox

import (
	"os"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpandEnvs(t *testing.T) {
	token := "foobarbaz"

	require.NoError(t, os.Setenv("TOKEN", token))
	c := &Config{
		Roadrunner: map[string]string{ref: "v2025.1.0"},
		Debug: &Debug{
			Enabled: true,
		},
		GitHub: &GitHub{
			Token: &Token{Token: "${TOKEN}"},
		},
		TargetPlatform: &TargetPlatform{
			OS:   "linux",
			Arch: "amd64",
		},
		Plugins: map[string]*Plugin{
			"logger": {
				Tag:        "v5.1.8",
				ModuleName: "github.com/roadrunner-server/logger/v5",
			},
		},
		Log: map[string]string{"level": "info", "mode": "production"},
	}

	require.NoError(t, c.Validate())

	assert.Equal(t, token, c.GitHub.Token.Token)
}

func TestNils(t *testing.T) {
	c := &Config{
		Log: nil,
	}

	require.Error(t, c.Validate())
}

func TestPluginsRequired(t *testing.T) {
	c := &Config{
		Roadrunner: map[string]string{ref: "v2025.1.0"},
		Log:        map[string]string{"level": "info"},
		Plugins:    nil,
	}

	require.Error(t, c.Validate())
	assert.Contains(t, c.Validate().Error(), "plugins configuration is required")
}

func TestPluginValidation(t *testing.T) {
	c := &Config{
		Roadrunner: map[string]string{ref: "v2025.1.0"},
		Log:        map[string]string{"level": "info"},
		Plugins: map[string]*Plugin{
			"invalid": {
				Tag:        "",
				ModuleName: "github.com/roadrunner-server/logger/v5",
			},
		},
	}

	require.Error(t, c.Validate())
	assert.Contains(t, c.Validate().Error(), "tag is required")
}

func TestTargetPlatformDefaults(t *testing.T) {
	c := &Config{
		Roadrunner: map[string]string{ref: "v2025.1.0"},
		Plugins: map[string]*Plugin{
			"logger": {
				Tag:        "v5.1.8",
				ModuleName: "github.com/roadrunner-server/logger/v5",
			},
		},
	}

	require.NoError(t, c.Validate())
	assert.NotNil(t, c.TargetPlatform)
	assert.Equal(t, runtime.GOOS, c.TargetPlatform.OS)
	assert.Equal(t, runtime.GOARCH, c.TargetPlatform.Arch)
}
