package plugin

import (
	"fmt"
	"math/rand"
)

const (
	pluginStructureStr string = "Plugin{}"
	letterBytes        string = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
)

type Plugin struct {
	prefix     string
	moduleName string
	tag        string
}

// NewPlugin Go module name looks like <hosting>/<owner>/<repo>/v<version> (or without version) + tag
// version example: github.com/hashicorp/golang-lru/v2 v2.0.7
// no version example: github.com/spf13/cobra v1.9.1
// NewPlugin creates a Plugin configured for the given module import path and tag and assigns a random
// 5-letter prefix to avoid import name collisions.
// The moduleName is the module import path (for example "host/owner/repo/vX"); tag is the module tag or version
// string that will be combined with the module name when requiring the module.
func NewPlugin(moduleName, tag string) *Plugin {
	return &Plugin{
		prefix:     randStringBytes(5),
		moduleName: moduleName,
		tag:        tag,
	}
}

func (p *Plugin) Prefix() string {
	return p.prefix
}

// Require returns module name + tag
func (p *Plugin) Require() string {
	return fmt.Sprintf("%s %s", p.moduleName, p.tag)
}

// Imports returns a prefix+module_name
func (p *Plugin) Imports() string {
	return fmt.Sprintf(`%s "%s"`, p.prefix, p.moduleName)
}

// Code returns a prefix+Plugin{}
func (p *Plugin) Code() string {
	return fmt.Sprintf("%s.%s", p.prefix, pluginStructureStr)
}

func randStringBytes(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))] //nolint:gosec
	}
	return string(b)
}