package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/roadrunner-server/velox"
	"github.com/roadrunner-server/velox/builder"
	"github.com/roadrunner-server/velox/github"
	"github.com/roadrunner-server/velox/gitlab"
	veloxv1 "go.buf.build/grpc/go/roadrunner-server/api/velox/v1"
	"go.uber.org/zap"
)

type Builder struct {
	log *zap.Logger
}

func (b *Builder) Build(_ context.Context, request *veloxv1.BuildRequest) (*veloxv1.BuildResponse, error) {
	var (
		gh *velox.CodeHosting
		gl *velox.CodeHosting
		mi []*velox.ModulesInfo
	)

	if request.GetGithub() != nil {
		gh = &velox.CodeHosting{
			Token: &velox.Token{
				Token: request.GetGithub().GetToken(),
			},
			Plugins: make(map[string]*velox.PluginConfig),
		}

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
	}

	if request.GetGitlab() != nil {
		gl = &velox.CodeHosting{
			Token: &velox.Token{
				Token: request.GetGitlab().GetToken(),
			},
			Plugins: make(map[string]*velox.PluginConfig),
		}

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

	v := request.GetMetaVersion()
	if v == "" {
		v = "2.12.0"
	}

	t := request.GetBuildTime()
	if t == "" {
		t = time.Now().String()
	}

	cfg := &velox.Config{
		Roadrunner: map[string]string{"roadrunner": request.GetRoadrunnerRef()},
		GitHub:     gh,
		GitLab:     gl,
		Log:        nil,
	}

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

	tmp := filepath.Join(os.TempDir(), uuid.NewString())
	path, err := rp.DownloadTemplate(tmp, cfg.Roadrunner["ref"])
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

	outputPath := filepath.Join(tmp, "bin")
	err = builder.NewBuilder(path, pMod, outputPath, b.log, buildArgs(v, t)).Build()
	if err != nil {
		return nil, err
	}

	return &veloxv1.BuildResponse{Path: outputPath}, nil
}

func buildArgs(version, buildtime string) []string {
	return []string{"-trimpath", "-ldflags", fmt.Sprintf("-s -X github.com/roadrunner-server/roadrunner/v2/internal/meta.version=%s -X github.com/roadrunner-server/roadrunner/v2/internal/meta.buildTime=%s", version, buildtime)}
}
