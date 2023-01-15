package builder

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

const res string = `
package container

import (
	"github.com/roadrunner-server/informer/v3"
	"github.com/roadrunner-server/resetter/v3"
	ab "github.com/roadrunner-server/rpc/v3"
	cd "github.com/roadrunner-server/http/v3"
	ef "github.com/roadrunner-server/grpc/v3"
	jk "github.com/roadrunner-server/logger/v3"
	
)

func Plugins() []any {
		return []any {
		// bundled
		// informer plugin (./rr workers, ./rr workers -i)
		&informer.Plugin{},
		// resetter plugin (./rr reset)
		&resetter.Plugin{},
	
		// std and custom plugins
		&ab.Plugin{},
		&cd.Plugin{},
		&ef.Plugin{},
		&jk.Plugin{},
		
	}
}
`

func TestCompile(t *testing.T) {
	tt := &Template{
		Entries: make([]*Entry, 0, 10),
	}

	tt.Entries = append(tt.Entries, &Entry{
		Module:    "github.com/roadrunner-server/rpc/v3",
		Structure: "Plugin{}",
		Prefix:    "ab",
	})
	tt.Entries = append(tt.Entries, &Entry{
		Module:    "github.com/roadrunner-server/http/v3",
		Structure: "Plugin{}",
		Prefix:    "cd",
	})
	tt.Entries = append(tt.Entries, &Entry{
		Module:    "github.com/roadrunner-server/grpc/v3",
		Structure: "Plugin{}",
		Prefix:    "ef",
	})
	tt.Entries = append(tt.Entries, &Entry{
		Module:    "github.com/roadrunner-server/logger/v3",
		Structure: "Plugin{}",
		Prefix:    "jk",
	})

	buf := new(bytes.Buffer)
	err := compileTemplate(buf, tt)
	require.NoError(t, err)

	require.Equal(t, res, buf.String())
}
