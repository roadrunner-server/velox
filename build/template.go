package build

import (
	"bytes"
	"text/template"
)

type Entry struct {
	Module    string
	Structure string
	Prefix    string
	Version   string
}

type Template struct {
	Entries []*Entry
}

var PluginsTemplate string = `
package container

import (
	"github.com/roadrunner-server/informer/v2"
	"github.com/roadrunner-server/resetter/v2"
    rrt "github.com/temporalio/roadrunner-temporal"
	{{range $v := .Entries}}{{$v.Prefix}} {{$v.Module}}
	{{end}}
)

func Plugins() []interface{} {
		return []interface{} {
		// bundled
		// informer plugin (./rr workers, ./rr workers -i)
		&informer.Plugin{},
		// resetter plugin (./rr reset)
		&resetter.Plugin{},
		// temporal plugins
		&rrt.Plugin{},
	
		// std and custom plugins
		{{range $v := .Entries}}&{{$v.Prefix}}.{{$v.Structure}},
		{{end}}
	}
}
`

func compileTemplate(buf *bytes.Buffer, data *Template) error {
	tmplt, err := template.New("plugins.go").Parse(PluginsTemplate)
	if err != nil {
		return err
	}

	err = tmplt.Execute(buf, data)
	if err != nil {
		return err
	}

	return nil
}
