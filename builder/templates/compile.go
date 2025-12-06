package templates

import (
	"bytes"
	"io"
	"text/template"

	"github.com/roadrunner-server/velox/v2025/plugin"
)

type Template struct {
	// RRModuleVersion is the main module version, like v2023.1.0
	RRModuleVersion string
	// Requires section in the go.mod, e.g.: module+tag
	Requires []string
	// Import names in the plugins.go, e.g. prefix+github.com/foo/bar
	Imports []string
	// code in the plugins.go, e.g. prefix+Plugin{}
	Code []string
}

// NewTemplate creates a template structure from plugins, populating imports, code, and requires arrays.
func NewTemplate(plugins []*plugin.Plugin) *Template {
	// Initialize the template with the provided plugins
	t := &Template{
		Requires: make([]string, 0, 10),
		Imports:  make([]string, 0, 10),
		Code:     make([]string, 0, 10),
	}

	for _, p := range plugins {
		t.Imports = append(t.Imports, p.Imports())
		t.Code = append(t.Code, p.Code())
		t.Requires = append(t.Requires, p.Require())
	}

	return t
}

// CompileGoModTemplate2025 generates a go.mod file for RoadRunner v2025 using the provided template data.
func CompileGoModTemplate2025(wr io.Writer, t *Template) error {
	tmpl, err := template.New("go.mod").Parse(GoModTemplateV2025)
	if err != nil {
		return err
	}

	return tmpl.Execute(wr, t)
}

// CompileTemplateV2025 generates a plugins.go file for RoadRunner v2025 using the provided template data.
func CompileTemplateV2025(buf *bytes.Buffer, data *Template) error {
	tmplt, err := template.New("plugins.go").Parse(PluginsTemplateV2025)
	if err != nil {
		return err
	}

	err = tmplt.Execute(buf, data)
	if err != nil {
		return err
	}

	return nil
}

// CompileGoModTemplate2024 generates a go.mod file for RoadRunner v2024 using the provided template data.
func CompileGoModTemplate2024(buf *bytes.Buffer, data *Template) error {
	tmplt, err := template.New("go.mod").Parse(GoModTemplateV2024)
	if err != nil {
		return err
	}

	err = tmplt.Execute(buf, data)
	if err != nil {
		return err
	}

	return nil
}

// CompileTemplateV2024 generates a plugins.go file for RoadRunner v2024 using the provided template data.
func CompileTemplateV2024(buf *bytes.Buffer, data *Template) error {
	tmplt, err := template.New("plugins.go").Parse(PluginsTemplateV2024)
	if err != nil {
		return err
	}

	err = tmplt.Execute(buf, data)
	if err != nil {
		return err
	}

	return nil
}
