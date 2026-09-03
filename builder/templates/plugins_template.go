package templates

import (
	"bytes"
	"errors"
	"fmt"
	"go/format"
	"text/template"

	"github.com/roadrunner-server/velox/v3/plugin"
)

// pluginsTemplate is the parameterized plugins.go body.
const pluginsTemplate = `package container

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

//nolint:gochecknoglobals // the template parses once at package init and stays immutable
var pluginsTmpl = template.Must(template.New("plugins.go").Parse(pluginsTemplate))

// Template is the data passed to the plugins.go template.
type Template struct {
	InformerImport string
	ResetterImport string
	Imports        []string
	Code           []string
}

// NewTemplate builds the template data from the bundled module paths and the plugin set.
func NewTemplate(informer, resetter string, plugins []*plugin.Plugin) *Template {
	t := &Template{
		InformerImport: informer,
		ResetterImport: resetter,
		Imports:        make([]string, 0, len(plugins)),
		Code:           make([]string, 0, len(plugins)),
	}
	for _, p := range plugins {
		t.Imports = append(t.Imports, p.Imports())
		t.Code = append(t.Code, p.Code())
	}
	return t
}

// Render executes the plugins.go template and returns gofmt-formatted source.
func Render(t *Template) ([]byte, error) {
	if t == nil {
		return nil, errors.New("templates: template must not be nil")
	}
	if t.InformerImport == "" || t.ResetterImport == "" {
		return nil, fmt.Errorf("templates: InformerImport and ResetterImport must be set (got %q, %q)",
			t.InformerImport, t.ResetterImport)
	}

	var buf bytes.Buffer
	if err := pluginsTmpl.Execute(&buf, t); err != nil {
		return nil, fmt.Errorf("templates: execute plugins.go template: %w", err)
	}

	src, err := format.Source(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("templates: generated plugins.go is not valid Go: %w", err)
	}
	return src, nil
}
