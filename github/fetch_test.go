package github

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFetch_FollowsRedirectWithToken(t *testing.T) {
	auth := make(chan string, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /roadrunner-server/roadrunner/archive/refs/heads/master.zip", func(w http.ResponseWriter, r *http.Request) {
		auth <- r.Header.Get("Authorization")
		http.Redirect(w, r, "/blob", http.StatusFound)
	})
	mux.HandleFunc("GET /blob", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("zip-bytes"))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c := NewClient(srv.URL, "tok", discardLogger())
	u, err := c.archiveURL("master")
	require.NoError(t, err)

	got, err := c.fetch(t.Context(), u)
	require.NoError(t, err)
	require.Equal(t, "zip-bytes", string(got))
	require.Equal(t, "Bearer tok", <-auth)
}

func TestFetch_TokenStaysOnBaseHost(t *testing.T) {
	blobAuth := make(chan string, 1)
	blob := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		blobAuth <- r.Header.Get("Authorization")
		_, _ = w.Write([]byte("zip-bytes"))
	}))
	t.Cleanup(blob.Close)
	originAuth := make(chan string, 1)
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		originAuth <- r.Header.Get("Authorization")
		http.Redirect(w, r, blob.URL+"/blob", http.StatusFound)
	}))
	t.Cleanup(origin.Close)

	// The base URL names the origin as localhost, so the hop to 127.0.0.1 leaves the configured host.
	c := NewClient(strings.Replace(origin.URL, "127.0.0.1", "localhost", 1), "tok", discardLogger())
	u, err := c.archiveURL("master")
	require.NoError(t, err)

	got, err := c.fetch(t.Context(), u)
	require.NoError(t, err)
	require.Equal(t, "zip-bytes", string(got))
	require.Equal(t, "Bearer tok", <-originAuth)
	require.Empty(t, <-blobAuth)
}

func TestFetch_DirectBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("zip-bytes"))
	}))
	t.Cleanup(srv.Close)

	c := NewClient(srv.URL, "", discardLogger())
	u, err := c.archiveURL("v2025.1.0")
	require.NoError(t, err)

	got, err := c.fetch(t.Context(), u)
	require.NoError(t, err)
	require.Equal(t, "zip-bytes", string(got))
}

func TestFetch_ReportsStatus(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(srv.Close)

	c := NewClient(srv.URL, "", discardLogger())
	u, err := c.archiveURL("master")
	require.NoError(t, err)

	_, err = c.fetch(t.Context(), u)
	require.ErrorContains(t, err, "404")
}
