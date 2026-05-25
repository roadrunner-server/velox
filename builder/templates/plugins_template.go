// Package templates renders the generated container/plugins.go that registers
// RoadRunner plugins. With Velox v3 the go.mod is no longer templated: only
// plugins.go is. The informer/resetter major version is read from the
// downloaded RR's own go.mod, so a single template covers every RR major.
package templates

import (
	"fmt"
	"io"
	"regexp"
	"text/template"

	"github.com/roadrunner-server/velox/v3/plugin"
)

// PluginsTemplate is the parameterized plugins.go body.
//
// InformerImport / ResetterImport hold the full module paths discovered in the
// upstream RR's go.mod (e.g. "github.com/roadrunner-server/informer/v6").
//
// Imports / Code come from the user-supplied plugin set; each entry is already
// prefixed with the deterministic 5-letter alias from plugin.Plugin.
const PluginsTemplate = `package container

import (
	informer "{{.InformerImport}}"
	resetter "{{.ResetterImport}}"
{{range $v := .Imports}}	{{$v}}
{{end}})

// Plugins returns the static plugin list compiled into this RoadRunner binary.
func Plugins() []any {
	return []any{
		// bundled
		&informer.Plugin{},
		&resetter.Plugin{},
		// user-supplied (matches order in velox.toml)
{{range $v := .Code}}		&{{$v}},
{{end}}	}
}
`

// Template is the data passed to the plugins.go template.
type Template struct {
	// InformerImport / ResetterImport are the full module paths for the
	// bundled informer and resetter plugins, discovered from upstream go.mod.
	InformerImport string
	ResetterImport string
	// Imports is the ordered list of `prefix "module"` lines.
	Imports []string
	// Code is the ordered list of `prefix.Plugin{}` initializers.
	Code []string
}

// NewTemplate constructs a Template by extracting Imports / Code from plugins.
// InformerImport / ResetterImport remain empty here; the builder fills them in
// after parsing the upstream go.mod.
func NewTemplate(plugins []*plugin.Plugin) *Template {
	t := &Template{
		Imports: make([]string, 0, len(plugins)),
		Code:    make([]string, 0, len(plugins)),
	}
	for _, p := range plugins {
		t.Imports = append(t.Imports, p.Imports())
		t.Code = append(t.Code, p.Code())
	}
	return t
}

// CompilePlugins renders the plugins.go template into w.
func CompilePlugins(w io.Writer, t *Template) error {
	if t.InformerImport == "" || t.ResetterImport == "" {
		return fmt.Errorf("templates: InformerImport and ResetterImport must be set (got %q, %q)",
			t.InformerImport, t.ResetterImport)
	}
	tmpl, err := template.New("plugins.go").Parse(PluginsTemplate)
	if err != nil {
		return err
	}
	return tmpl.Execute(w, t)
}

// informerLineRe / resetterLineRe match the require lines in an upstream go.mod
// that pin informer and resetter. Both modules use semantic import versioning
// (e.g. /v6), so the regex captures the full module path with its /vN suffix.
var (
	informerLineRe = regexp.MustCompile(`(?m)^\s*(github\.com/roadrunner-server/informer/v\d+)\s+`)
	resetterLineRe = regexp.MustCompile(`(?m)^\s*(github\.com/roadrunner-server/resetter/v\d+)\s+`)
)

// ParseUpstreamModules extracts the full informer and resetter module paths
// from the bytes of an upstream RoadRunner go.mod. The paths include the /vN
// major-version suffix so the generated import is bit-exact with what the RR
// repository ships.
func ParseUpstreamModules(goMod []byte) (informer, resetter string, err error) {
	if m := informerLineRe.FindSubmatch(goMod); m != nil {
		informer = string(m[1])
	}
	if m := resetterLineRe.FindSubmatch(goMod); m != nil {
		resetter = string(m[1])
	}
	if informer == "" {
		return "", "", fmt.Errorf("templates: could not find informer/vN require line in upstream go.mod")
	}
	if resetter == "" {
		return "", "", fmt.Errorf("templates: could not find resetter/vN require line in upstream go.mod")
	}
	return informer, resetter, nil
}
