package builder

import (
	"bytes"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"github.com/roadrunner-server/velox/common"
	"go.uber.org/zap"
)

const (
	// path to the file which should be generated from the template
	pluginsPath        string = "/container/plugins.go"
	letterBytes               = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	goModStr           string = "go.mod"
	pluginStructureStr string = "Plugin{}"
	rrMainGo           string = "cmd/rr/main.go"
	executableName     string = "rr"
	// cleanup pattern
	cleanupPattern string = "roadrunner-server*"
)

var replaceRegexp = regexp.MustCompile("(\t| )(.+) => (.+)")

type Builder struct {
	rrTempPath string
	out        string
	modules    []*common.ModulesInfo
	log        *zap.Logger
	buildArgs  []string
}

func NewBuilder(rrTmpPath string, modules []*common.ModulesInfo, out string, log *zap.Logger, buildArgs []string) *Builder {
	return &Builder{
		rrTempPath: rrTmpPath,
		modules:    modules,
		buildArgs:  buildArgs,
		out:        out,
		log:        log,
	}
}

func (b *Builder) Build() error { //nolint:gocyclo
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

	b.log.Debug("[RESULTING TEMPLATE]", zap.String("template", buf.String()))

	f, err := os.Open(b.rrTempPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
		files, errGl := filepath.Glob(filepath.Join(os.TempDir(), cleanupPattern))
		if errGl != nil {
			return
		}

		for i := 0; i < len(files); i++ {
			b.log.Info("[CLEANING UP]", zap.String("file/folder", files[i]))
			_ = os.RemoveAll(files[i])
		}
	}()

	// remove old plugins.go
	err = os.Remove(path.Join(b.rrTempPath, pluginsPath))
	if err != nil {
		return err
	}

	err = os.WriteFile(path.Join(b.rrTempPath, pluginsPath), buf.Bytes(), os.ModePerm)
	if err != nil {
		return err
	}

	err = os.Remove(path.Join(b.rrTempPath, goModStr))
	if err != nil {
		return err
	}

	goModFile, err := os.Create(path.Join(b.rrTempPath, goModStr))
	if err != nil {
		return err
	}

	buf.Reset()

	for i := 0; i < len(t.Entries); i++ {
		if t.Entries[i].Replace != "" {
			t.Entries = append(t.Entries, b.getDepsReplace(t.Entries[i].Replace)...)
		}
	}

	err = compileGoModTemplate(buf, t)
	if err != nil {
		return err
	}

	_, err = goModFile.Write(buf.Bytes())
	if err != nil {
		return err
	}

	buf.Reset()

	b.log.Info("[SWITCHING WORKING DIR]", zap.String("wd", b.rrTempPath))
	err = syscall.Chdir(b.rrTempPath)
	if err != nil {
		return err
	}

	for i := 0; i < len(t.Entries); i++ {
		// go get only deps w/o replace
		if t.Entries[i].Replace != "" {
			continue
		}
		err = b.goGetMod(t.Entries[i].Module, t.Entries[i].Version)
		if err != nil {
			return err
		}
	}

	// upgrade to 1.18
	err = b.goModTidyCmd()
	if err != nil {
		return err
	}

	b.log.Info("[CHECKING OUTPUT DIR]", zap.String("dir", b.out))
	err = os.MkdirAll(b.out, os.ModeDir)
	if err != nil {
		return err
	}

	err = b.goBuildCmd(filepath.Join(b.rrTempPath, executableName))
	if err != nil {
		return err
	}

	b.log.Info("[MOVING EXECUTABLE]", zap.String("file", filepath.Join(b.rrTempPath, executableName)), zap.String("to", filepath.Join(b.out, executableName)))
	err = moveFile(filepath.Join(b.rrTempPath, executableName), filepath.Join(b.out, executableName))
	if err != nil {
		return err
	}

	return nil
}

func RandStringBytes(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))] //nolint:gosec
	}
	return string(b)
}

func (b *Builder) goBuildCmd(out string) error {
	var cmd *exec.Cmd
	if len(b.buildArgs) != 0 {
		buildCmdArgs := make([]string, 0, len(b.buildArgs)+5)
		buildCmdArgs = append(buildCmdArgs, "build")
		// verbose
		buildCmdArgs = append(buildCmdArgs, "-v")
		// build args
		buildCmdArgs = append(buildCmdArgs, b.buildArgs...)
		// output file
		buildCmdArgs = append(buildCmdArgs, "-o")
		// path
		buildCmdArgs = append(buildCmdArgs, out)
		// path to main.go
		buildCmdArgs = append(buildCmdArgs, rrMainGo)
		cmd = exec.Command("go", buildCmdArgs...)
	} else {
		cmd = exec.Command("go", "build", "-o", out, rrMainGo)
	}

	b.log.Info("[EXECUTING CMD]", zap.String("cmd", cmd.String()))
	cmd.Stderr = b
	cmd.Stdout = b
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
	b.log.Info("[EXECUTING CMD]", zap.String("cmd", "go mod tidy -go=1.18"))
	cmd := exec.Command("go", "mod", "tidy", "-go=1.18")
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
	cmd := exec.Command("go", "get", repo+"@"+hash) //nolint:gosec
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

func (b *Builder) getDepsReplace(repl string) []*Entry {
	b.log.Info("[REPLACING DEPENDENCIES]", zap.String("dependency", repl))
	modFile, err := os.ReadFile(path.Join(repl, goModStr))
	if err != nil {
		return nil
	}

	//nolint:prealloc
	var result []*Entry
	replaces := replaceRegexp.FindAllStringSubmatch(string(modFile), -1)
	for i := 0; i < len(replaces); i++ {
		split := strings.Split(strings.TrimSpace(replaces[i][0]), " => ")
		if len(split) != 2 {
			b.log.Error("Error while trying to split", zap.String("replace", replaces[i][0]))
			continue
		}

		moduleName := split[0]
		moduleReplace := split[1]

		if strings.HasPrefix(moduleReplace, ".") {
			moduleReplace = path.Join(repl, moduleReplace)
		}

		result = append(result, &Entry{
			Module:  moduleName,
			Replace: moduleReplace,
		})
	}

	return result
}

func moveFile(from, to string) error {
	ffInfo, err := os.Stat(from)
	if err != nil {
		return err
	}

	fFile, err := os.ReadFile(from)
	if err != nil {
		return err
	}

	toFile, err := os.Create(to)
	if err != nil {
		return err
	}

	err = toFile.Chmod(ffInfo.Mode())
	if err != nil {
		return err
	}

	_, err = toFile.Write(fFile)
	if err != nil {
		return err
	}

	return toFile.Close()
}

func (b *Builder) Write(d []byte) (int, error) {
	b.log.Debug("[STDERR OUTPUT]", zap.ByteString("log", d))
	return len(d), nil
}
