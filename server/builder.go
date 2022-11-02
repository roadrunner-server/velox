package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/roadrunner-server/velox"
	"github.com/roadrunner-server/velox/builder"
	"github.com/roadrunner-server/velox/github"
	"github.com/roadrunner-server/velox/gitlab"
	veloxv1 "go.buf.build/grpc/go/roadrunner-server/api/velox/v1"
	"go.uber.org/zap"
)

const (
	bin            string = "bin"
	defaultVersion string = "v2.12.0"
)

type Builder struct {
	log     *zap.Logger
	cfgPool sync.Pool
}

func (b *Builder) Build(_ context.Context, request *veloxv1.BuildRequest) (*veloxv1.BuildResponse, error) { //nolint:gocyclo
	var (
		gh *velox.CodeHosting
		gl *velox.CodeHosting
		mi []*velox.ModulesInfo
	)

	if request.GetGithub() == nil {
		return nil, errors.New("github section could not be null")
	}

	gh = &velox.CodeHosting{
		Token: &velox.Token{
			Token: request.GetGithub().GetToken(),
		},
		Plugins: make(map[string]*velox.PluginConfig),
	}

	// convert proto plugins declarations into domain structures
	for i := 0; i < len(request.GetGithub().GetPlugins()); i++ {
		plugin := request.GetGithub().GetPlugins()[i]

		gh.Plugins[plugin.GetRepository()] = &velox.PluginConfig{
			Ref:     plugin.GetRef(),
			Owner:   plugin.GetOwner(),
			Repo:    plugin.GetRepository(),
			Folder:  plugin.GetFolder(),
			Replace: plugin.GetReplace(),
		}
	}

	// GitLab section is optional
	if request.GetGitlab() != nil {
		// token is mandatory
		if request.GetGitlab().GetToken() == "" {
			return nil, errors.New("should provide a GitLab token")
		}

		gl = &velox.CodeHosting{
			Token: &velox.Token{
				Token: request.GetGitlab().GetToken(),
			},
			Plugins: make(map[string]*velox.PluginConfig),
		}

		// convert proto plugins declarations into domain structures
		for i := 0; i < len(request.GetGitlab().GetPlugins()); i++ {
			plugin := request.GetGitlab().GetPlugins()[i]

			gl.Plugins[plugin.GetRepository()] = &velox.PluginConfig{
				Ref:     plugin.GetRef(),
				Owner:   plugin.GetOwner(),
				Repo:    plugin.GetRepository(),
				Folder:  plugin.GetFolder(),
				Replace: plugin.GetReplace(),
			}
		}
	}

	// get version for the RR
	v := request.GetMetaVersion()
	if v == "" {
		v = defaultVersion
	}

	// get build time
	t := request.GetBuildTime()
	if t == "" {
		t = time.Now().String()
	}

	//
	cfg := b.get()
	defer b.put(cfg)

	cfg.GitHub = gh
	cfg.GitLab = gl

	err := cfg.Validate()
	if err != nil {
		return nil, err
	}

	if request.GetGitlab() != nil {
		rp, errGL := gitlab.NewGLRepoInfo(cfg, b.log)
		if errGL != nil {
			return nil, errGL
		}

		mi, err = rp.GetPluginsModData()
		if err != nil {
			return nil, err
		}
	}

	rp := github.NewGHRepoInfo(cfg, b.log)

	// create unique tmp path for the every build
	tmp := filepath.Join(os.TempDir(), uuid.NewString())
	path, err := rp.DownloadTemplate(tmp, request.GetRoadrunnerRef())
	if err != nil {
		return nil, err
	}

	pMod, err := rp.GetPluginsModData()
	if err != nil {
		return nil, err
	}

	// append data from gitlab
	if mi != nil {
		pMod = append(pMod, mi...)
	}

	outputPath := filepath.Join(tmp, bin)
	err = builder.NewBuilder(path, pMod, outputPath, b.log, buildArgs(v, t)).Build()
	if err != nil {
		return nil, err
	}

	return &veloxv1.BuildResponse{Path: outputPath}, nil
}

func (b *Builder) get() *velox.Config {
	return b.cfgPool.Get().(*velox.Config)
}

func (b *Builder) put(cfg *velox.Config) {
	cfg.GitLab = nil
	cfg.GitHub = nil
	cfg.Velox = nil
	cfg.Log = nil
	cfg.Roadrunner = nil

	b.cfgPool.Put(cfg)
}

func buildArgs(version, buildtime string) []string {
	return []string{"-trimpath", "-ldflags", fmt.Sprintf("-s -X github.com/roadrunner-server/roadrunner/v2/internal/meta.version=%s -X github.com/roadrunner-server/roadrunner/v2/internal/meta.buildTime=%s", version, buildtime)}
}
