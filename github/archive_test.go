package github

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// newZip builds an in-memory archive with the entries in the given order.
func newZip(t *testing.T, entries [][2]string) []byte {
	t.Helper()

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, e := range entries {
		f, err := w.Create(e[0])
		require.NoError(t, err)
		_, err = f.Write([]byte(e[1]))
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
	return buf.Bytes()
}

// zipFiles lists the entries of data; an insecure entry name is part of what the tests exercise.
func zipFiles(t *testing.T, data []byte) []*zip.File {
	t.Helper()

	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil && !errors.Is(err, zip.ErrInsecurePath) {
		require.NoError(t, err)
	}
	return r.File
}

func TestArchiveRoot(t *testing.T) {
	root, err := archiveRoot(zipFiles(t, newZip(t, [][2]string{
		{"roadrunner-master/", ""},
		{"roadrunner-master/go.mod", "module x"},
	})))
	require.NoError(t, err)
	require.Equal(t, "roadrunner-master", root)

	_, err = archiveRoot(zipFiles(t, newZip(t, [][2]string{{"a/x", ""}, {"b/y", ""}})))
	require.Error(t, err)

	_, err = archiveRoot(nil)
	require.Error(t, err)
}

func TestSaveRR_ExtractsArchive(t *testing.T) {
	// The file entry precedes its directory entry, so extraction must create the parents itself.
	data := newZip(t, [][2]string{
		{"roadrunner-master/cmd/rr/main.go", "package main"},
		{"roadrunner-master/", ""},
		{"roadrunner-master/go.mod", "module github.com/roadrunner-server/roadrunner/v2025"},
	})
	c := NewClient("", "", discardLogger())
	dl := t.TempDir()

	root, err := c.saveRR(data, "feature/x", dl)
	require.NoError(t, err)
	want, err := filepath.Abs(filepath.Join(dl, "roadrunner-server-feature_x", "roadrunner-master"))
	require.NoError(t, err)
	require.Equal(t, want, root)

	got, err := os.ReadFile(filepath.Join(root, "cmd", "rr", "main.go"))
	require.NoError(t, err)
	require.Equal(t, "package main", string(got))
}

func TestExtract_RejectsTraversal(t *testing.T) {
	files := zipFiles(t, newZip(t, [][2]string{{"../evil", "x"}}))
	require.Len(t, files, 1)

	err := extract(t.TempDir(), files[0])
	require.Error(t, err)
	require.Contains(t, err.Error(), "CWE-22")
}
