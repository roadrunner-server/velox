package templates

const GoModTemplateV2024 string = `
module github.com/roadrunner-server/roadrunner/{{.RRModuleVersion}}

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

const PluginsTemplateV2024 string = `
package container

import (
	"github.com/roadrunner-server/informer/v4"
	"github.com/roadrunner-server/resetter/v4"
	{{range $v := .Imports}}{{$v}}
	{{end}}
)

func Plugins() []any {
	return []any{
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
