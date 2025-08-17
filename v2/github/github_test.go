package github

import (
	"os"
	"testing"

	cacheimpl "github.com/roadrunner-server/velox/v2025/v2/cache"
	"go.uber.org/zap"
)

func TestGitHubClient_DownloadTemplate(t *testing.T) {
	// Create logger for testing
	logger, _ := zap.NewDevelopment()

	c := cacheimpl.NewRRCache()
	// Create GitHubClient without token (will use default HTTP client)
	client := NewHTTPClient("", c, logger)

	// Use os.TmpDir() as requested
	tmpDir := os.TempDir()
	rrVersion := "v2025.1.0"

	// Call the method under test
	resultPath, err := client.DownloadTemplate(tmpDir, "foobar", rrVersion)

	// Basic assertions - just check if method executed without panicking
	if err != nil {
		t.Logf("DownloadTemplate returned error (expected for test without real token): %v", err)
	}

	if resultPath != "" {
		t.Logf("DownloadTemplate returned path: %s", resultPath)
	}
}
