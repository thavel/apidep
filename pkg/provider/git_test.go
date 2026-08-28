package provider

import (
	"sort"
	"testing"
	"time"

	"github.com/go-git/go-billy/v6/memfs"
	gogit "github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/go-git/go-git/v6/storage/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGit_Match(t *testing.T) {
	g := &Git{}
	assert.True(t, g.Match("git@github.com:org/repo.git"))
	assert.True(t, g.Match("https://github.com/org/repo.git"))
	assert.True(t, g.Match("ssh://git@host/org/repo.git"))
}

func TestIsSSHURI(t *testing.T) {
	assert.True(t, isSSHURI("ssh://git@host/repo.git"))
	assert.True(t, isSSHURI("git@github.com:org/repo.git"))
	assert.False(t, isSSHURI("https://github.com/org/repo.git"))
	assert.False(t, isSSHURI("/local/path"))
}

func TestSSHUser(t *testing.T) {
	assert.Equal(t, "git", sshUser("git@github.com:org/repo.git"))
	assert.Equal(t, "deploy", sshUser("deploy@host:org/repo.git"))
	assert.Equal(t, "git", sshUser("ssh://host/repo.git"))
	assert.Equal(t, "alice", sshUser("ssh://alice@host/repo.git"))
}

func TestBuildAuth_HTTPSNoCredentials(t *testing.T) {
	opts, err := buildAuth("https://github.com/org/repo.git")
	require.NoError(t, err)
	assert.Nil(t, opts)
}

func TestBuildAuth_InvalidScheme(t *testing.T) {
	_, err := buildAuth("ftp://example.com/repo")
	require.Error(t, err)
}

// newTestRepo builds an in-memory git repo with the given files and returns a
// gitSource pointing at the single commit.
func newTestRepo(t *testing.T, files map[string]string) *gitSource {
	t.Helper()

	fs := memfs.New()
	repo, err := gogit.Init(memory.NewStorage(), gogit.WithWorkTree(fs))
	require.NoError(t, err)

	wt, err := repo.Worktree()
	require.NoError(t, err)

	for name, content := range files {
		if dir := dirOf(name); dir != "" {
			require.NoError(t, fs.MkdirAll(dir, 0o755))
		}
		f, err := fs.Create(name)
		require.NoError(t, err)
		_, err = f.Write([]byte(content))
		require.NoError(t, err)
		require.NoError(t, f.Close())
		_, err = wt.Add(name)
		require.NoError(t, err)
	}

	hash, err := wt.Commit("init", &gogit.CommitOptions{
		Author: &object.Signature{Name: "t", Email: "t@t", When: time.Now()},
	})
	require.NoError(t, err)

	return &gitSource{repo: repo, hash: hash}
}

func dirOf(name string) string {
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '/' {
			return name[:i]
		}
	}
	return ""
}

func TestGitSource_Fetch(t *testing.T) {
	src := newTestRepo(t, map[string]string{
		"openapi/a.yaml": "A",
		"openapi/b.yaml": "B",
	})

	content, err := src.Fetch("openapi/a.yaml")
	require.NoError(t, err)
	assert.Equal(t, "A", string(content))

	_, err = src.Fetch("missing.yaml")
	assert.Error(t, err)

	assert.NotEmpty(t, src.Commit())
}

func TestGitSource_Glob(t *testing.T) {
	src := newTestRepo(t, map[string]string{
		"openapi/a.yaml": "A",
		"openapi/b.yaml": "B",
		"openapi/c.json": "C",
		"grpc/svc.proto": "P",
	})

	matches, err := src.Glob("openapi/*.yaml")
	require.NoError(t, err)
	sort.Strings(matches)
	assert.Equal(t, []string{"openapi/a.yaml", "openapi/b.yaml"}, matches)

	// Glob results round-trip through Fetch.
	for _, m := range matches {
		_, err := src.Fetch(m)
		require.NoError(t, err)
	}
}

func TestGitSource_GlobNoMatch(t *testing.T) {
	src := newTestRepo(t, map[string]string{"openapi/a.yaml": "A"})
	matches, err := src.Glob("grpc/*.proto")
	require.NoError(t, err)
	assert.Empty(t, matches)
}
