package github

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

const (
	rrOwner string = "roadrunner-server"
	rrRepo  string = "roadrunner"
	zipExt  string = ".zip"
)

type cache interface {
	Get(key string) *bytes.Buffer
	Set(key string, value *bytes.Buffer)
}

/*
GitHubClient represents template repository
*/
type GitHubClient struct {
	internalClient *http.Client
	log            *zap.Logger
	// version -> in-memory zipped RR
	cache cache
}

func NewHTTPClient(accessToken string, cache cache, log *zap.Logger) *GitHubClient {
	client := &http.Client{
		Timeout: time.Minute,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// if a token exists, use it to increase rate limiter
	if accessToken != "" {
		ctx := context.Background()
		ctx = context.WithValue(ctx, oauth2.HTTPClient, client)
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: accessToken})
		return &GitHubClient{
			internalClient: oauth2.NewClient(ctx, ts),
			log:            log,
			cache:          cache,
		}
	}

	return &GitHubClient{
		internalClient: client,
		log:            log,
		cache:          cache,
	}
}

// DownloadTemplate downloads template repository (roadrunner), unpacks it and returns a result path
func (r *GitHubClient) DownloadTemplate(downloadDir, hash, rrVersion string) (string, error) { //nolint:gocyclo
	if rrbuf := r.cache.Get(rrVersion); rrbuf != nil {
		// here we know that we have a cached buffer
		// just save it to the new location (downloadDir + hash)
		// rrbuf is a copy of the buffer, so we can freely clear it
		defer rrbuf.Reset()
		return r.saveRR(rrbuf, rrVersion, filepath.Join(downloadDir, hash))
	}

	r.log.Info("obtaining link", zap.String("owner", rrOwner), zap.String("repository", rrRepo), zap.String("encoding", "zip"), zap.String("ref", rrVersion))
	// We can have 3 possible options here:
	// 1. Tag -> link to use: https://github.com/roadrunner-server/roadrunner/archive/refs/tags/v2025.1.2.zip
	// 2. BranchName -> link to use: https://github.com/roadrunner-server/roadrunner/archive/refs/heads/master.zip
	// 3. CommitSHA -> link to use: https://github.com/roadrunner-server/roadrunner/archive/569ffe0d833580af456150546eec35c44b7ca1fa.zip
	rrurl, err := r.parseRRref(rrVersion)
	if err != nil {
		return "", err
	}

	buf := new(bytes.Buffer)
	err = r.downloadRR(buf, rrurl)
	if err != nil {
		return "", err
	}

	// save zipped rr buffer
	r.cache.Set(rrVersion, buf)

	return r.saveRR(buf, rrVersion, filepath.Join(downloadDir, hash))
}

// downloadRR method used to save a raw zipped roadrunner repository into the provided buffer pointer
func (r *GitHubClient) downloadRR(buf *bytes.Buffer, rrurl *url.URL) error {
	r.log.Info("sending download request", zap.String("url", rrurl.String()))
	req, err := http.NewRequest(http.MethodGet, rrurl.String(), nil)
	if err != nil {
		return err
	}

	// first request with the original link should return a redirect
	resp, err := r.internalClient.Do(req)
	if err != nil {
		return err
	}

	// check the redirect
	if resp.StatusCode != http.StatusFound {
		return fmt.Errorf("wrong response status, got: %d", resp.StatusCode)
	}

	// we need to follow the redirect, it should be only 1 redirect, so we don't use a recursive approach
	locurl, err := resp.Location()
	if err != nil {
		return fmt.Errorf("failed to get location from response: %w", err)
	}

	if locurl == nil {
		return errors.New("failed to get location from response: no location header found")
	}

	// perform the final request to the redirected URL
	resp, err = r.internalClient.Get(locurl.String())
	if err != nil {
		return fmt.Errorf("failed to download repository: %w", err)
	}

	_, err = io.Copy(buf, resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return fmt.Errorf("failed to copy response body: %w", err)
	}

	return nil
}

func (r *GitHubClient) saveRR(buf *bytes.Buffer, rrVersion, downloadDir string) (string, error) {
	// replace '/' in the branch name or tag with the '_' to prevent using '/' as a path separator
	rrVersion = strings.ReplaceAll(rrVersion, "/", "_")
	// rrSaveDest is a path to the directory where the repository will be saved
	rrSaveDest := filepath.Join(downloadDir, "roadrunner-server-"+rrVersion)

	_, err := os.Stat(rrSaveDest)
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("stat failed for output directory %s: %w", rrSaveDest, err)
	}
	if os.IsExist(err) {
		_ = os.RemoveAll(rrSaveDest)
	}
	err = os.MkdirAll(rrSaveDest, os.ModeDir|os.ModePerm)
	if err != nil {
		return "", fmt.Errorf("failed to create directory %s: %w", rrSaveDest, err)
	}

	r.log.Debug("saving repository in temporary folder", zap.String("path", rrSaveDest+zipExt))
	f, err := os.Create(rrSaveDest + zipExt)
	if err != nil {
		return "", fmt.Errorf("failed to create zip file %s: %w", rrSaveDest+zipExt, err)
	}

	defer func() {
		_ = f.Close()
	}()

	n, err := f.Write(buf.Bytes())
	if err != nil {
		return "", err
	}

	r.log.Debug("repository saved", zap.Int("bytes written", n))

	rc, err := zip.OpenReader(rrSaveDest + zipExt)
	if err != nil {
		return "", err
	}

	defer func() {
		_ = rc.Close()
	}()

	// absolute filename
	dest, err := filepath.Abs(rrSaveDest)
	if err != nil {
		return "", err
	}

	err = os.RemoveAll(dest)
	if err != nil {
		return "", err
	}

	err = os.Mkdir(rrSaveDest, os.ModePerm)
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
	if strings.Contains(outDir, "..") {
		return "", errors.New("CWE-22, output dir from a zip file can't contain a '..' filesystem operation, more info: https://cwe.mitre.org/data/definitions/22.html")
	}

	for _, zf := range rc.File {
		r.log.Debug("extracting repository", zap.String("file", zf.Name), zap.String("path", dest))
		err = extract(dest, zf)
		if err != nil {
			return "", err
		}
	}

	r.log.Info("repository saved", zap.String("path", filepath.Join(dest, outDir))) //nolint:gosec
	// first name is the output path
	return filepath.Join(dest, outDir), nil //nolint:gosec
}

func (r *GitHubClient) parseRRref(rrVersion string) (*url.URL, error) {
	// 1. Tag -> link to use: https://github.com/roadrunner-server/roadrunner/archive/refs/tags/v2025.1.2.zip
	// 2. BranchName -> link to use: https://github.com/roadrunner-server/roadrunner/archive/refs/heads/master.zip
	// 3. CommitSHA -> link to use: https://github.com/roadrunner-server/roadrunner/archive/569ffe0d833580af456150546eec35c44b7ca1fa.zip

	// if we have a v prefix -> use tag
	if strings.HasPrefix(rrVersion, "v") {
		return url.Parse(fmt.Sprintf("https://github.com/roadrunner-server/roadrunner/archive/refs/tags/%s.zip", rrVersion))
	}

	// assume that we have a sha here
	if len(rrVersion) == 40 {
		return url.Parse(fmt.Sprintf("https://github.com/roadrunner-server/roadrunner/archive/%s.zip", rrVersion))
	}

	// for all other cases, assume that we have a branch name
	return url.Parse(fmt.Sprintf("https://github.com/roadrunner-server/roadrunner/archive/refs/heads/%s.zip", rrVersion))
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
