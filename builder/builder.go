package builder

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"github.com/hashicorp/go-version"
	"github.com/roadrunner-server/velox/v2024"
	"github.com/roadrunner-server/velox/v2024/builder/templates"
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
	modules    []*velox.ModulesInfo
	log        *zap.Logger
	buildArgs  []string
}

func NewBuilder(rrTmpPath string, modules []*velox.ModulesInfo, out string, log *zap.Logger, buildArgs []string) *Builder {
	return &Builder{
		rrTempPath: rrTmpPath,
		modules:    modules,
		buildArgs:  buildArgs,
		out:        out,
		log:        log,
	}
}

// Build builds a RR based on the provided modules info
func (b *Builder) Build(rrModule string) error { //nolint:gocyclo
	t := new(templates.Template)

	module, err := validateModule(rrModule)
	if err != nil {
		return err
	}

	t.ModuleVersion = module
	t.Entries = make([]*templates.Entry, len(b.modules))
	for i := 0; i < len(b.modules); i++ {
		t.Entries[i] = &templates.Entry{
			Module:        b.modules[i].ModuleName,
			Prefix:        randStringBytes(5),
			Structure:     pluginStructureStr,
			PseudoVersion: b.modules[i].PseudoVersion,
			Replace:       b.modules[i].Replace,
		}
	}

	buf := new(bytes.Buffer)

	// compatibility with version 2
	switch t.ModuleVersion {
	case velox.V2024:
		err = templates.CompileTemplateV2024(buf, t)
		if err != nil {
			return err
		}
	case velox.V2023:
		err = templates.CompileTemplateV2023(buf, t)
		if err != nil {
			return err
		}
	case velox.V2:
		err = templates.CompileTemplateV2(buf, t)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown module version: %s", t.ModuleVersion)
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

	err = os.WriteFile(path.Join(b.rrTempPath, pluginsPath), buf.Bytes(), 0600)
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

	// compatibility with version 2
	switch t.ModuleVersion {
	case velox.V2024:
		err = templates.CompileGoModTemplate2024(buf, t)
		if err != nil {
			return err
		}
	case velox.V2023:
		err = templates.CompileGoModTemplate2023(buf, t)
		if err != nil {
			return err
		}
	case velox.V2:
		err = templates.CompileGoModTemplateV2(buf, t)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown module version: %s", t.ModuleVersion)
	}

	b.log.Debug("[RESULTING TEMPLATE]", zap.String("template", buf.String()))

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

	err = b.goModDowloadCmd()
	if err != nil {
		return err
	}

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

func (b *Builder) Write(d []byte) (int, error) {
	b.log.Debug("[STDERR OUTPUT]", zap.ByteString("log", d))
	return len(d), nil
}

func validateModule(module string) (string, error) {
	if module == "master" {
		// default branch
		return velox.V2024, nil
	}

	v, err := version.NewVersion(module)
	if err != nil {
		return "", err
	}

	// return major version (v2, v2023, etc)
	return fmt.Sprintf("v%d", v.Segments()[0]), nil
}

func randStringBytes(n int) string {
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

func (b *Builder) goModDowloadCmd() error {
	b.log.Info("[EXECUTING CMD]", zap.String("cmd", "go mod download"))
	cmd := exec.Command("go", "mod", "download")
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

func (b *Builder) getDepsReplace(repl string) []*templates.Entry {
	b.log.Info("[REPLACING DEPENDENCIES]", zap.String("dependency", repl))
	modFile, err := os.ReadFile(path.Join(repl, goModStr))
	if err != nil {
		return nil
	}

	var result []*templates.Entry //nolint:prealloc
	replaces := replaceRegexp.FindAllStringSubmatch(string(modFile), -1)
	for i := 0; i < len(replaces); i++ {
		split := strings.Split(strings.TrimSpace(replaces[i][0]), " => ")
		if len(split) != 2 {
			b.log.Error("not enough split args", zap.String("replace", replaces[i][0]))
			continue
		}

		moduleName := split[0]
		moduleReplace := split[1]

		if strings.HasPrefix(moduleReplace, ".") {
			moduleReplace = path.Join(repl, moduleReplace)
		}

		result = append(result, &templates.Entry{
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
