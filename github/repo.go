package github

import (
	"archive/zip"
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

	"github.com/google/go-github/v53/github"
	"github.com/roadrunner-server/velox"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

const (
	rrOwner string = "roadrunner-server"
	rrRepo  string = "roadrunner"
	zipExt  string = ".zip"
)

/*
GHRepo represents template repository
*/
type GHRepo struct {
	client *github.Client
	config *velox.Config
	log    *zap.Logger
}

func NewGHRepoInfo(cfg *velox.Config, log *zap.Logger) *GHRepo {
	var client *http.Client

	// if a token exists, use it to increase rate limiter
	if t := cfg.GitHub.Token; t != nil {
		ctx := context.Background()
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: t.Token})
		client = oauth2.NewClient(ctx, ts)
	}

	return &GHRepo{
		log:    log,
		config: cfg,
		client: github.NewClient(client),
	}
}

// DownloadTemplate downloads template repository ->
func (r *GHRepo) DownloadTemplate(tmp, version string) (string, error) { //nolint:gocyclo
	r.log.Info("[GET ARCHIVE LINK]", zap.String("owner", rrOwner), zap.String("repository", rrRepo), zap.String("encoding", "zip"), zap.String("ref", version))
	url, resp, err := r.client.Repositories.GetArchiveLink(context.Background(), rrOwner, rrRepo, github.Zipball, &github.RepositoryContentGetOptions{Ref: version}, true)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusFound {
		return "", fmt.Errorf("wrong response status, got: %d", resp.StatusCode)
	}

	r.log.Info("[REQUESTING REPO]", zap.String("url", url.String()))
	request, err := r.client.NewRequest(http.MethodGet, url.String(), nil)
	if err != nil {
		return "", err
	}

	buf := new(bytes.Buffer)
	r.log.Info("[FETCHING CONTENT]", zap.String("url", url.String()))
	do, err := r.client.Do(context.Background(), request, buf)
	if err != nil {
		return "", err
	}

	_, _ = io.Copy(io.Discard, do.Body)
	_ = do.Body.Close()

	// replace '/' in the branch name or tag with the '_' to prevent using '/' as a path separator
	version = strings.ReplaceAll(version, "/", "_")

	name := path.Join(tmp, "roadrunner-server-"+version)
	_ = os.RemoveAll(name)

	r.log.Debug("[FLUSHING DATA ON THE DISK]", zap.String("path", name+zipExt))
	f, err := os.Create(name + zipExt)
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

	rc, err := zip.OpenReader(name + zipExt)
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

	abs, err := filepath.Abs(rc.File[0].Name)
	if err != nil {
		return "", err
	}
	// for this repository (roadrunner-server/roadrunner), 0-th element is a directory with content
	if strings.Contains(abs, "..") {
		return "", errors.New("path should not contain the '..' symbols")
	}

	outDir := rc.File[0].Name
	for _, zf := range rc.File {
		r.log.Debug("[EXTRACTING]", zap.String("file", zf.Name), zap.String("path", dest))
		err = extract(dest, zf)
		if err != nil {
			return "", err
		}
	}

	r.log.Info("[REPOSITORY SUCCESSFULLY SAVED]", zap.String("path", filepath.Join(dest, outDir))) //nolint:gosec
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

func (r *GHRepo) GetPluginsModData() ([]*velox.ModulesInfo, error) {
	poolExecutor := newPool(r.log, r.client)
	for k, v := range r.config.GitHub.Plugins {
		poolExecutor.add(&pcfg{
			pluginCfg: v,
			name:      k,
		})
	}

	poolExecutor.wait()

	if len(poolExecutor.errors()) != 0 {
		return nil, errors.Join(poolExecutor.errors()...)
	}

	mi := poolExecutor.moduleinfo()
	poolExecutor.stop()

	return mi, nil
}
