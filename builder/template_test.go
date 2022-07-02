package builder

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

const res string = `
package container

import (
	"github.com/roadrunner-server/informer/v2"
	"github.com/roadrunner-server/resetter/v2"
	ab "github.com/roadrunner-server/rpc/v2"
	cd "github.com/roadrunner-server/http/v2"
	ef "github.com/roadrunner-server/grpc/v2"
	jk "github.com/roadrunner-server/logger/v2"
	
)

func Plugins() []interface{} {
		return []interface{} {
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
		Module:    "github.com/roadrunner-server/rpc/v2",
		Structure: "Plugin{}",
		Prefix:    "ab",
	})
	tt.Entries = append(tt.Entries, &Entry{
		Module:    "github.com/roadrunner-server/http/v2",
		Structure: "Plugin{}",
		Prefix:    "cd",
	})
	tt.Entries = append(tt.Entries, &Entry{
		Module:    "github.com/roadrunner-server/grpc/v2",
		Structure: "Plugin{}",
		Prefix:    "ef",
	})
	tt.Entries = append(tt.Entries, &Entry{
		Module:    "github.com/roadrunner-server/logger/v2",
		Structure: "Plugin{}",
		Prefix:    "jk",
	})

	buf := new(bytes.Buffer)
	err := compileTemplate(buf, tt)
	require.NoError(t, err)

	require.Equal(t, res, buf.String())
}
