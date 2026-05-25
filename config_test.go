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

func TestWindowsTargetRejected(t *testing.T) {
	c := &Config{
		Roadrunner:     map[string]string{ref: "v3.0.0"},
		TargetPlatform: &TargetPlatform{OS: "windows", Arch: "amd64"},
		Plugins: map[string]*Plugin{
			"logger": {Tag: "v5.1.8", ModuleName: "github.com/roadrunner-server/logger/v5"},
		},
	}
	err := c.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Windows")
}

func TestGitHubBaseURLDefault(t *testing.T) {
	c := &Config{
		Roadrunner: map[string]string{ref: "v3.0.0"},
		Plugins: map[string]*Plugin{
			"logger": {Tag: "v5.1.8", ModuleName: "github.com/roadrunner-server/logger/v5"},
		},
	}
	require.NoError(t, c.Validate())
	require.NotNil(t, c.GitHub)
	assert.Equal(t, defaultGitHubBaseURL, c.GitHub.BaseURL)
}

func TestReplaceValidation(t *testing.T) {
	cases := []struct {
		name    string
		r       Replace
		wantErr string
	}{
		{"empty old", Replace{New: "../foo"}, "new and old are required"},
		{"empty new", Replace{Old: "github.com/foo/bar"}, "new and old are required"},
		{"local with version", Replace{New: "../foo@v1.0.0", Old: "github.com/foo/bar"}, "must not include @version"},
		{"valid local", Replace{New: "../foo", Old: "github.com/foo/bar"}, ""},
		{"valid module", Replace{New: "github.com/me/fork@v1.2.3", Old: "github.com/foo/bar@v1.2.3"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.r.Validate()
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestReplaceDuplicateOld(t *testing.T) {
	c := &Config{
		Roadrunner: map[string]string{ref: "v3.0.0"},
		Plugins: map[string]*Plugin{
			"logger": {Tag: "v5.1.8", ModuleName: "github.com/roadrunner-server/logger/v5"},
		},
		Replaces: []Replace{
			{New: "../foo", Old: "github.com/foo/bar"},
			{New: "../baz", Old: "github.com/foo/bar"},
		},
	}
	err := c.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate old")
}

func TestExcludeValidation(t *testing.T) {
	c := &Config{
		Roadrunner: map[string]string{ref: "v3.0.0"},
		Plugins: map[string]*Plugin{
			"logger": {Tag: "v5.1.8", ModuleName: "github.com/roadrunner-server/logger/v5"},
		},
		Excludes: []Exclude{{Module: "", Version: "v1.0.0"}},
	}
	err := c.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "module and version are required")
}

func TestIsLocalPath(t *testing.T) {
	cases := map[string]bool{
		"./foo":                 true,
		"../foo":                true,
		"/abs/path":             true,
		"github.com/foo":        false,
		"github.com/foo@v1.0.0": false,
	}
	for input, want := range cases {
		t.Run(input, func(t *testing.T) {
			if got := IsLocalPath(input); got != want {
				t.Errorf("IsLocalPath(%q) = %v, want %v", input, got, want)
			}
		})
	}
}
