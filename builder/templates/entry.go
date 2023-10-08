package templates

import (
	"bytes"
	"text/template"
)

// Entry represents all info about module
type Entry struct {
	Time      string
	Module    string
	Structure string
	Prefix    string
	Version   string
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
