package github

import (
	"archive/zip"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

const (
	rrOwner = "roadrunner-server"
	rrRepo  = "roadrunner"
	zipExt  = ".zip"

	commitSHALen = 40

	httpTimeout = time.Minute
)

// Client fetches the upstream RR source tree.
type Client struct {
	http    *http.Client
	log     *slog.Logger
	baseURL string
	token   string
}

// NewClient builds a client for baseURL, defaulting to github.com. accessToken, when set, goes into the Authorization header of the archive request, and net/http drops that header on a redirect to another host.
func NewClient(baseURL, accessToken string, log *slog.Logger) *Client {
	if baseURL == "" {
		baseURL = "https://github.com"
	}
	return &Client{
		http:    &http.Client{Timeout: httpTimeout},
		log:     log,
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   accessToken,
	}
}

// DownloadTemplate fetches the archive for rrRef, unpacks it into downloadDir, and returns the source tree path.
func (c *Client) DownloadTemplate(ctx context.Context, downloadDir, rrRef string) (string, error) {
	archiveURL, err := c.archiveURL(rrRef)
	if err != nil {
		return "", err
	}
	c.log.Info("downloading RR archive", "ref", rrRef, "url", archiveURL.String())

	zipBytes, err := c.fetch(ctx, archiveURL)
	if err != nil {
		return "", err
	}
	return c.saveRR(zipBytes, rrRef, downloadDir)
}

// isCommitSHA reports whether ref is a 40-character hexadecimal commit SHA.
func isCommitSHA(ref string) bool {
	if len(ref) != commitSHALen {
		return false
	}
	_, err := hex.DecodeString(ref)
	return err == nil
}

// archiveURL routes a tag to refs/tags, a commit SHA to /archive/<sha>.zip, and anything else to refs/heads.
func (c *Client) archiveURL(rrRef string) (*url.URL, error) {
	var raw string
	switch {
	case semver.IsValid(rrRef):
		raw = fmt.Sprintf("%s/%s/%s/archive/refs/tags/%s%s", c.baseURL, rrOwner, rrRepo, rrRef, zipExt)
	case isCommitSHA(rrRef):
		raw = fmt.Sprintf("%s/%s/%s/archive/%s%s", c.baseURL, rrOwner, rrRepo, rrRef, zipExt)
	default:
		raw = fmt.Sprintf("%s/%s/%s/archive/refs/heads/%s%s", c.baseURL, rrOwner, rrRepo, rrRef, zipExt)
	}
	return url.Parse(raw)
}

// fetch gets archiveURL and returns the body bytes; net/http follows the redirect to the archive host with the request context intact.
func (c *Client) fetch(ctx context.Context, archiveURL *url.URL) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, archiveURL.String(), nil)
	if err != nil {
		return nil, err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", archiveURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s returned %d", archiveURL, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read archive body: %w", err)
	}
	return body, nil
}

// saveRR writes the archive bytes to disk, extracts them, and returns the absolute root directory.
func (c *Client) saveRR(zipBytes []byte, rrRef, downloadDir string) (string, error) {
	// A branch name can contain "/", which would create extra nested directories on disk.
	safeRef := strings.ReplaceAll(rrRef, "/", "_")
	rrSaveDest := filepath.Join(downloadDir, "roadrunner-server-"+safeRef)
	_ = os.RemoveAll(rrSaveDest)
	if err := os.MkdirAll(rrSaveDest, 0o755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", rrSaveDest, err)
	}

	zipPath := rrSaveDest + zipExt
	c.log.Debug("writing archive to disk", "path", zipPath, "bytes", len(zipBytes))
	if err := os.WriteFile(zipPath, zipBytes, 0o600); err != nil {
		return "", fmt.Errorf("write archive %s: %w", zipPath, err)
	}

	rc, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", fmt.Errorf("open zip %s: %w", zipPath, err)
	}
	defer func() { _ = rc.Close() }()

	if len(rc.File) == 0 {
		return "", errors.New("empty zip archive")
	}

	dest, err := filepath.Abs(rrSaveDest)
	if err != nil {
		return "", err
	}
	outDir, err := archiveRoot(rc.File)
	if err != nil {
		return "", err
	}

	for _, zf := range rc.File {
		if err := extract(dest, zf); err != nil {
			return "", err
		}
	}
	rootPath := filepath.Join(dest, outDir)
	c.log.Info("RR archive extracted", "path", rootPath)
	return rootPath, nil
}

// archiveRoot returns the single top-level directory shared by every zip entry.
func archiveRoot(files []*zip.File) (string, error) {
	var root string
	for _, zf := range files {
		// A zip entry name uses "/" as the separator on every platform.
		first, _, _ := strings.Cut(zf.Name, "/")
		if first == "" {
			return "", fmt.Errorf("zip entry %q has no top-level directory", zf.Name)
		}
		if root == "" {
			root = first
			continue
		}
		if first != root {
			return "", fmt.Errorf("zip has several top-level directories: %q and %q", root, first)
		}
	}
	if root == "" {
		return "", errors.New("zip has no entries")
	}
	return root, nil
}

// extract writes a single zip entry to dest and refuses any entry whose resolved path escapes dest (CWE-22).
func extract(dest string, zf *zip.File) (err error) {
	pt := filepath.Join(dest, zf.Name) //nolint:gosec // G305: the prefix check below rejects paths that escape dest
	cleanDest := filepath.Clean(dest) + string(os.PathSeparator)
	if !strings.HasPrefix(filepath.Clean(pt)+string(os.PathSeparator), cleanDest) {
		return fmt.Errorf("CWE-22: zip entry %q escapes %q", zf.Name, dest)
	}

	if zf.FileInfo().IsDir() {
		return os.MkdirAll(pt, 0o755)
	}

	// Archives normally list a directory before its files, but the order is not guaranteed.
	if err := os.MkdirAll(filepath.Dir(pt), 0o755); err != nil {
		return err
	}

	destFile, err := os.OpenFile(pt, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, zf.Mode())
	if err != nil {
		return err
	}
	// A write that fails at close leaves a truncated file, so the close error counts once the copy succeeded.
	defer func() {
		if cerr := destFile.Close(); err == nil {
			err = cerr
		}
	}()

	zr, err := zf.Open()
	if err != nil {
		return err
	}
	defer func() { _ = zr.Close() }()

	_, err = io.Copy(destFile, zr) //nolint:gosec // G110: the archive comes from github.com or the configured GHE host
	return err
}
