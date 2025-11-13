package templates

const GoModTemplateV2025 string = `
module github.com/roadrunner-server/roadrunner/{{.RRModuleVersion}}

go 1.25.2
toolchain go1.25.2

require (
	github.com/olekukonko/tablewriter v1.0.8
	github.com/buger/goterm v1.0.4
	github.com/dustin/go-humanize v1.0.1
	github.com/fatih/color v1.18.0
	github.com/joho/godotenv v1.5.1
	github.com/spf13/cobra v1.9.1
	github.com/spf13/viper v1.20.1
	github.com/stretchr/testify v1.10.0
	go.uber.org/automaxprocs v1.6.0
	github.com/roadrunner-server/informer/v5 latest
	github.com/roadrunner-server/resetter/v5 latest

	// format 'abcde github.com/foo/bar/<version> <tag>'
	{{range $v := .Requires}}{{$v}}
	{{end}}
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
	{{range $v := .Imports}}{{$v}}
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
		// format should use prefix as it used in the .Plugins in the Mod template
		{{range $v := .Code}}&{{$v}},
		{{end}}
	}
}
`
