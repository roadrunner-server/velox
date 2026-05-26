package plugin

import (
	"strings"
	"testing"
)

func TestNewPluginIsDeterministic(t *testing.T) {
	a := NewPlugin("github.com/roadrunner-server/http/v6", "v6.1.0")
	b := NewPlugin("github.com/roadrunner-server/http/v6", "v6.1.0")
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
		"github.com/roadrunner-server/http/v6",
		"github.com/roadrunner-server/logger/v6",
		"github.com/roadrunner-server/rpc/v6",
		"github.com/roadrunner-server/metrics/v6",
		"github.com/roadrunner-server/otel/v6",
		"github.com/roadrunner-server/gzip/v6",
		"github.com/roadrunner-server/prometheus/v6",
		"github.com/roadrunner-server/centrifuge/v6",
		"github.com/temporalio/roadrunner-temporal/v6",
		"github.com/roadrunner-server/app-logger/v6",
	}
	plugins := make([]*Plugin, 0, len(mods))
	for _, m := range mods {
		plugins = append(plugins, NewPlugin(m, "v6.0.0"))
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

func TestResolvePrefixCollisionsIsOrderIndependent(t *testing.T) {
	mods := []string{
		"github.com/roadrunner-server/http/v6",
		"github.com/roadrunner-server/logger/v6",
		"github.com/roadrunner-server/rpc/v6",
		"github.com/roadrunner-server/metrics/v6",
	}
	build := func(order []int) map[string]string {
		ps := make([]*Plugin, len(order))
		for i, idx := range order {
			ps[i] = NewPlugin(mods[idx], "v6.0.0")
		}
		ResolvePrefixCollisions(ps)
		out := make(map[string]string, len(ps))
		for _, p := range ps {
			out[p.ModuleName()] = p.Prefix()
		}
		return out
	}
	forward := build([]int{0, 1, 2, 3})
	reverse := build([]int{3, 2, 1, 0})
	shuffled := build([]int{2, 0, 3, 1})
	if !mapsEqual(forward, reverse) || !mapsEqual(forward, shuffled) {
		t.Fatalf("prefix assignment depends on input order:\n forward=%v\n reverse=%v\n shuffled=%v",
			forward, reverse, shuffled)
	}
}

func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func TestImportsAndCode(t *testing.T) {
	p := NewPlugin("github.com/roadrunner-server/http/v6", "v6.1.0")
	if got := p.Imports(); !strings.HasPrefix(got, p.Prefix()+" ") || !strings.Contains(got, `"github.com/roadrunner-server/http/v6"`) {
		t.Fatalf("Imports() = %q", got)
	}
	if got, want := p.Code(), p.Prefix()+".Plugin{}"; got != want {
		t.Fatalf("Code() = %q, want %q", got, want)
	}
	if got, want := p.RequireArg(), "github.com/roadrunner-server/http/v6@v6.1.0"; got != want {
		t.Fatalf("RequireArg() = %q, want %q", got, want)
	}
}
