package templates

import (
	"io"
	"text/template"
)

const GoModTemplateV2025 string = `
module github.com/roadrunner-server/roadrunner/{{.ModuleVersion}}

go 1.25

require (
	github.com/olekukonko/tablewriter v1.1.0
	github.com/buger/goterm v1.0.4
	github.com/dustin/go-humanize v1.0.1
	github.com/fatih/color v1.18.0
	github.com/joho/godotenv v1.5.1
	github.com/spf13/cobra v1.10.1
	github.com/spf13/viper v1.21.0
	github.com/stretchr/testify v1.10.0
	github.com/roadrunner-server/informer/v5 latest
	github.com/roadrunner-server/resetter/v5 latest
	github.com/roadrunner-server/config/v5 latest

	// Go module pseudo-version
	{{range $v := .Entries}}{{$v.Module}} {{$v.PseudoVersion}}
	{{end}}
)

replace (
	{{range $v := .Entries}}{{if (ne $v.Replace "")}}{{$v.Module}} => {{$v.Replace}}
	{{end}}{{end}}
)

exclude (
	github.com/olekukonko/tablewriter v1.1.1
	github.com/redis/go-redis/v9 v9.15.0
	github.com/redis/go-redis/v9 v9.15.1
	github.com/spf13/viper v1.18.0
	github.com/spf13/viper v1.18.1
	go.temporal.io/api v1.26.1
)
`

const PluginsTemplateV2025 string = `
package container

import (
	"github.com/roadrunner-server/informer/v5"
	"github.com/roadrunner-server/resetter/v5"
	{{range $v := .Entries}}{{$v.Prefix}} "{{$v.Module}}"
	{{end}}
)

func Plugins() []any {
		return []any {
		// bundled
		// informer plugin (./rr workers, ./rr workers -i)
		&informer.Plugin{},
		// resetter plugin (./rr reset)
		&resetter.Plugin{},

		// std and custom plugins
		{{range $v := .Entries}}&{{$v.Prefix}}.{{$v.StructureName}},
		{{end}}
	}
}
`

// CompileGoModTemplate2025 compiles the go.mod template for v2025
func CompileGoModTemplate2025(wr io.Writer, t *Template) error {
	tmpl, err := template.New("goModV2025").Parse(GoModTemplateV2025)
	if err != nil {
		return err
	}

	return tmpl.Execute(wr, t)
}
