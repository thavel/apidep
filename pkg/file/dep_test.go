package file

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOutput(t *testing.T) {
	tests := []struct {
		name                          string
		filePath, root, dep, ref, def string
		want                          string
	}{
		{
			name:     "ref directory suffix keeps base name",
			filePath: "openapi/api.yaml",
			ref:      "out/",
			want:     "out/api.yaml",
		},
		{
			name:     "ref dot suffix preserves full path",
			filePath: "openapi/api.yaml",
			ref:      "out/.",
			want:     "out/openapi/api.yaml",
		},
		{
			name:     "ref explicit file",
			filePath: "openapi/api.yaml",
			ref:      "out/renamed.yaml",
			want:     "out/renamed.yaml",
		},
		{
			name:     "dep dot suffix preserves full path",
			filePath: "openapi/api.yaml",
			dep:      "out/.",
			want:     "out/openapi/api.yaml",
		},
		{
			name:     "dep directory keeps base name",
			filePath: "openapi/api.yaml",
			dep:      "out/deps.yml",
			want:     "out/api.yaml",
		},
		{
			name:     "root fallback keeps base name",
			filePath: "openapi/api.yaml",
			root:     "root/out.yml",
			want:     "root/api.yaml",
		},
		{
			name:     "default fallback keeps base name",
			filePath: "openapi/api.yaml",
			def:      "apidep/",
			want:     "apidep/api.yaml",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Output(tt.filePath, tt.root, tt.dep, tt.ref, tt.def)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestReadDep(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "api.dep.yml")
	require.NoError(t, os.WriteFile(path, []byte(
		"version: 1\noutput: ./apidep/\ndeps:\n"+
			"  - source: ./remote\n    ref: api.ref.yml\n"), 0o644))

	dep, err := ReadDep(path)
	require.NoError(t, err)
	assert.Equal(t, 1, dep.Version)
	assert.Equal(t, "./apidep/", dep.Output)
	require.Len(t, dep.Deps, 1)
	assert.Equal(t, "./remote", dep.Deps[0].Source)
	assert.Equal(t, "api.ref.yml", dep.Deps[0].Ref)
}

func TestReadDep_Missing(t *testing.T) {
	_, err := ReadDep(filepath.Join(t.TempDir(), "nope.yml"))
	require.Error(t, err)
}

func TestWriteDep_Status(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "sub", "api.yaml")
	var mu sync.Mutex

	status, err := WriteDep(&mu, dest, []byte("v1"))
	require.NoError(t, err)
	assert.Equal(t, FileNew, status)

	status, err = WriteDep(&mu, dest, []byte("v1"))
	require.NoError(t, err)
	assert.Equal(t, FileUnchanged, status)

	status, err = WriteDep(&mu, dest, []byte("v2"))
	require.NoError(t, err)
	assert.Equal(t, FileUpdated, status)

	got, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Equal(t, []byte("v2"), got)
}
