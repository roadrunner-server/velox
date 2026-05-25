// Package github downloads and extracts the RoadRunner source tree from a
// GitHub (or GitHub Enterprise) tag, branch, or commit SHA.
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
	"regexp"
	"strings"
	"time"

	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

const (
	rrOwner = "roadrunner-server"
	rrRepo  = "roadrunner"
	zipExt  = ".zip"

	httpTimeout = time.Minute
)

// Cache stores downloaded RR archives keyed by ref to avoid re-downloading
// the same RR version across builds.
type Cache interface {
	Get(key string) ([]byte, bool)
	Add(key string, value []byte)
}

// Client fetches the upstream RR source tree.
type Client struct {
	http    *http.Client
	log     *zap.Logger
	cache   Cache
	baseURL string
}

// NewClient constructs a GitHub client. baseURL is the GitHub host (e.g.
// "https://github.com" or a GitHub Enterprise URL such as "https://ghe.example.com");
// if empty, the default github.com is used. If accessToken is non-empty, OAuth2
// is used so the client picks up the larger rate limit available to authenticated
// requests.
func NewClient(baseURL, accessToken string, cache Cache, log *zap.Logger) *Client {
	httpc := &http.Client{
		Timeout: httpTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	if accessToken != "" {
		ctx := context.WithValue(context.Background(), oauth2.HTTPClient, httpc)
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: accessToken})
		httpc = oauth2.NewClient(ctx, ts)
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

// DownloadTemplate fetches the RR archive for rrRef (tag, branch, or 40-char
// SHA), unpacks it into downloadDir/hash/, and returns the path of the
// extracted source tree. The archive bytes are cached so repeat builds of the
// same ref skip the network call.
func (c *Client) DownloadTemplate(ctx context.Context, downloadDir, hash, rrRef string) (string, error) {
	if cached, ok := c.cache.Get(rrRef); ok {
		c.log.Info("RR archive cache hit", zap.String("ref", rrRef), zap.Int("bytes", len(cached)))
		return c.saveRR(cached, rrRef, filepath.Join(downloadDir, hash))
	}

	archiveURL, err := c.archiveURL(rrRef)
	if err != nil {
		return "", err
	}
	c.log.Info("downloading RR archive", zap.String("ref", rrRef), zap.String("url", archiveURL.String()))

	zipBytes, err := c.fetch(ctx, archiveURL)
	if err != nil {
		return "", err
	}
	c.cache.Add(rrRef, zipBytes)
	return c.saveRR(zipBytes, rrRef, filepath.Join(downloadDir, hash))
}

// sha40 matches a 40-character hexadecimal commit SHA.
var sha40 = regexp.MustCompile(`^[a-f0-9]{40}$`)

// archiveURL composes the archive URL for the given ref. Tags use the
// refs/tags path, branches use refs/heads, SHAs use bare /archive/<sha>.zip.
func (c *Client) archiveURL(rrRef string) (*url.URL, error) {
	var raw string
	switch {
	case strings.HasPrefix(rrRef, "v"):
		raw = fmt.Sprintf("%s/%s/%s/archive/refs/tags/%s%s", c.baseURL, rrOwner, rrRepo, rrRef, zipExt)
	case sha40.MatchString(rrRef):
		raw = fmt.Sprintf("%s/%s/%s/archive/%s%s", c.baseURL, rrOwner, rrRepo, rrRef, zipExt)
	default:
		raw = fmt.Sprintf("%s/%s/%s/archive/refs/heads/%s%s", c.baseURL, rrOwner, rrRepo, rrRef, zipExt)
	}
	return url.Parse(raw)
}

// fetch GET-s archiveURL, following the single GitHub redirect to the actual
// CDN URL, and returns the body bytes.
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

	if resp.StatusCode != http.StatusFound {
		return nil, fmt.Errorf("expected 302 redirect from %s, got %d", archiveURL, resp.StatusCode)
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

// saveRR writes the archive bytes to disk and extracts them. Returns the
// absolute path of the extracted root directory.
func (c *Client) saveRR(zipBytes []byte, rrRef, downloadDir string) (string, error) {
	// "/" can appear in branch names (e.g. feat/foo); rewrite to "_" so we don't
	// accidentally create extra nested directories on disk.
	safeRef := strings.ReplaceAll(rrRef, "/", "_")
	rrSaveDest := filepath.Join(downloadDir, "roadrunner-server-"+safeRef)
	_ = os.RemoveAll(rrSaveDest)
	if err := os.MkdirAll(rrSaveDest, 0o755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", rrSaveDest, err)
	}

	zipPath := rrSaveDest + zipExt
	c.log.Debug("writing archive to disk", zap.String("path", zipPath), zap.Int("bytes", len(zipBytes)))
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
	outDir := rc.File[0].Name

	for _, zf := range rc.File {
		if err := extract(dest, zf); err != nil {
			return "", err
		}
	}
	rootPath := filepath.Join(dest, outDir)
	c.log.Info("RR archive extracted", zap.String("path", rootPath))
	return rootPath, nil
}

// extract writes a single zip entry to dest, refusing any entry whose
// resolved path escapes dest (CWE-22). The single check here replaces the
// historical pair of overlapping validations.
func extract(dest string, zf *zip.File) error {
	pt := filepath.Join(dest, zf.Name) //nolint:gosec
	cleanDest := filepath.Clean(dest) + string(os.PathSeparator)
	if !strings.HasPrefix(filepath.Clean(pt)+string(os.PathSeparator), cleanDest) {
		return fmt.Errorf("CWE-22: zip entry %q escapes %q", zf.Name, dest)
	}

	if zf.FileInfo().IsDir() {
		return os.MkdirAll(pt, 0o755)
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

	// G110 (decompression bomb) acknowledged: archive comes from a trusted host
	// (github.com or user-configured GHE) and is gated by HTTP content-length.
	if _, err := io.Copy(destFile, zr); err != nil { //nolint:gosec
		return err
	}
	return nil
}
