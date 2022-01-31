package github

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/go-github/v42/github"
	"github.com/roadrunner-server/velox"
	"github.com/roadrunner-server/velox/structures"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

const (
	rrOwner string = "roadrunner-server"
	rrRepo  string = "roadrunner"
	// keep in sync with the configuration
	tokenKey string = "token"
)

type GitRepository interface {
	GetGoMod(owner, repo, ref string) ([]byte, error)
}

/*
GHRepo represents template repository
*/
type GHRepo struct {
	client *github.Client
	config *velox.Config
	log    *zap.Logger
}

func NewRepoInfo(cfg *velox.Config, log *zap.Logger) *GHRepo {
	var client *http.Client

	// if token exists, use it to increase rate limiter
	if t, ok := cfg.Token[tokenKey]; ok {
		ctx := context.Background()
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: t})
		client = oauth2.NewClient(ctx, ts)
	}

	return &GHRepo{
		log:    log,
		config: cfg,
		client: github.NewClient(client),
	}
}

// DownloadTemplate downloads template repository ->
func (r *GHRepo) DownloadTemplate(version string) (string, error) {
	r.log.Debug("[GET ARCHIVE LINK]", zap.String("owner", rrOwner), zap.String("GHRepo", rrRepo), zap.String("encoding", "zip"), zap.String("ref", version))
	url, resp, err := r.client.Repositories.GetArchiveLink(context.Background(), rrOwner, rrRepo, github.Zipball, &github.RepositoryContentGetOptions{Ref: version}, true)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusFound {
		return "", fmt.Errorf("wrong response status, got: %d", resp.StatusCode)
	}

	r.log.Debug("[REQUESTING REPO]", zap.String("url", url.String()))
	request, err := r.client.NewRequest(http.MethodGet, url.String(), nil)
	if err != nil {
		return "", err
	}

	buf := new(bytes.Buffer)
	r.log.Debug("[FETCHING CONTENT]", zap.String("url", url.String()))
	do, err := r.client.Do(context.Background(), request, buf)
	if err != nil {
		return "", err
	}

	_, _ = io.Copy(io.Discard, do.Body)
	_ = do.Body.Close()

	tmp := os.TempDir()
	name := path.Join(tmp, "roadrunner-server-"+version)
	_ = os.RemoveAll(name)

	r.log.Debug("[FLUSHING DATA ON THE DISK]", zap.String("path", name+".zip"))
	f, err := os.Create(name + ".zip")
	if err != nil {
		return "", err
	}

	defer func() {
		_ = f.Close()
	}()

	n, err := f.Write(buf.Bytes())
	if err != nil {
		return "", err
	}

	r.log.Debug("[SAVED]", zap.Int("bytes written", n))

	rc, err := zip.OpenReader(name + ".zip")
	if err != nil {
		return "", err
	}

	defer func() {
		_ = rc.Close()
	}()

	// absolute filename
	dest, err := filepath.Abs(name)
	if err != nil {
		return "", err
	}

	err = os.RemoveAll(dest)
	if err != nil {
		return "", err
	}

	err = os.Mkdir(name, os.ModePerm)
	if err != nil {
		return "", err
	}

	if len(rc.File) == 0 {
		return "", errors.New("empty zip archive")
	}

	outDir := rc.File[0].Name

	for _, zf := range rc.File {
		r.log.Debug("[EXTRACTING]", zap.String("file", zf.Name), zap.String("path", dest))
		err = extract(dest, zf)
		if err != nil {
			return "", err
		}
	}
	// first name is the output path
	return filepath.Join(dest, outDir), nil //nolint:gosec
}

func extract(dest string, zf *zip.File) error {
	pt := filepath.Join(dest, zf.Name) //nolint:gosec

	if !strings.HasPrefix(pt, filepath.Clean(dest)+string(os.PathSeparator)) {
		return fmt.Errorf("invalid file path: %s", pt)
	}

	if zf.FileInfo().IsDir() {
		err := os.MkdirAll(pt, os.ModePerm)
		if err != nil {
			return err
		}
		return nil
	}

	destFile, err := os.OpenFile(pt, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, zf.Mode())
	if err != nil {
		return err
	}

	zippedFile, err := zf.Open()
	if err != nil {
		_ = destFile.Close()
		return err
	}

	// G110: Potential DoS vulnerability via decompression bomb
	_, err = io.Copy(destFile, zippedFile) //nolint:gosec
	if err != nil {
		_ = destFile.Close()
		_ = zippedFile.Close()
		return err
	}

	_ = destFile.Close()
	_ = zippedFile.Close()
	return nil
}

// https://github.com/roadrunner-server/static/archive/refs/heads/master.zip
// https://github.com/spiral/roadrunner-binary/archive/refs/tags/v2.7.0.zip

func (r *GHRepo) GetPluginsModData() ([]*structures.ModulesInfo, error) {
	modInfoRet := make([]*structures.ModulesInfo, 0, 5)

	for k, v := range r.config.Plugins {
		modInfo := new(structures.ModulesInfo)
		r.log.Debug("[FETCHING PLUGIN DATA]", zap.String("repository", v.Repo), zap.String("owner", v.Owner), zap.String("plugin", k), zap.String("ref", v.Ref))

		if v.Ref == "" {
			return nil, errors.New("ref can't be empty")
		}

		rc, resp, err := r.client.Repositories.DownloadContents(context.Background(), v.Owner, v.Repo, "go.mod", &github.RepositoryContentGetOptions{Ref: v.Ref})
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("bad response status: %d", resp.StatusCode)
		}

		rdr := bufio.NewReader(rc)
		ret, err := rdr.ReadString('\n')
		if err != nil {
			return nil, err
		}

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

		r.log.Debug("[REQUESTING COMMIT SHA-1]", zap.String("plugin", k), zap.String("ref", v.Ref))
		commits, rsp, err := r.client.Repositories.ListCommits(context.Background(), v.Owner, v.Repo, &github.CommitsListOptions{
			SHA:   v.Ref,
			Until: time.Now(),
			ListOptions: github.ListOptions{
				Page:    1,
				PerPage: 1,
			},
		})
		if err != nil {
			return nil, err
		}

		if rsp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("bad response status: %d", rsp.StatusCode)
		}

		for i := 0; i < len(commits); i++ {
			modInfo.Version = *commits[i].SHA
		}

		if v.Replace != "" {
			r.log.Debug("[REPLACE REQUESTED]", zap.String("plugin", k), zap.String("path", v.Replace))
		}

		modInfo.Replace = v.Replace
		modInfoRet = append(modInfoRet, modInfo)
	}

	return modInfoRet, nil
}
