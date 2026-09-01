package velox

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/mod/semver"
)

const (
	ref                  = "ref"
	defaultBranch        = "master"
	defaultGitHubBaseURL = "https://github.com"

	// LogLevelKey / LogModeKey are the velox.toml keys for the Log map.
	LogLevelKey = "level"
	LogModeKey  = "mode"
)

var ErrWindowsTarget = errors.New("velox v3 does not support Windows targets")

// ValidateTargetOS rejects target operating systems that velox v3 does not support.
func ValidateTargetOS(goos string) error {
	if strings.EqualFold(goos, "windows") {
		return ErrWindowsTarget
	}
	return nil
}

type Config struct {
	Roadrunner     map[string]string  `mapstructure:"roadrunner"`
	Debug          *Debug             `mapstructure:"debug"`
	Log            map[string]string  `mapstructure:"log"`
	TargetPlatform *TargetPlatform    `mapstructure:"target_platform"`
	GitHub         *GitHub            `mapstructure:"github"`
	Plugins        map[string]*Plugin `mapstructure:"plugins"`
	Replaces       []Replace          `mapstructure:"replaces"`
	Excludes       []Exclude          `mapstructure:"excludes"`
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

// Replace mirrors `replace old => new` in go.mod, with the version embedded inline as "module@v1.2.3".
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
	// "=" breaks the old=new split that `go mod edit -replace` performs on the operand.
	if strings.Contains(r.Old, "=") {
		return fmt.Errorf("replace: %q in `old` must not contain '='", r.Old)
	}
	if strings.Contains(r.New, "=") {
		return fmt.Errorf("replace: %q in `new` must not contain '='", r.New)
	}
	if IsLocalPath(r.New) {
		// Only a trailing "@<semver>" is a version; "@" inside a directory name is fine.
		if _, ver, ok := strings.CutLast(r.New, "@"); ok && semver.IsValid(ver) {
			return fmt.Errorf("replace: local path %q in `new` must not include @version", r.New)
		}
	}
	return nil
}

// ValidateRef rejects refs carrying characters that are unsafe in the unquoted -ldflags value.
func ValidateRef(ref string) error {
	if ref == "" {
		return errors.New("roadrunner ref must not be empty")
	}
	for i := range len(ref) {
		c := ref[i]
		allowed := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' ||
			c == '.' || c == '_' || c == '/' || c == '+' || c == '-'
		if !allowed {
			return fmt.Errorf("roadrunner ref %q contains an unsupported character %q", ref, string(c))
		}
	}
	return nil
}

func (e Exclude) Validate() error {
	if e.Module == "" || e.Version == "" {
		return errors.New("exclude: module and version are required")
	}
	return nil
}

// Validate checks the configuration, applies the defaults, and expands ${ENV} in the GitHub token.
func (c *Config) Validate() error {
	if c.Roadrunner == nil {
		c.Roadrunner = map[string]string{}
	}
	if _, ok := c.Roadrunner[ref]; !ok {
		c.Roadrunner[ref] = defaultBranch
	}
	if err := ValidateRef(c.Roadrunner[ref]); err != nil {
		return err
	}

	if c.TargetPlatform == nil {
		c.TargetPlatform = &TargetPlatform{OS: runtime.GOOS, Arch: runtime.GOARCH}
	}
	if err := ValidateTargetOS(c.TargetPlatform.OS); err != nil {
		return err
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
