package build

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"path"
	"syscall"

	"github.com/roadrunner-server/velox/structures"
	"go.uber.org/zap"
)

const (
	goModContent = `
module github.com/roadrunner-server/roadrunner/v2

go 1.17
`
)

const (
	// path to the file which should be generated from the template
	pluginsPath string = "/internal/container/plugins.go"
	letterBytes        = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
)

type Builder struct {
	rrPath  string
	out     string
	modules []*structures.ModulesInfo
	log     *zap.Logger
}

func NewBuilder(rrPath string, modules []*structures.ModulesInfo, out string, log *zap.Logger) *Builder {
	return &Builder{
		rrPath:  rrPath,
		modules: modules,
		out:     out,
		log:     log,
	}
}

func (b *Builder) Build() error {
	t := new(Template)
	t.Entries = make([]*Entry, len(b.modules))
	for i := 0; i < len(b.modules); i++ {
		e := new(Entry)

		e.Module = fmt.Sprintf(`"%s"`, b.modules[i].ModuleName)
		e.Prefix = RandStringBytes(5)
		e.Structure = "Plugin{}"
		e.Version = b.modules[i].Version

		t.Entries[i] = e
	}

	pluginsGo := new(bytes.Buffer)
	err := compileTemplate(pluginsGo, t)
	if err != nil {
		return err
	}

	f, err := os.Open(b.rrPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
	}()

	err = os.Remove(path.Join(b.rrPath, "go.mod"))
	if err != nil {
		return err
	}

	goModFile, err := os.Create(path.Join(b.rrPath, "go.mod"))
	if err != nil {
		return err
	}

	_, err = goModFile.Write([]byte(goModContent))
	if err != nil {
		return err
	}

	// remove old plugins.go
	err = os.Remove(path.Join(b.rrPath, pluginsPath))
	if err != nil {
		return err
	}

	err = os.WriteFile(path.Join(b.rrPath, pluginsPath), pluginsGo.Bytes(), os.ModePerm)
	if err != nil {
		return err
	}

	// change wd to
	err = syscall.Chdir(b.rrPath)
	if err != nil {
		return err
	}

	return nil
}

func RandStringBytes(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}
