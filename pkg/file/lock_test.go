package file

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHash_OrderIndependent(t *testing.T) {
	a := map[string][]byte{"a.yaml": []byte("A"), "b.yaml": []byte("B")}
	b := map[string][]byte{"b.yaml": []byte("B"), "a.yaml": []byte("A")}
	assert.Equal(t, Hash(a), Hash(b))
}

func TestHash_SensitiveToContent(t *testing.T) {
	a := map[string][]byte{"a.yaml": []byte("A")}
	b := map[string][]byte{"a.yaml": []byte("different")}
	assert.NotEqual(t, Hash(a), Hash(b))
}

func TestHash_SensitiveToPath(t *testing.T) {
	a := map[string][]byte{"a.yaml": []byte("A")}
	b := map[string][]byte{"b.yaml": []byte("A")}
	assert.NotEqual(t, Hash(a), Hash(b))
}

func TestHash_VersionPrefix(t *testing.T) {
	h := Hash(map[string][]byte{"a.yaml": []byte("A")})
	assert.True(t, strings.HasPrefix(h, prefix(version)+":"))
}

func TestApiLock_UpsertAndGet(t *testing.T) {
	l := NewLock()
	assert.Equal(t, version, l.Version)

	_, ok := l.Get("src")
	assert.False(t, ok)

	assert.True(t, l.Upsert(DepLock{Source: "src", Hash: "h1"}))
	require.Len(t, l.Deps, 1)

	entry, ok := l.Get("src")
	require.True(t, ok)
	assert.Equal(t, "h1", entry.Hash)

	// Upsert with same source and same content: no change.
	assert.False(t, l.Upsert(DepLock{Source: "src", Hash: "h1"}))
	require.Len(t, l.Deps, 1)

	// Upsert with same source but different hash replaces, does not append.
	assert.True(t, l.Upsert(DepLock{Source: "src", Hash: "h2"}))
	require.Len(t, l.Deps, 1)
	entry, _ = l.Get("src")
	assert.Equal(t, "h2", entry.Hash)

	assert.True(t, l.Upsert(DepLock{Source: "other", Hash: "h3"}))
	require.Len(t, l.Deps, 2)
}

func TestReadLock_MissingReturnsEmpty(t *testing.T) {
	l, err := ReadLock(filepath.Join(t.TempDir(), "absent.yml"))
	require.NoError(t, err)
	assert.Equal(t, version, l.Version)
	assert.Empty(t, l.Deps)
}

func TestReadWriteLock_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "api.lock.yml")

	in := NewLock()
	in.Upsert(DepLock{Source: "src", Commit: "abc", Hash: "h1"})
	require.NoError(t, WriteLock(path, in))

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(string(raw), header))

	out, err := ReadLock(path)
	require.NoError(t, err)
	assert.Equal(t, in.Deps, out.Deps)
}
