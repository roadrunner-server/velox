package builder

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"strings"

	"golang.org/x/mod/module"
)

const (
	informerModule = "github.com/roadrunner-server/informer"
	resetterModule = "github.com/roadrunner-server/resetter"
	metaPkgSuffix  = "/internal/meta"
)

// upstreamModule holds the module paths read from the downloaded RoadRunner go.mod.
type upstreamModule struct {
	Path     string
	Informer string
	Resetter string
}

// upstreamModule asks the Go toolchain to describe the downloaded RoadRunner go.mod.
func (b *Builder) upstreamModule(ctx context.Context) (upstreamModule, error) {
	res, err := b.runGo(ctx, "mod", "edit", "-json")
	if err != nil {
		return upstreamModule{}, fmt.Errorf("go mod edit -json: %w", err)
	}

	var mod struct {
		Module struct {
			Path string
		}
		Require []struct {
			Path     string
			Version  string
			Indirect bool
		}
	}
	if err := json.Unmarshal(res.Stdout, &mod); err != nil {
		return upstreamModule{}, fmt.Errorf("parse upstream go.mod: %w", err)
	}

	up := upstreamModule{Path: mod.Module.Path}
	// Upstream requires both plugins directly; an indirect match names a module the binary does not link.
	for _, req := range mod.Require {
		if req.Indirect {
			continue
		}
		switch {
		case isModuleUnder(req.Path, informerModule):
			up.Informer = req.Path
		case isModuleUnder(req.Path, resetterModule):
			up.Resetter = req.Path
		}
	}

	if up.Informer == "" {
		return upstreamModule{}, fmt.Errorf("upstream go.mod (%s) does not directly require %s",
			up.Path, informerModule)
	}
	if up.Resetter == "" {
		return upstreamModule{}, fmt.Errorf("upstream go.mod (%s) does not directly require %s",
			up.Path, resetterModule)
	}
	return up, nil
}

// isModuleUnder reports whether path is base or a module nested under base.
func isModuleUnder(path, base string) bool {
	return path == base || strings.HasPrefix(path, base+"/")
}

// ldflags builds the -X flags that inject build metadata into the binary.
func ldflags(modPath, version, buildTime string) string {
	meta := modPath + metaPkgSuffix
	return fmt.Sprintf("-X %s.version=%s -X %s.buildTime=%s", meta, version, meta, buildTime)
}

// majorVersion returns the major-version suffix of a module path, "v1" when absent.
func majorVersion(modPath string) string {
	_, pathMajor, ok := module.SplitPathVersion(modPath)
	if !ok || pathMajor == "" {
		return "v1"
	}
	return strings.TrimPrefix(pathMajor, "/")
}
