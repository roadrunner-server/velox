package builder

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/hashicorp/go-version"
	"github.com/roadrunner-server/velox/v2025"
	"github.com/roadrunner-server/velox/v2025/builder/templates"
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
	ldflags        string = "-X github.com/roadrunner-server/roadrunner/v2025/internal/meta.version=%s -X github.com/roadrunner-server/roadrunner/v2025/internal/meta.buildTime=%s"
)

var replaceRegexp = regexp.MustCompile("(\t| )(.+) => (.+)")

type Builder struct {
	// rrTempPath - path, where RR was saved
	rrTempPath string
	// outputDir - output directory
	outputDir string
	modules   []*velox.ModulesInfo
	log       *zap.Logger
	sb        *strings.Builder
	debug     bool
	rrVersion string
	goos      string
	goarch    string
}

// NewBuilder creates a new Builder with the given required parameters and optional configuration
func NewBuilder(rrTmpPath string, modules []*velox.ModulesInfo, opts ...Option) *Builder {
	b := &Builder{
		rrTempPath: rrTmpPath,
		modules:    modules,
	}

	// Apply all options
	for _, opt := range opts {
		opt(b)
	}

	return b
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
	for i := range b.modules {
		t.Entries[i] = &templates.Entry{
			Module: b.modules[i].ModuleName,
			// we need to set prefix to avoid collisions
			Prefix:        randStringBytes(5),
			StructureName: pluginStructureStr,
			PseudoVersion: b.modules[i].PseudoVersion,
			Replace:       b.modules[i].Replace,
		}
	}

	buf := new(bytes.Buffer)

	// compatibility with version 2
	switch t.ModuleVersion {
	case velox.V2025:
		err = templates.CompileTemplateV2025(buf, t)
		if err != nil {
			return err
		}
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

	b.log.Debug("template", zap.String("template", buf.String()))

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

		for i := range files {
			b.log.Info("cleaning temporary folders", zap.String("file/folder", files[i]))
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

	// reuse buffer
	buf.Reset()

	// compatibility with version 2
	switch t.ModuleVersion {
	case velox.V2025:
		err = templates.CompileGoModTemplate2025(buf, t)
		if err != nil {
			return err
		}
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

	b.log.Debug("template", zap.String("template", buf.String()))

	_, err = goModFile.Write(buf.Bytes())
	if err != nil {
		return err
	}

	// reuse buffer
	buf.Reset()

	err = b.exec([]string{"go", "mod", "download"})
	if err != nil {
		return err
	}

	err = b.exec([]string{"go", "mod", "tidy"})
	if err != nil {
		return err
	}

	b.log.Info("creating output directory", zap.String("dir", b.outputDir))
	_, err = os.Stat(b.outputDir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("stat failed for output directory %s: %w", b.outputDir, err)
	}

	if os.IsExist(err) {
		b.log.Info("output path already exists, cleaning up", zap.String("dir", b.outputDir))
		_ = os.RemoveAll(b.outputDir)
	}

	err = os.MkdirAll(b.outputDir, os.ModeDir|os.ModePerm)
	if err != nil {
		return err
	}

	// INFO: we can get go envs via go env GOOS for example, but instead we will set them manually
	err = b.goBuildCmd(filepath.Join(b.rrTempPath, executableName))
	if err != nil {
		return err
	}

	b.log.Info("moving binary", zap.String("file", filepath.Join(b.rrTempPath, executableName)), zap.String("to", filepath.Join(b.outputDir, executableName)))
	err = moveFile(filepath.Join(b.rrTempPath, executableName), filepath.Join(b.outputDir, executableName))
	if err != nil {
		return err
	}

	return nil
}

func (b *Builder) Write(d []byte) (int, error) {
	b.log.Debug("[STDERR OUTPUT]", zap.ByteString("log", d))
	if b.sb != nil {
		// error is always nil
		_, _ = b.sb.Write(d)
	}
	return len(d), nil
}

func validateModule(module string) (string, error) {
	if module == "master" {
		// default branch
		return velox.V2025, nil
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

func (b *Builder) goBuildCmd(outputPath string) error {
	var cmd *exec.Cmd

	buildCmdArgs := make([]string, 0, 5)
	// regular Go build command starts here.
	buildCmdArgs = append(buildCmdArgs, "go", "build", "-v", "-trimpath")

	// var ld []string
	switch b.debug {
	case true:
		// debug flags
		// turn off optimizations
		buildCmdArgs = append(buildCmdArgs, "-gcflags", "-N")
		// turn off inlining
		buildCmdArgs = append(buildCmdArgs, "-gcflags", "-l")
		// build with debug tags
		buildCmdArgs = append(buildCmdArgs, "-tags", "debug")
	case false:
		buildCmdArgs = append(buildCmdArgs, "-ldflags", "-s")
	}

	// LDFLAGS for version and build time, always appended
	buildCmdArgs = append(buildCmdArgs, "-ldflags")
	buildCmdArgs = append(buildCmdArgs, fmt.Sprintf(ldflags, b.rrVersion, time.Now().UTC().Format(time.RFC3339)))

	// output
	buildCmdArgs = append(buildCmdArgs, "-o")
	// path
	buildCmdArgs = append(buildCmdArgs, outputPath)
	// path to main.go
	buildCmdArgs = append(buildCmdArgs, rrMainGo)

	// gosec: don't need here, since we control the input
	cmd = exec.CommandContext(context.Background(), buildCmdArgs[0], buildCmdArgs[1:]...) //nolint: gosec
	cmd.Dir = b.rrTempPath
	// set GOOS and GOARCH if specified (used in the server command)
	if b.goos != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("GOOS=%s", b.goos))
	}
	if b.goarch != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("GOARCH=%s", b.goarch))
	}
	cmd.Env = append(cmd.Env, "CGO_ENABLED=0") // disable cgo
	hd, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get user home dir: %w", err)
	}
	cmd.Env = append(cmd.Env, fmt.Sprintf("GOPATH=%s", filepath.Join(hd, "go", b.goos, b.goarch)))
	cmd.Env = append(cmd.Env, fmt.Sprintf("GOCACHE=%s", filepath.Join(hd, "go", b.goos, b.goarch, "go-build")))

	b.log.Info("building RoadRunner", zap.String("cmd", cmd.String()))
	cmd.Stderr = b
	cmd.Stdout = b
	err = cmd.Start()
	if err != nil {
		return err
	}
	err = cmd.Wait()
	if err != nil {
		return err
	}
	return nil
}

func (b *Builder) exec(cmd []string) error {
	b.log.Info("executing command", zap.String("cmd", strings.Join(cmd, " ")))
	// gosec: this is not user-controlled input
	command := exec.CommandContext(context.Background(), cmd[0], cmd[1:]...) //nolint:gosec
	command.Stderr = b
	command.Stdout = b
	command.Dir = b.rrTempPath
	err := command.Start()
	if err != nil {
		return err
	}
	err = command.Wait()
	if err != nil {
		return err
	}
	return nil
}

func (b *Builder) getDepsReplace(repl string) []*templates.Entry {
	b.log.Info("found replace, processing", zap.String("dependency", repl))
	modFile, err := os.ReadFile(path.Join(repl, goModStr))
	if err != nil {
		return nil
	}

	var result []*templates.Entry //nolint:prealloc
	replaces := replaceRegexp.FindAllStringSubmatch(string(modFile), -1)
	for i := range replaces {
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
