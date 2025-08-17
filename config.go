package velox

import (
	"fmt"
	"os"

	"github.com/pkg/errors"
)

const (
	ref           string = "ref"
	defaultBranch string = "master"
	V2025         string = "v2025"
	V2024         string = "v2024"
	V2023         string = "v2023"
	V2            string = "v2"
	gitlabBaseURL string = "https://gitlab.com"
)

var DefaultRRRef = ref       //nolint:gochecknoglobals
var DefaultConfig = &Config{ //nolint:gochecknoglobals
	Roadrunner: map[string]string{
		ref: defaultBranch,
	},
	Debug: &Debug{
		Enabled: false,
	},
	Log: map[string]string{
		"level": "debug", "mode": "development",
	},
}

type Config struct {
	// Version
	Roadrunner map[string]string `mapstructure:"roadrunner"`
	// Debug configuration
	Debug *Debug `mapstructure:"debug"`
	// GitHub configuration
	GitHub *CodeHosting `mapstructure:"github"`
	// GitLab configuration
	GitLab *CodeHosting `mapstructure:"gitlab"`
	// Log contains log configuration
	Log map[string]string `mapstructure:"log"`
}

type Debug struct {
	Enabled bool `mapstructure:"enabled"`
}

type Token struct {
	Token string `mapstructure:"token"`
}

type Endpoint struct {
	BaseURL string `mapstructure:"endpoint"`
}

type CodeHosting struct {
	BaseURL *Endpoint                `mapstructure:"endpoint"`
	Token   *Token                   `mapstructure:"token"`
	Plugins map[string]*PluginConfig `mapstructure:"plugins"`
}

type PluginConfig struct {
	Ref     string `mapstructure:"ref"`
	Owner   string `mapstructure:"owner"`
	Repo    string `mapstructure:"repository"`
	Folder  string `mapstructure:"folder"`
	Replace string `mapstructure:"replace"`
}

func (c *Config) Validate() error { //nolint:gocognit,gocyclo
	if _, ok := c.Roadrunner[ref]; !ok {
		if c.Roadrunner == nil {
			c.Roadrunner = make(map[string]string)
		}

		c.Roadrunner[ref] = defaultBranch
	}

	if c.Debug == nil {
		c.Debug = &Debug{
			Enabled: false,
		}
	}

	if c.GitHub == nil || c.GitHub.Token == nil || c.GitHub.Token.Token == "" {
		return errors.New("github section should contain a token to download RoadRunner")
	}

	// section exists, but no plugin specified
	if (c.GitLab != nil && len(c.GitLab.Plugins) == 0) && (c.GitHub != nil && len(c.GitHub.Plugins) == 0) {
		return errors.New("no plugins specified in the configuration")
	}

	// we have a GitHub section
	if c.GitHub != nil {
		for k, v := range c.GitHub.Plugins {
			if v.Owner == "" {
				return fmt.Errorf("no owner specified for the plugin: %s", k)
			}

			if v.Ref == "" {
				return fmt.Errorf("no ref specified for the plugin: %s", k)
			}

			if v.Repo == "" {
				return fmt.Errorf("no repository specified for the plugin: %s", k)
			}
		}

		if c.GitHub.Token == nil || c.GitHub.Token.Token == "" {
			return errors.New("github.token should not be empty, create a token with any permissions: https://github.com/settings/tokens")
		}

		c.GitHub.Token.Token = os.ExpandEnv(c.GitHub.Token.Token)
	}

	// we have a GitLab section
	if c.GitLab != nil {
		for k, v := range c.GitLab.Plugins {
			if v.Owner == "" {
				return fmt.Errorf("no owner specified for the plugin: %s", k)
			}

			if v.Ref == "" {
				return fmt.Errorf("no ref specified for the plugin: %s", k)
			}

			if v.Repo == "" {
				return fmt.Errorf("no repository specified for the plugin: %s", k)
			}
		}

		if c.GitLab.BaseURL == nil {
			c.GitLab.BaseURL = &Endpoint{BaseURL: gitlabBaseURL}
		}

		if c.GitLab.Token == nil || c.GitLab.Token.Token == "" {
			return errors.New("gitlab.token should not be empty, create a token with at least [api, read_api] permissions: https://gitlab.com/-/profile/personal_access_tokens")
		}

		c.GitLab.Token.Token = os.ExpandEnv(c.GitLab.Token.Token)
	}

	if len(c.Log) == 0 {
		c.Log = map[string]string{"level": "debug", "mode": "development"}
	}

	return nil
}
