package builder

import (
	"bytes"
	"strings"
	"testing"

	"github.com/roadrunner-server/velox/v2024/builder/templates"
	"github.com/stretchr/testify/require"
)

const res string = `
package container

import (
	"github.com/roadrunner-server/informer/v4"
	"github.com/roadrunner-server/resetter/v4"
	aba "github.com/roadrunner-server/some_plugin"
	abc "github.com/roadrunner-server/some_plugin/v2"
	abd "github.com/roadrunner-server/some_plugin/v22234"
	ab "github.com/roadrunner-server/rpc/v4"
	cd "github.com/roadrunner-server/http/v4"
	ef "github.com/roadrunner-server/grpc/v4"
	jk "github.com/roadrunner-server/logger/v4"

)

func Plugins() []any {
		return []any {
		// bundled
		// informer plugin (./rr workers, ./rr workers -i)
		&informer.Plugin{},
		// resetter plugin (./rr reset)
		&resetter.Plugin{},

		// std and custom plugins
		&aba.Plugin{},
		&abc.Plugin{},
		&abd.Plugin{},
		&ab.Plugin{},
		&cd.Plugin{},
		&ef.Plugin{},
		&jk.Plugin{},
	}
}
`

func TestCompile(t *testing.T) {
	tt := &templates.Template{
		Entries: make([]*templates.Entry, 0, 10),
	}

	tt.Entries = append(tt.Entries, &templates.Entry{
		Module:        "github.com/roadrunner-server/some_plugin",
		StructureName: "Plugin{}",
		Prefix:        "aba",
	})

	tt.Entries = append(tt.Entries, &templates.Entry{
		Module:        "github.com/roadrunner-server/some_plugin/v2",
		StructureName: "Plugin{}",
		Prefix:        "abc",
	})

	tt.Entries = append(tt.Entries, &templates.Entry{
		Module:        "github.com/roadrunner-server/some_plugin/v22234",
		StructureName: "Plugin{}",
		Prefix:        "abd",
	})

	tt.Entries = append(tt.Entries, &templates.Entry{
		Module:        "github.com/roadrunner-server/rpc/v4",
		StructureName: "Plugin{}",
		Prefix:        "ab",
	})
	tt.Entries = append(tt.Entries, &templates.Entry{
		Module:        "github.com/roadrunner-server/http/v4",
		StructureName: "Plugin{}",
		Prefix:        "cd",
	})
	tt.Entries = append(tt.Entries, &templates.Entry{
		Module:        "github.com/roadrunner-server/grpc/v4",
		StructureName: "Plugin{}",
		Prefix:        "ef",
	})
	tt.Entries = append(tt.Entries, &templates.Entry{
		Module:        "github.com/roadrunner-server/logger/v4",
		StructureName: "Plugin{}",
		Prefix:        "jk",
	})

	buf := new(bytes.Buffer)
	err := templates.CompileTemplateV2024(buf, tt)
	require.NoError(t, err)

	bufstr := buf.String()
	bufstr = strings.ReplaceAll(bufstr, "\t", "")
	bufstr = strings.ReplaceAll(bufstr, "\n", "")

	resstr := strings.ReplaceAll(res, "\t", "")
	resstr = strings.ReplaceAll(resstr, "\n", "")

	require.Equal(t, resstr, bufstr)
}
