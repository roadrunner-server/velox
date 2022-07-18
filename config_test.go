package velox

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpandEnvs(t *testing.T) {
	tn := time.Now().Format(time.Kitchen)
	token := "foobarbaz"

	require.NoError(t, os.Setenv("TIME", tn))
	require.NoError(t, os.Setenv("VERSION", "v2.10.5"))
	require.NoError(t, os.Setenv("TOKEN", token))
	c := &Config{
		Velox:      map[string][]string{"build_args": {"github.com/roadrunner-server/roadrunner/v2/internal/meta.buildTime=${TIME}", "github.com/roadrunner-server/roadrunner/v2/internal/meta.version=${VERSION}"}},
		Roadrunner: map[string]string{"": ""},
		GitHub: &CodeHosting{
			BaseURL: nil,
			Token:   &Token{Token: "${TOKEN}"},
			Plugins: map[string]*PluginConfig{"foo": {
				Ref:     "master",
				Owner:   "roadrunner-server",
				Repo:    "logger",
				Replace: "",
			},
			},
		},
		GitLab: &CodeHosting{
			BaseURL: nil,
			Token:   &Token{Token: "${TOKEN}"},
			Plugins: map[string]*PluginConfig{"foo": {
				Ref:     "master",
				Owner:   "roadrunner-server",
				Repo:    "logger",
				Replace: "",
			},
			},
		},
		Log: nil,
	}

	require.NoError(t, c.Validate())

	assert.Equal(t, "github.com/roadrunner-server/roadrunner/v2/internal/meta.buildTime="+tn, c.Velox["build_args"][0])
	assert.Equal(t, "github.com/roadrunner-server/roadrunner/v2/internal/meta.version=v2.10.5", c.Velox["build_args"][1])
	assert.Equal(t, token, c.GitHub.Token.Token)
	assert.Equal(t, token, c.GitLab.Token.Token)
}

func TestNils(t *testing.T) {
	c := &Config{
		Velox: map[string][]string{"build_args": {"github.com/roadrunner-server/roadrunner/v2/internal/meta.buildTime=${TIME}", "github.com/roadrunner-server/roadrunner/v2/internal/meta.version=${VERSION}"}},
		Log:   nil,
	}

	require.Error(t, c.Validate())
}
