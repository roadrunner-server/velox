package plugin

import (
	"strings"
	"testing"
)

func TestNewPluginIsDeterministic(t *testing.T) {
	a := NewPlugin("github.com/roadrunner-server/http/v5", "v5.1.0")
	b := NewPlugin("github.com/roadrunner-server/http/v5", "v5.1.0")
	if a.Prefix() != b.Prefix() {
		t.Fatalf("same module must yield same prefix, got %q vs %q", a.Prefix(), b.Prefix())
	}
}

func TestPrefixAlphaOnlyAndLength(t *testing.T) {
	p := NewPlugin("github.com/foo/bar/v2", "v2.0.0")
	pref := p.Prefix()
	if len(pref) != prefixLen {
		t.Fatalf("prefix length = %d, want %d", len(pref), prefixLen)
	}
	for _, r := range pref {
		if r < 'a' || r > 'z' {
			t.Fatalf("prefix %q contains non-alpha-lowercase: %q", pref, r)
		}
	}
}

func TestResolvePrefixCollisions(t *testing.T) {
	// Force a collision by reusing an identical module name twice.
	plugins := []*Plugin{
		NewPlugin("github.com/foo/bar", "v1"),
		NewPlugin("github.com/foo/bar", "v2"), // duplicate moduleName → same base prefix
	}
	ResolvePrefixCollisions(plugins)
	if plugins[0].Prefix() == plugins[1].Prefix() {
		t.Fatalf("collisions not resolved: both prefixes are %q", plugins[0].Prefix())
	}
}

func TestResolvePrefixCollisionsRealisticSet(t *testing.T) {
	mods := []string{
		"github.com/roadrunner-server/http/v5",
		"github.com/roadrunner-server/logger/v5",
		"github.com/roadrunner-server/rpc/v5",
		"github.com/roadrunner-server/metrics/v5",
		"github.com/roadrunner-server/otel/v5",
		"github.com/roadrunner-server/gzip/v5",
		"github.com/roadrunner-server/prometheus/v5",
		"github.com/roadrunner-server/centrifuge/v5",
		"github.com/temporalio/roadrunner-temporal/v5",
		"github.com/roadrunner-server/app-logger/v5",
	}
	plugins := make([]*Plugin, 0, len(mods))
	for _, m := range mods {
		plugins = append(plugins, NewPlugin(m, "v5.0.0"))
	}
	ResolvePrefixCollisions(plugins)

	seen := map[string]string{}
	for _, p := range plugins {
		if other, dup := seen[p.Prefix()]; dup {
			t.Fatalf("collision after resolve: %q and %q both got %q", other, p.ModuleName(), p.Prefix())
		}
		seen[p.Prefix()] = p.ModuleName()
	}
}

func TestImportsAndCode(t *testing.T) {
	p := NewPlugin("github.com/roadrunner-server/http/v5", "v5.1.0")
	if got := p.Imports(); !strings.HasPrefix(got, p.Prefix()+" ") || !strings.Contains(got, `"github.com/roadrunner-server/http/v5"`) {
		t.Fatalf("Imports() = %q", got)
	}
	if got, want := p.Code(), p.Prefix()+".Plugin{}"; got != want {
		t.Fatalf("Code() = %q, want %q", got, want)
	}
	if got, want := p.RequireArg(), "github.com/roadrunner-server/http/v5@v5.1.0"; got != want {
		t.Fatalf("RequireArg() = %q, want %q", got, want)
	}
}
