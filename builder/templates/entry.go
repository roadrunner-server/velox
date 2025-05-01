package templates

import (
	"bytes"
	"text/template"
)

// Entry represents all info about module
type Entry struct {
	// Module is the module name (github.com/roadrunner-server/logger/v2)
	Module string
	// StructureName is the structure name of the plugin (Plugin{})
	StructureName string
	// Prefix is the prefix for the plugin to avoid collisions
	Prefix string
	// PseudoVersion is the pseudo version of the module (v0.0.0-20210101000000-000000000000)
	PseudoVersion string
	// Replace directive, should include a path
	Replace string
}

type Template struct {
	// ModuleVersion is the main module version, like v2023.1.0
	ModuleVersion string
	Entries       []*Entry
}

func CompileTemplateV2(buf *bytes.Buffer, data *Template) error {
	tmplt, err := template.New("plugins.go").Parse(PluginsTemplateV2)
	if err != nil {
		return err
	}

	err = tmplt.Execute(buf, data)
	if err != nil {
		return err
	}

	return nil
}

func CompileGoModTemplateV2(buf *bytes.Buffer, data *Template) error {
	tmplt, err := template.New("go.mod").Parse(GoModTemplateV2)
	if err != nil {
		return err
	}

	err = tmplt.Execute(buf, data)
	if err != nil {
		return err
	}

	return nil
}

func CompileTemplateV2023(buf *bytes.Buffer, data *Template) error {
	tmplt, err := template.New("plugins.go").Parse(PluginsTemplateV2023)
	if err != nil {
		return err
	}

	err = tmplt.Execute(buf, data)
	if err != nil {
		return err
	}

	return nil
}

func CompileGoModTemplate2023(buf *bytes.Buffer, data *Template) error {
	tmplt, err := template.New("go.mod").Parse(GoModTemplateV2023)
	if err != nil {
		return err
	}

	err = tmplt.Execute(buf, data)
	if err != nil {
		return err
	}

	return nil
}

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
