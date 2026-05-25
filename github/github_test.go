package github

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestArchiveURL(t *testing.T) {
	c := NewClient("https://github.com", "", NewLRUCache(0), zap.NewNop())

	cases := []struct {
		ref  string
		want string
	}{
		{"v2025.1.2", "https://github.com/roadrunner-server/roadrunner/archive/refs/tags/v2025.1.2.zip"},
		{"v3.0.0", "https://github.com/roadrunner-server/roadrunner/archive/refs/tags/v3.0.0.zip"},
		{"master", "https://github.com/roadrunner-server/roadrunner/archive/refs/heads/master.zip"},
		{"feature/x", "https://github.com/roadrunner-server/roadrunner/archive/refs/heads/feature/x.zip"},
		{"569ffe0d833580af456150546eec35c44b7ca1fa", "https://github.com/roadrunner-server/roadrunner/archive/569ffe0d833580af456150546eec35c44b7ca1fa.zip"},
	}
	for _, tc := range cases {
		t.Run(tc.ref, func(t *testing.T) {
			u, err := c.archiveURL(tc.ref)
			require.NoError(t, err)
			require.Equal(t, tc.want, u.String())
		})
	}
}

func TestArchiveURL_CustomBaseURL(t *testing.T) {
	// GitHub Enterprise hostname.
	c := NewClient("https://ghe.example.com/", "tok", NewLRUCache(0), zap.NewNop())
	u, err := c.archiveURL("v3.0.0")
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(u.String(), "https://ghe.example.com/"), "got %s", u.String())
	require.NotContains(t, u.String(), "//roadrunner-server", "trailing slash in baseURL must be trimmed")
}

func TestNewClient_DefaultBaseURL(t *testing.T) {
	c := NewClient("", "", NewLRUCache(0), zap.NewNop())
	u, err := c.archiveURL("master")
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(u.String(), "https://github.com/"))
}

func TestLRUCache_GetSet(t *testing.T) {
	c := NewLRUCache(2)
	_, ok := c.Get("missing")
	require.False(t, ok)

	c.Add("a", []byte("payload-a"))
	got, ok := c.Get("a")
	require.True(t, ok)
	require.Equal(t, []byte("payload-a"), got)
}
