package build

import (
	"bytes"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"syscall"

	"github.com/roadrunner-server/velox/structures"
	"go.uber.org/zap"
)

const (
	// path to the file which should be generated from the template
	pluginsPath        string = "/internal/container/plugins.go"
	letterBytes               = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	goModStr           string = "go.mod"
	pluginStructureStr string = "Plugin{}"
)

type Builder struct {
	rrPath    string
	out       string
	modules   []*structures.ModulesInfo
	log       *zap.Logger
	buildArgs string
}

func NewBuilder(rrPath string, modules []*structures.ModulesInfo, out string, log *zap.Logger, buildArgs string) *Builder {
	return &Builder{
		rrPath:    rrPath,
		modules:   modules,
		buildArgs: buildArgs,
		out:       out,
		log:       log,
	}
}

func (b *Builder) Build() error {
	t := new(Template)
	t.Entries = make([]*Entry, len(b.modules))
	for i := 0; i < len(b.modules); i++ {
		e := new(Entry)

		e.Module = b.modules[i].ModuleName
		e.Prefix = RandStringBytes(5)
		e.Structure = pluginStructureStr
		e.Version = b.modules[i].Version
		e.Replace = b.modules[i].Replace

		t.Entries[i] = e
	}

	buf := new(bytes.Buffer)
	err := compileTemplate(buf, t)
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

	// remove old plugins.go
	err = os.Remove(path.Join(b.rrPath, pluginsPath))
	if err != nil {
		return err
	}

	err = os.WriteFile(path.Join(b.rrPath, pluginsPath), buf.Bytes(), os.ModePerm)
	if err != nil {
		return err
	}

	err = os.Remove(path.Join(b.rrPath, goModStr))
	if err != nil {
		return err
	}

	goModFile, err := os.Create(path.Join(b.rrPath, goModStr))
	if err != nil {
		return err
	}

	buf.Reset()

	err = compileGoModTemplate(buf, t)
	if err != nil {
		return err
	}

	_, err = goModFile.Write(buf.Bytes())
	if err != nil {
		return err
	}

	buf.Reset()

	b.log.Debug("[SWITCHING WORKING DIR]", zap.String("wd", b.rrPath))
	err = syscall.Chdir(b.rrPath)
	if err != nil {
		return err
	}

	err = b.goModTidyCmd()
	if err != nil {
		return err
	}

	for i := 0; i < len(t.Entries); i++ {
		if t.Entries[i].Replace != "" {
			continue
		}
		err = b.goGetMod(t.Entries[i].Module, t.Entries[i].Version)
		if err != nil {
			return err
		}
	}

	err = b.goModTidyCmd()
	if err != nil {
		return err
	}

	err = b.goBuildCmd(b.out)
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

func (b *Builder) goBuildCmd(out string) error {
	b.log.Info("[EXECUTING CMD]", zap.String("cmd", "go build "+b.buildArgs+" -o "+out+"cmd/rr/main.go"))
	var cmd *exec.Cmd
	if b.buildArgs != "" {
		cmd = exec.Command("go", "build", b.buildArgs, "-o", out, "cmd/rr/main.go")
	}

	cmd = exec.Command("go", "build", "-o", out, "cmd/rr/main.go")
	cmd.Stderr = b
	err := cmd.Start()
	if err != nil {
		return err
	}
	err = cmd.Wait()
	if err != nil {
		return err
	}
	return nil
}

func (b *Builder) goModTidyCmd() error {
	b.log.Info("[EXECUTING CMD]", zap.String("cmd", "go mod tidy"))
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Stderr = b
	err := cmd.Start()
	if err != nil {
		return err
	}
	err = cmd.Wait()
	if err != nil {
		return err
	}
	return nil
}

func (b *Builder) goGetMod(repo, hash string) error {
	b.log.Info("[EXECUTING CMD]", zap.String("cmd", "go get "+repo+"@"+hash))
	cmd := exec.Command("go", "get", repo+"@"+hash)
	cmd.Stderr = b

	err := cmd.Start()
	if err != nil {
		return err
	}
	err = cmd.Wait()
	if err != nil {
		return err
	}
	return nil
}

func (b *Builder) Write(d []byte) (int, error) {
	b.log.Info("[STDERR OUTPUT]", zap.ByteString("log", d))
	return len(d), nil
}
