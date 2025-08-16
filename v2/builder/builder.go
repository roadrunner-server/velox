package builder

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashicorp/go-version"
	"github.com/roadrunner-server/velox/v2025"
	"github.com/roadrunner-server/velox/v2025/v2/builder/templates"
	"github.com/roadrunner-server/velox/v2025/v2/plugin"
	"go.uber.org/zap"
)

const (
	// path to the file which should be generated from the template
	pluginsPath           string = "/container/plugins.go"
	goModStr              string = "go.mod"
	rrMainGo              string = "cmd/rr/main.go"
	executableName        string = "rr"
	executableNameWindows string = "rr.exe"
	// cleanup pattern
	cleanupPattern string = "roadrunner-server*"
	ldflags        string = "-X github.com/roadrunner-server/roadrunner/v2025/internal/meta.version=%s -X github.com/roadrunner-server/roadrunner/v2025/internal/meta.buildTime=%s"
)

type Builder struct {
	// rrTempPath - path, where RR was saved
	rrTempPath string
	// outputDir - output directory
	outputDir string
	log       *zap.Logger
	sb        *strings.Builder
	plugins   []*plugin.Plugin
	debug     bool
	rrVersion string
	goos      string
	goarch    string
}

// NewBuilder creates a new Builder with the given required parameters and optional configuration
func NewBuilder(rrTmpPath string, opts ...Option) *Builder {
	b := &Builder{
		rrTempPath: rrTmpPath,
	}

	// Apply all options
	for _, opt := range opts {
		opt(b)
	}

	return b
}

// Build builds a RR based on the provided modules info
func (b *Builder) Build(rrRef string) error { //nolint:gocyclo
	if len(b.plugins) == 0 {
		return fmt.Errorf("please, use WithPlugins to add plugins to the RR build")
	}

	t := templates.NewTemplate(b.plugins)

	module, err := validateModule(rrRef)
	if err != nil {
		return err
	}

	t.RRModuleVersion = module
	buf := new(bytes.Buffer)

	// compatibility with version 2
	switch t.RRModuleVersion {
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
	default:
		return fmt.Errorf("unknown module version: %s", t.RRModuleVersion)
	}

	b.log.Debug("template", zap.String("template", buf.String()))

	f, err := os.Open(b.rrTempPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
		// clean output directory, remove everything except RR binary
		files, errGl := filepath.Glob(filepath.Join(b.outputDir, cleanupPattern))
		if errGl != nil {
			return
		}

		for i := range files {
			b.log.Info("cleaning temporary folders", zap.String("file/folder", files[i]))
			_ = os.RemoveAll(files[i])
		}
	}()

	// remove old plugins.go
	err = os.Remove(filepath.Join(b.rrTempPath, pluginsPath))
	if err != nil {
		return err
	}

	err = os.WriteFile(filepath.Join(b.rrTempPath, pluginsPath), buf.Bytes(), 0600)
	if err != nil {
		return err
	}

	err = os.Remove(filepath.Join(b.rrTempPath, goModStr))
	if err != nil {
		return err
	}

	goModFile, err := os.Create(filepath.Join(b.rrTempPath, goModStr))
	if err != nil {
		return err
	}

	// reuse buffer
	buf.Reset()

	// compatibility with version 2
	switch t.RRModuleVersion {
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
	default:
		return fmt.Errorf("unknown module version: %s", t.RRModuleVersion)
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
	err = b.goBuildCmd(filepath.Join(b.rrTempPath, generateExecutableName(b.goos)))
	if err != nil {
		return err
	}

	b.log.Info("moving binary", zap.String("file", filepath.Join(b.rrTempPath, generateExecutableName(b.goos))), zap.String("to", filepath.Join(b.outputDir, generateExecutableName(b.goos))))
	err = moveFile(filepath.Join(b.rrTempPath, generateExecutableName(b.goos)), filepath.Join(b.outputDir, generateExecutableName(b.goos)))
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

// for Windows we should use .exe pattern
func generateExecutableName(goos string) string {
	if strings.ToLower(goos) == "windows" {
		return executableNameWindows
	}
	return executableName
}
