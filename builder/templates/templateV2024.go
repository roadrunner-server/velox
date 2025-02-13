package templates

const GoModTemplateV2024 string = `
module github.com/roadrunner-server/roadrunner/{{.ModuleVersion}}

go 1.24

require (
    github.com/buger/goterm v1.0.4
    github.com/dustin/go-humanize v1.0.1
	github.com/fatih/color v1.17.0
    github.com/joho/godotenv v1.5.1
    github.com/olekukonko/tablewriter v0.0.5
    github.com/spf13/cobra v1.8.0
	github.com/spf13/viper v1.19.0
    github.com/stretchr/testify v1.9.0
	go.uber.org/automaxprocs v1.5.3
	github.com/roadrunner-server/informer/v4 latest
	github.com/roadrunner-server/resetter/v4 latest
	github.com/roadrunner-server/config/v4 latest

	// Go module pseudo-version
	{{range $v := .Entries}}{{$v.Module}} {{$v.PseudoVersion}}
	{{end}}
)

exclude (
	github.com/spf13/viper v1.18.0
	github.com/spf13/viper v1.18.1
	github.com/uber-go/tally/v4 v4.1.11
	github.com/uber-go/tally/v4 v4.1.12
	go.temporal.io/api v1.26.1
)

replace github.com/uber-go/tally/v4 => github.com/uber-go/tally/v4 v4.1.10

replace (
	{{range $v := .Entries}}{{if (ne $v.Replace "")}}{{$v.Module}} => {{$v.Replace}}
	{{end}}{{end}}
)
`

const PluginsTemplateV2024 string = `
package container

import (
	"github.com/roadrunner-server/informer/v4"
	"github.com/roadrunner-server/resetter/v4"
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
