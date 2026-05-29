package velox

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	ref                  = "ref"
	defaultBranch        = "master"
	defaultGitHubBaseURL = "https://github.com"

	// LogLevelKey / LogModeKey are the velox.toml keys for the Log map.
	LogLevelKey = "level"
	LogModeKey  = "mode"

	// V3 is the canonical major version for the current RoadRunner line.
	V3 = "v3"
)

type Config struct {
	// Roadrunner holds the ref (tag, branch, or SHA) under the "ref" key.
	Roadrunner map[string]string `mapstructure:"roadrunner"`
	// Debug toggles debug build flags.
	Debug *Debug `mapstructure:"debug"`
	// Log holds level/mode settings for the zap logger.
	Log map[string]string `mapstructure:"log"`
	// TargetPlatform overrides GOOS/GOARCH for cross-compilation. Defaults to host.
	TargetPlatform *TargetPlatform `mapstructure:"target_platform"`
	// GitHub configures token + (optional) GitHub Enterprise base URL.
	GitHub *GitHub `mapstructure:"github"`
	// Plugins is the map of user plugins to inject.
	Plugins map[string]*Plugin `mapstructure:"plugins"`
	// Replaces is an optional list of go.mod replace directives applied before tidy.
	Replaces []Replace `mapstructure:"replaces"`
	// Excludes is an optional list of go.mod exclude directives applied before tidy.
	Excludes []Exclude `mapstructure:"excludes"`
}

type Debug struct {
	Enabled bool `mapstructure:"enabled"`
}

type TargetPlatform struct {
	OS   string `mapstructure:"os"`
	Arch string `mapstructure:"arch"`
}

type GitHub struct {
	Token   *Token `mapstructure:"token"`
	BaseURL string `mapstructure:"base_url"`
}

type Token struct {
	Token string `mapstructure:"token"`
}

type Plugin struct {
	Tag        string `mapstructure:"tag"`
	ModuleName string `mapstructure:"module_name"`
}

// Replace mirrors `replace old => new` in go.mod.
// Both fields embed @version inline when needed (e.g., "module@v1.2.3").
// Local paths (./, ../, /abs) in New must NOT carry @version.
type Replace struct {
	New string `mapstructure:"new"`
	Old string `mapstructure:"old"`
}

// Exclude mirrors `exclude module version` in go.mod.
type Exclude struct {
	Module  string `mapstructure:"module"`
	Version string `mapstructure:"version"`
}

// IsLocalPath reports whether s denotes a local filesystem path (./, ../, or absolute).
func IsLocalPath(s string) bool {
	return strings.HasPrefix(s, "./") || strings.HasPrefix(s, "../") || filepath.IsAbs(s)
}

func (r Replace) Validate() error {
	if r.New == "" || r.Old == "" {
		return errors.New("replace: new and old are required")
	}
	if IsLocalPath(r.New) && strings.Contains(r.New, "@") {
		return fmt.Errorf("replace: local path %q in `new` must not include @version", r.New)
	}
	return nil
}

func (e Exclude) Validate() error {
	if e.Module == "" || e.Version == "" {
		return errors.New("exclude: module and version are required")
	}
	return nil
}

// Validate validates the configuration, applies defaults, and expands ${ENV} in
// the GitHub token. The Roadrunner ref defaults to "master", TargetPlatform to
// runtime GOOS/GOARCH, log to debug/development, GitHub base URL to github.com.
func (c *Config) Validate() error {
	if c.Roadrunner == nil {
		c.Roadrunner = map[string]string{}
	}
	if _, ok := c.Roadrunner[ref]; !ok {
		c.Roadrunner[ref] = defaultBranch
	}

	if c.TargetPlatform == nil {
		c.TargetPlatform = &TargetPlatform{OS: runtime.GOOS, Arch: runtime.GOARCH}
	}
	if strings.EqualFold(c.TargetPlatform.OS, "windows") {
		return errors.New("velox v3 does not support Windows targets")
	}

	if c.GitHub == nil {
		c.GitHub = &GitHub{}
	}
	if c.GitHub.Token != nil {
		c.GitHub.Token.Token = os.ExpandEnv(c.GitHub.Token.Token)
	}
	if c.GitHub.BaseURL == "" {
		c.GitHub.BaseURL = defaultGitHubBaseURL
	}

	if len(c.Plugins) == 0 {
		return errors.New("plugins configuration is required")
	}
	for name, plugin := range c.Plugins {
		if plugin == nil {
			return fmt.Errorf("plugin %q is empty", name)
		}
		if plugin.ModuleName == "" {
			return fmt.Errorf("plugin %q module name is required", name)
		}
		if plugin.Tag == "" {
			return fmt.Errorf("plugin %q (%s) tag is required", name, plugin.ModuleName)
		}
	}

	seen := make(map[string]struct{}, len(c.Replaces))
	for i, r := range c.Replaces {
		if err := r.Validate(); err != nil {
			return fmt.Errorf("replaces[%d]: %w", i, err)
		}
		if _, dup := seen[r.Old]; dup {
			return fmt.Errorf("replaces[%d]: duplicate old %q", i, r.Old)
		}
		seen[r.Old] = struct{}{}
	}
	for i, e := range c.Excludes {
		if err := e.Validate(); err != nil {
			return fmt.Errorf("excludes[%d]: %w", i, err)
		}
	}

	if len(c.Log) == 0 {
		c.Log = map[string]string{LogLevelKey: "debug", LogModeKey: "development"}
	}

	return nil
}
