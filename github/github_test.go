package github

import (
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func discardLogger() *slog.Logger { return slog.New(slog.DiscardHandler) }

func TestArchiveURL(t *testing.T) {
	c := NewClient("https://github.com", "", discardLogger())

	cases := []struct {
		ref  string
		want string
	}{
		{"v2025.1.2", "https://github.com/roadrunner-server/roadrunner/archive/refs/tags/v2025.1.2.zip"},
		{"v3.0.0", "https://github.com/roadrunner-server/roadrunner/archive/refs/tags/v3.0.0.zip"},
		{"master", "https://github.com/roadrunner-server/roadrunner/archive/refs/heads/master.zip"},
		{"feature/x", "https://github.com/roadrunner-server/roadrunner/archive/refs/heads/feature/x.zip"},
		{"version-fix", "https://github.com/roadrunner-server/roadrunner/archive/refs/heads/version-fix.zip"},
		{"569ffe0d833580af456150546eec35c44b7ca1fa", "https://github.com/roadrunner-server/roadrunner/archive/569ffe0d833580af456150546eec35c44b7ca1fa.zip"},
		{"v1.2.3.4", "https://github.com/roadrunner-server/roadrunner/archive/refs/heads/v1.2.3.4.zip"},
		{"v2-wip", "https://github.com/roadrunner-server/roadrunner/archive/refs/heads/v2-wip.zip"},
		{"v2025", "https://github.com/roadrunner-server/roadrunner/archive/refs/tags/v2025.zip"},
		{"569FFE0D833580AF456150546EEC35C44B7CA1FA", "https://github.com/roadrunner-server/roadrunner/archive/569FFE0D833580AF456150546EEC35C44B7CA1FA.zip"},
	}
	for _, tc := range cases {
		t.Run(tc.ref, func(t *testing.T) {
			u, err := c.archiveURL(tc.ref)
			require.NoError(t, err)
			require.Equal(t, tc.want, u.String())
		})
	}
}

func TestIsCommitSHA(t *testing.T) {
	cases := map[string]bool{
		"569ffe0d833580af456150546eec35c44b7ca1fa":  true,
		"569FFE0D833580AF456150546EEC35C44B7CA1FA":  true,
		"569Ffe0d833580af456150546eec35c44b7ca1fA":  true,
		"569ffe0d833580af456150546eec35c44b7ca1f":   false,
		"569ffe0d833580af456150546eec35c44b7ca1fab": false,
		"569ffe0d833580af456150546eec35c44b7ca1fz":  false,
		"master": false,
		"":       false,
	}
	for input, want := range cases {
		t.Run(input, func(t *testing.T) {
			require.Equal(t, want, isCommitSHA(input))
		})
	}
}

func TestArchiveURL_CustomBaseURL(t *testing.T) {
	// GitHub Enterprise hostname.
	c := NewClient("https://ghe.example.com/", "tok", discardLogger())
	u, err := c.archiveURL("v3.0.0")
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(u.String(), "https://ghe.example.com/"), "got %s", u.String())
	require.NotContains(t, u.String(), "//roadrunner-server", "trailing slash in baseURL must be trimmed")
}

func TestNewClient_DefaultBaseURL(t *testing.T) {
	c := NewClient("", "", discardLogger())
	u, err := c.archiveURL("master")
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(u.String(), "https://github.com/"))
}
