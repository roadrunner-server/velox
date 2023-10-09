package gitlab

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/roadrunner-server/velox"
	"github.com/xanzy/go-gitlab"
	"go.uber.org/zap"
)

/*
GLRepo represents template repository
*/
type GLRepo struct {
	client *gitlab.Client
	config *velox.Config
	log    *zap.Logger
}

func NewGLRepoInfo(cfg *velox.Config, log *zap.Logger) (*GLRepo, error) {
	glc, err := gitlab.NewClient(cfg.GitLab.Token.Token, gitlab.WithBaseURL(cfg.GitLab.BaseURL.BaseURL))
	if err != nil {
		return nil, err
	}

	return &GLRepo{
		log:    log,
		config: cfg,
		client: glc,
	}, nil
}

func (r *GLRepo) GetPluginsModData() ([]*velox.ModulesInfo, error) {
	modInfoRet := make([]*velox.ModulesInfo, 0, 5)

	for k, v := range r.config.GitLab.Plugins {
		modInfo := new(velox.ModulesInfo)
		r.log.Debug("[FETCHING PLUGIN DATA]", zap.String("repository", v.Repo), zap.String("owner", v.Owner), zap.String("folder", v.Folder), zap.String("plugin", k), zap.String("ref", v.Ref))

		if v.Ref == "" {
			return nil, errors.New("ref can't be empty")
		}

		file, resp, err := r.client.RepositoryFiles.GetFile(v.Repo, path.Join(v.Folder, "go.mod"), &gitlab.GetFileOptions{
			Ref: toPtr(v.Ref),
		})
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("bad response status: %d", resp.StatusCode)
		}

		contentStr, err := base64.StdEncoding.DecodeString(file.Content)
		if err != nil {
			return nil, err
		}

		scanner := bufio.NewScanner(bytes.NewReader(contentStr))
		// we need only the first line
		scanner.Scan()
		ret := scanner.Text()

		r.log.Debug("[READING MODULE INFO]", zap.String("plugin", k), zap.String("mod", ret))

		// module github.com/roadrunner-server/logger/v2, we split and get the second part
		retMod := strings.Split(ret, " ")
		if len(retMod) < 2 {
			return nil, fmt.Errorf("failed to parse module info for the plugin: %s", ret)
		}

		err = resp.Body.Close()
		if err != nil {
			return nil, err
		}

		modInfo.ModuleName = strings.TrimRight(retMod[1], "\n")

		r.log.Debug("[REQUESTING REPO BY REF]", zap.String("plugin", k), zap.String("ref", v.Ref))
		commits, rsp, err := r.client.Commits.ListCommits(v.Repo, &gitlab.ListCommitsOptions{
			ListOptions: gitlab.ListOptions{
				Page:    1,
				PerPage: 1,
			},
			RefName: toPtr(v.Ref),
			Until:   toPtr(time.Now()),
		})
		if err != nil {
			return nil, err
		}

		if rsp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("bad response status: %d", rsp.StatusCode)
		}

		// should be only one commit
		if len(commits) == 0 || len(commits) > 1 {
			return nil, errors.New("no commits/more than 1 commit selected")
		}

		modInfo.Version = commits[0].ID
		if len(commits[0].ID) < 12 {
			return nil, errors.New("commit SHA is too short")
		}

		// [:12] because of go.mod pseudo format specs
		modInfo.Version = commits[0].ID[:12]

		at := commits[0].CommittedDate
		if at == nil {
			return nil, errors.New("commit date is nil")
		}

		modInfo.PseudoVersion = velox.ParseModuleInfo(modInfo.ModuleName, *at, modInfo.Version)

		if v.Replace != "" {
			r.log.Debug("[REPLACE REQUESTED]", zap.String("plugin", k), zap.String("path", v.Replace))
		}

		modInfo.Replace = v.Replace
		modInfoRet = append(modInfoRet, modInfo)
	}

	return modInfoRet, nil
}

func toPtr[T any](val T) *T {
	return &val
}
