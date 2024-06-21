package templates

const GoModTemplateV2 string = `
module github.com/roadrunner-server/roadrunner/{{.ModuleVersion}}

go 1.20

require (
        github.com/buger/goterm v1.0.4
        github.com/dustin/go-humanize v1.0.1
        github.com/joho/godotenv v1.5.1
        github.com/olekukonko/tablewriter v0.0.5
        github.com/spf13/cobra v1.7.0
		github.com/spf13/viper v1.15.0
        github.com/stretchr/testify v1.8.2
		go.uber.org/automaxprocs v1.5.2
)

replace (
	{{range $v := .Entries}}{{if (ne $v.Replace "")}}{{$v.Module}} => {{$v.Replace}}
	{{end}}{{end}}
)
`

const PluginsTemplateV2 string = `
package container

import (
	"github.com/roadrunner-server/informer/v3"
	"github.com/roadrunner-server/resetter/v3"
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
