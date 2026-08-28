package provider

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func TestFS_Match(t *testing.T) {
	fs := &FS{}
	assert.True(t, fs.Match("/abs/path"))
	assert.True(t, fs.Match("./rel"))
	assert.True(t, fs.Match("../rel"))
	assert.True(t, fs.Match("file:///abs"))
	assert.False(t, fs.Match("git@github.com:org/repo.git"))
	assert.False(t, fs.Match("https://github.com/org/repo.git"))
}

func TestFS_ParseAndFetch(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "api.yaml"), "openapi: 3.0.0\n")

	src, err := (&FS{}).Parse(dir, "")
	require.NoError(t, err)

	content, err := src.Fetch("api.yaml")
	require.NoError(t, err)
	assert.Equal(t, "openapi: 3.0.0\n", string(content))

	assert.Empty(t, src.Commit())

	_, err = src.Fetch("missing.yaml")
	assert.Error(t, err)
}

func TestFS_ParseStripsFileScheme(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "api.yaml"), "x")

	src, err := (&FS{}).Parse("file://"+dir, "")
	require.NoError(t, err)
	content, err := src.Fetch("api.yaml")
	require.NoError(t, err)
	assert.Equal(t, "x", string(content))
}

// The glob results must be base-relative so they can be fed back into Fetch,
// even when base is an absolute path (as t.TempDir returns).
func TestFS_GlobRoundTripsWithFetch(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "openapi"), 0o755))
	write(t, filepath.Join(dir, "openapi", "a.yaml"), "A")
	write(t, filepath.Join(dir, "openapi", "b.yaml"), "B")
	write(t, filepath.Join(dir, "openapi", "note.txt"), "nope")

	src, err := (&FS{}).Parse(dir, "")
	require.NoError(t, err)

	matches, err := src.Glob("openapi/*.yaml")
	require.NoError(t, err)
	sort.Strings(matches)
	assert.Equal(t, []string{
		filepath.Join("openapi", "a.yaml"),
		filepath.Join("openapi", "b.yaml"),
	}, matches)

	// Each match must be fetchable without a double base prefix.
	for _, m := range matches {
		_, err := src.Fetch(m)
		require.NoError(t, err, "match %q should be fetchable", m)
	}
}

func TestFS_GlobNoMatch(t *testing.T) {
	src, err := (&FS{}).Parse(t.TempDir(), "")
	require.NoError(t, err)
	matches, err := src.Glob("*.yaml")
	require.NoError(t, err)
	assert.Empty(t, matches)
}
