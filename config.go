package velox

import (
	"fmt"

	"github.com/pkg/errors"
)

type Config struct {
	Velox map[string][]string `mapstructure:"velox"`

	// Version
	Roadrunner map[string]string `mapstructure:"roadrunner"`

	// GH token
	Token map[string]string `mapstructure:"github_token"`

	// Plugins Config
	Plugins map[string]*PluginConfig `mapstructure:"plugins"`
}

type PluginConfig struct {
	Ref        string   `mapstructure:"ref"`
	Owner      string   `mapstructure:"owner"`
	Repo       string   `mapstructure:"repository"`
	Replace    string   `mapstructure:"replace"`
	BuildFlags []string `mapstructure:"build-flags"`
}

func (c *Config) Validate() error {
	if _, ok := c.Roadrunner["ref"]; !ok {
		c.Roadrunner["ref"] = "master"
	}

	if len(c.Plugins) == 0 {
		return errors.New("no plugins specified in the configuration")
	}
	for k, v := range c.Plugins {
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

	return nil
}
