// Package velox ...
package velox

import (
	"fmt"
	"os"
	"runtime"

	"github.com/pkg/errors"
)

const (
	ref           string = "ref"
	defaultBranch string = "master"
	V2025         string = "v2025"
	V2024         string = "v2024"
)

type Config struct {
	// Version
	Roadrunner map[string]string `mapstructure:"roadrunner"`
	// Debug configuration
	Debug *Debug `mapstructure:"debug"`
	// Log contains log configuration
	Log map[string]string `mapstructure:"log"`
	// Target platform configuration
	TargetPlatform *TargetPlatform `mapstructure:"target_platform"`
	// GitHub configuration
	GitHub *GitHub `mapstructure:"github"`
	// Plugins configuration
	Plugins map[string]*Plugin `mapstructure:"plugins"`
}

type Debug struct {
	Enabled bool `mapstructure:"enabled"`
}

type TargetPlatform struct {
	OS   string `mapstructure:"os"`
	Arch string `mapstructure:"arch"`
}

type GitHub struct {
	Token *Token `mapstructure:"token"`
}

type Token struct {
	Token string `mapstructure:"token"`
}

type Plugin struct {
	Tag        string `mapstructure:"tag"`
	ModuleName string `mapstructure:"module_name"`
}

func (c *Config) Validate() error { //nolint:gocognit,gocyclo
	if _, ok := c.Roadrunner[ref]; !ok {
		if c.Roadrunner == nil {
			c.Roadrunner = make(map[string]string)
		}

		c.Roadrunner[ref] = defaultBranch
	}

	if c.TargetPlatform == nil {
		c.TargetPlatform = &TargetPlatform{
			OS:   runtime.GOOS,
			Arch: runtime.GOARCH,
		}
	}

	// Expand environment variables in GitHub token if present
	if c.GitHub != nil && c.GitHub.Token != nil {
		c.GitHub.Token.Token = os.ExpandEnv(c.GitHub.Token.Token)
	}

	if c.Plugins == nil {
		return errors.New("plugins configuration is required")
	}

	for _, plugin := range c.Plugins {
		if plugin == nil {
			return fmt.Errorf("plugin is required")
		}
		if plugin.Tag == "" {
			return fmt.Errorf("plugin %s tag is required", plugin.ModuleName)
		}
		if plugin.ModuleName == "" {
			return fmt.Errorf("plugin %s module name is required", plugin.Tag)
		}
	}

	if len(c.Log) == 0 {
		c.Log = map[string]string{"level": "debug", "mode": "development"}
	}

	return nil
}
