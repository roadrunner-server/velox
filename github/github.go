package github

import (
	"archive/zip"
	"bytes"
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
	"golang.org/x/oauth2"
)

const (
	rrOwner = "roadrunner-server"
	rrRepo  = "roadrunner"
	zipExt  = ".zip"

	commitSHALen = 40

	httpTimeout = time.Minute
)

// Cache stores downloaded RoadRunner archives keyed by ref.
type Cache interface {
	Get(key string) ([]byte, bool)
	Add(key string, value []byte)
}

// Client fetches the upstream RR source tree.
type Client struct {
	http    *http.Client
	log     *slog.Logger
	cache   Cache
	baseURL string
}

// NewClient builds a client for baseURL, defaulting to github.com and adding OAuth2 when accessToken is set.
func NewClient(baseURL, accessToken string, cache Cache, log *slog.Logger) *Client {
	// fetch reads the Location header itself, so the client must stop at the 3xx response.
	noFollow := func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	httpc := &http.Client{Timeout: httpTimeout, CheckRedirect: noFollow}

	if accessToken != "" {
		// oauth2.NewClient returns a new client that inherits the Transport alone, so re-apply CheckRedirect and Timeout.
		ctx := context.WithValue(context.Background(), oauth2.HTTPClient, httpc)
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: accessToken})
		httpc = oauth2.NewClient(ctx, ts)
		httpc.CheckRedirect = noFollow
		httpc.Timeout = httpTimeout
	}

	if baseURL == "" {
		baseURL = "https://github.com"
	}
	return &Client{
		http:    httpc,
		log:     log,
		cache:   cache,
		baseURL: strings.TrimRight(baseURL, "/"),
	}
}

// DownloadTemplate fetches the archive for rrRef, unpacks it into downloadDir, and returns the source tree path.
func (c *Client) DownloadTemplate(ctx context.Context, downloadDir, rrRef string) (string, error) {
	if cached, ok := c.cache.Get(rrRef); ok {
		c.log.Info("RR archive cache hit", "ref", rrRef, "bytes", len(cached))
		return c.saveRR(cached, rrRef, downloadDir)
	}

	archiveURL, err := c.archiveURL(rrRef)
	if err != nil {
		return "", err
	}
	c.log.Info("downloading RR archive", "ref", rrRef, "url", archiveURL.String())

	zipBytes, err := c.fetch(ctx, archiveURL)
	if err != nil {
		return "", err
	}
	c.cache.Add(rrRef, zipBytes)
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

// fetch gets archiveURL, follows the single redirect to the CDN, and returns the body bytes.
func (c *Client) fetch(ctx context.Context, archiveURL *url.URL) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, archiveURL.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", archiveURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	// GitHub.com answers with 302; accept any 3xx for GitHub Enterprise and proxies that send 301, 307, or 308.
	if resp.StatusCode < 300 || resp.StatusCode >= 400 {
		return nil, fmt.Errorf("expected 3xx redirect from %s, got %d", archiveURL, resp.StatusCode)
	}
	loc, err := resp.Location()
	if err != nil {
		return nil, fmt.Errorf("read redirect Location: %w", err)
	}
	if loc == nil {
		return nil, errors.New("redirect response had no Location header")
	}

	// Follow the redirect with a context-aware request so cancellation works.
	req2, err := http.NewRequestWithContext(ctx, http.MethodGet, loc.String(), nil)
	if err != nil {
		return nil, err
	}
	resp2, err := c.http.Do(req2)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", loc, err)
	}
	defer func() { _ = resp2.Body.Close() }()
	if resp2.StatusCode >= 300 {
		return nil, fmt.Errorf("download %s returned %d", loc, resp2.StatusCode)
	}
	buf := &bytes.Buffer{}
	if _, err := io.Copy(buf, resp2.Body); err != nil {
		return nil, fmt.Errorf("read archive body: %w", err)
	}
	return buf.Bytes(), nil
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
func extract(dest string, zf *zip.File) error {
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
	defer func() { _ = destFile.Close() }()

	zr, err := zf.Open()
	if err != nil {
		return err
	}
	defer func() { _ = zr.Close() }()

	if _, err := io.Copy(destFile, zr); err != nil { //nolint:gosec // G110: the archive comes from github.com or the configured GHE host
		return err
	}
	return nil
}
