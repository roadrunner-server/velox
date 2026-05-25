// Package plugin describes a single RoadRunner plugin entry and how it is
// rendered into the generated container/plugins.go file.
package plugin

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
)

const prefixLen = 5

type Plugin struct {
	prefix     string
	moduleName string
	tag        string
}

// NewPlugin returns a Plugin with a deterministic alpha-only 5-letter prefix
// derived from moduleName. The prefix is used in the generated plugins.go to
// avoid import-name collisions between different plugin modules.
//
// If multiple plugins might share a prefix, call ResolvePrefixCollisions on
// the slice after construction to re-salt any duplicates.
func NewPlugin(moduleName, tag string) *Plugin {
	return &Plugin{
		prefix:     deterministicPrefix(moduleName, 0),
		moduleName: moduleName,
		tag:        tag,
	}
}

// Prefix returns the per-plugin import-name prefix used in the generated plugins.go.
func (p *Plugin) Prefix() string     { return p.prefix }
func (p *Plugin) ModuleName() string { return p.moduleName }
func (p *Plugin) Tag() string        { return p.tag }

// Require returns "moduleName tag" — the form historically embedded in go.mod
// template require() blocks. With the v3 go-mod-edit driven flow, the value is
// instead passed as `go mod edit -require=moduleName@tag`; see RequireArg.
func (p *Plugin) Require() string { return fmt.Sprintf("%s %s", p.moduleName, p.tag) }

// RequireArg returns "moduleName@tag" suitable for `go mod edit -require=...`.
func (p *Plugin) RequireArg() string { return p.moduleName + "@" + p.tag }

// Imports returns the import line embedded in the generated plugins.go:
//
//	prefix "moduleName"
func (p *Plugin) Imports() string { return fmt.Sprintf("%s %q", p.prefix, p.moduleName) }

// Code returns the plugin-registration expression embedded in plugins.go:
//
//	prefix.Plugin{}
func (p *Plugin) Code() string { return p.prefix + ".Plugin{}" }

// ResolvePrefixCollisions assigns unique prefixes to every plugin. Assignment
// order is independent of the input slice order: prefixes are computed by
// walking a copy sorted lexicographically by module name, so the same plugin
// set always yields the same prefix mapping regardless of how the caller
// happened to construct the slice. The original slice order is preserved for
// downstream rendering (the function mutates Plugin objects in place).
//
// On collision, the salt is bumped (up to maxSalt) until a unique prefix is
// found. Two plugins sharing a module name (rare) get distinct salts.
func ResolvePrefixCollisions(plugins []*Plugin) {
	const maxSalt = 1 << 16

	ordered := make([]*Plugin, len(plugins))
	copy(ordered, plugins)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].moduleName != ordered[j].moduleName {
			return ordered[i].moduleName < ordered[j].moduleName
		}
		return ordered[i].tag < ordered[j].tag
	})

	seen := make(map[string]struct{}, len(ordered))
	for _, p := range ordered {
		for salt := range maxSalt {
			cand := deterministicPrefix(p.moduleName, uint16(salt))
			if _, dup := seen[cand]; !dup {
				p.prefix = cand
				seen[cand] = struct{}{}
				break
			}
		}
	}
}

// deterministicPrefix produces an alpha-only (a-z) prefix of length prefixLen
// from sha256(moduleName||salt). With ~26^5 ≈ 11.8M outputs, collisions among
// realistic plugin sets are rare; salts > 0 are used to break the ties.
func deterministicPrefix(moduleName string, salt uint16) string {
	var buf [2]byte
	buf[0] = byte(salt >> 8)
	buf[1] = byte(salt & 0xff)
	h := sha256.New()
	_, _ = h.Write([]byte(moduleName))
	_, _ = h.Write(buf[:])
	sum := h.Sum(nil)

	var sb strings.Builder
	sb.Grow(prefixLen)
	for i := 0; sb.Len() < prefixLen && i < len(sum); i++ {
		sb.WriteByte('a' + sum[i]%26)
	}
	return sb.String()
}
