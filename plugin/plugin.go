package plugin

import (
	"cmp"
	"crypto/sha256"
	"fmt"
	"go/token"
	"slices"
	"strings"
)

const (
	prefixLen = 5
	maxSalt   = 1 << 16
)

type Plugin struct {
	prefix     string
	moduleName string
	tag        string
}

// NewPlugin returns a Plugin whose import prefix is derived from moduleName alone.
func NewPlugin(moduleName, tag string) *Plugin {
	return &Plugin{
		prefix:     prefixFor(moduleName, nil),
		moduleName: moduleName,
		tag:        tag,
	}
}

// Prefix returns the per-plugin import-name prefix used in the generated plugins.go.
func (p *Plugin) Prefix() string     { return p.prefix }
func (p *Plugin) ModuleName() string { return p.moduleName }
func (p *Plugin) Tag() string        { return p.tag }

// RequireArg returns "moduleName@tag" suitable for `go mod edit -require=...`.
func (p *Plugin) RequireArg() string { return p.moduleName + "@" + p.tag }

// Imports returns the `prefix "moduleName"` import line for the generated plugins.go.
func (p *Plugin) Imports() string { return fmt.Sprintf("%s %q", p.prefix, p.moduleName) }

// Code returns the `prefix.Plugin{}` registration expression for the generated plugins.go.
func (p *Plugin) Code() string { return p.prefix + ".Plugin{}" }

// ResolvePrefixCollisions gives every plugin a unique prefix by raising the salt until the candidate is free.
func ResolvePrefixCollisions(plugins []*Plugin) {
	// Sorting a copy keeps the assignment independent of the caller slice order and leaves that order intact.
	ordered := slices.Clone(plugins)
	slices.SortStableFunc(ordered, func(a, b *Plugin) int {
		return cmp.Or(cmp.Compare(a.moduleName, b.moduleName), cmp.Compare(a.tag, b.tag))
	})

	taken := make(map[string]struct{}, len(ordered))
	for _, p := range ordered {
		p.prefix = prefixFor(p.moduleName, taken)
		taken[p.prefix] = struct{}{}
	}
}

// prefixFor returns the lowest-salt prefix of moduleName that is not taken and not a Go keyword, which the generated import declaration could not use.
func prefixFor(moduleName string, taken map[string]struct{}) string {
	for salt := range maxSalt {
		cand := deterministicPrefix(moduleName, uint16(salt))
		if _, dup := taken[cand]; dup || token.IsKeyword(cand) {
			continue
		}
		return cand
	}
	return deterministicPrefix(moduleName, 0)
}

// deterministicPrefix maps sha256(moduleName, salt) to a prefixLen prefix over a-z.
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
