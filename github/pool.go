package github

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/v74/github"
	"github.com/roadrunner-server/velox/v2025"
	"go.uber.org/zap"
)

const (
	gomod   string = "go.mod"
	modLine string = "module"
)

type processor struct {
	maxWorkers int
	errs       []error
	wg         sync.WaitGroup
	mu         sync.Mutex
	log        *zap.Logger
	queueCh    chan *pcfg
	modinfo    []*velox.ModulesInfo
	client     *github.Client
}

type pcfg struct {
	pluginCfg *velox.PluginConfig
	name      string
}

func newPool(log *zap.Logger, client *github.Client) *processor {
	p := &processor{
		maxWorkers: 10,
		log:        log,
		client:     client,
		modinfo:    make([]*velox.ModulesInfo, 0, 10),
		queueCh:    make(chan *pcfg, 100),
		wg:         sync.WaitGroup{},
		mu:         sync.Mutex{},
		errs:       make([]error, 0, 1),
	}

	// start the processor
	p.run()

	return p
}

func (p *processor) run() {
	for range p.maxWorkers {
		go func() {
			for v := range p.queueCh {
				modInfo := new(velox.ModulesInfo)
				p.log.Debug("fetching plugin data",
					zap.String("repository", v.pluginCfg.Repo),
					zap.String("owner", v.pluginCfg.Owner),
					zap.String("folder", v.pluginCfg.Folder),
					zap.String("plugin", v.name),
					zap.String("ref", v.pluginCfg.Ref),
				)

				if v.pluginCfg.Ref == "" {
					p.appendErr(errors.New("ref can't be empty"))
					continue
				}

				rc, resp, err := p.client.Repositories.DownloadContents(context.Background(),
					v.pluginCfg.Owner,
					v.pluginCfg.Repo,
					filepath.Join(v.pluginCfg.Folder, gomod), &github.RepositoryContentGetOptions{Ref: v.pluginCfg.Ref},
				)
				if err != nil {
					p.appendErr(err)
					continue
				}

				if resp.StatusCode != http.StatusOK {
					p.appendErr(fmt.Errorf("bad response status: %d", resp.StatusCode))
					continue
				}

				scanner := bufio.NewScanner(rc)
				for scanner.Scan() {
					line := scanner.Text()
					switch { //nolint:gocritic
					case strings.HasPrefix(line, modLine):
						p.log.Debug("reading module info", zap.String("plugin", v.name), zap.String("module", line))

						// module github.com/roadrunner-server/logger/v2, we split and get the second part
						retMod := strings.Split(line, " ")
						if len(retMod) < 2 || len(retMod) > 2 {
							p.appendErr(fmt.Errorf("failed to parse module info for the plugin: %s", line))
							continue
						}

						modInfo.ModuleName = strings.TrimRight(retMod[1], "\n")
						goto out
					}
				}

			out:
				if errs := scanner.Err(); errs != nil {
					p.appendErr(errs)
					continue
				}

				err = resp.Body.Close()
				if err != nil {
					p.log.Warn("failed to close response body, continuing", zap.Any("error", err))
				}

				p.log.Debug("requesting commit", zap.String("plugin", v.name), zap.String("ref", v.pluginCfg.Ref))
				commits, rsp, err := p.client.Repositories.ListCommits(context.Background(), v.pluginCfg.Owner, v.pluginCfg.Repo, &github.CommitsListOptions{
					SHA:   v.pluginCfg.Ref,
					Until: time.Now(),
					ListOptions: github.ListOptions{
						Page:    1,
						PerPage: 1,
					},
				})
				if err != nil {
					p.appendErr(err)
					continue
				}

				if rsp.StatusCode != http.StatusOK {
					p.appendErr(fmt.Errorf("bad response status: %d", rsp.StatusCode))
					continue
				}

				if len(commits) == 0 {
					p.appendErr(errors.New("no commits in the repository"))
					continue
				}

				// should be only one commit
				at := commits[0].GetCommit().GetCommitter().GetDate()
				// [:12] because of go.mod pseudo format specs
				if len(commits[0].GetSHA()) < 12 {
					p.appendErr(fmt.Errorf("commit SHA is too short: %s", commits[0].GetSHA()))
					continue
				}

				modInfo.Version = commits[0].GetSHA()[:12]
				modInfo.PseudoVersion = velox.ParseModuleInfo(modInfo.ModuleName, at.Time, commits[0].GetSHA()[:12])

				if v.pluginCfg.Replace != "" {
					modInfo.Replace = v.pluginCfg.Replace
					p.log.Debug("found replace, applying", zap.String("plugin", v.name), zap.String("path", v.pluginCfg.Replace))
				}

				p.mu.Lock()
				p.modinfo = append(p.modinfo, modInfo)
				p.mu.Unlock()

				p.wg.Done()
			}
		}()
	}
}

func (p *processor) appendErr(err error) {
	p.mu.Lock()
	p.errs = append(p.errs, err)
	p.mu.Unlock()
	p.wg.Done()
}

func (p *processor) add(pjob *pcfg) {
	p.wg.Add(1)
	p.queueCh <- pjob
}

func (p *processor) errors() []error {
	p.mu.Lock()
	defer p.mu.Unlock()
	errs := make([]error, len(p.errs))
	copy(errs, p.errs)
	return errs
}

func (p *processor) moduleinfo() []*velox.ModulesInfo {
	p.mu.Lock()
	defer p.mu.Unlock()
	modinfo := make([]*velox.ModulesInfo, len(p.modinfo))
	copy(modinfo, p.modinfo)
	return modinfo
}

func (p *processor) wait() {
	p.wg.Wait()
}

func (p *processor) stop() {
	close(p.queueCh)
}
