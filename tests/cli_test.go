package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/thavel/apidep/cmd"
	"github.com/thavel/apidep/pkg/file"
	"github.com/thavel/apidep/pkg/provider"
)

// copyTree recursively copies src into dst, preserving structure.
func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	require.NoError(t, filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	}))
}

// stage copies the committed demo files into a fresh temp dir and returns its
// path. The lock file is only needed by the integration test.
func stage(t *testing.T, withLock bool) string {
	t.Helper()
	root := t.TempDir()
	for _, name := range []string{"api.dep.yml", "api.ref.yml"} {
		data, err := os.ReadFile(name)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(root, name), data, 0o644))
	}
	copyTree(t, "samples", filepath.Join(root, "samples"))
	if withLock {
		data, err := os.ReadFile("api.lock.yml")
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(root, "api.lock.yml"), data, 0o644))
	}
	return root
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}

const validOpenAPI = "openapi: 3.0.0\ninfo:\n  title: t\n  version: \"1\"\npaths: {}\n"

// writeFile writes content to dir/name, creating parent directories.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	full := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
	require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
}

// keepLocalDeps rewrites the dep file in dir to keep only dependencies served
// by the local FS provider, so sync can run offline.
func keepLocalDeps(t *testing.T, dir string) {
	t.Helper()
	dep, err := file.ReadDep(filepath.Join(dir, "api.dep.yml"))
	require.NoError(t, err)

	fs := &provider.FS{}
	kept := dep.Deps[:0]
	for _, d := range dep.Deps {
		if fs.Match(d.Source) {
			kept = append(kept, d)
		}
	}
	dep.Deps = kept
	require.NotEmpty(t, dep.Deps, "expected at least one local dependency in api.dep.yml")

	out, err := yaml.Marshal(dep)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "api.dep.yml"), out, 0o644))
}

func TestValidate(t *testing.T) {
	root := stage(t, false)
	t.Chdir(root)

	require.NoError(t, cmd.Validate.Run(t.Context(),
		[]string{"validate", "-r", "api.ref.yml"}))
}

// sync writes the samples and lock, then ci confirms they're up to date.
func TestSyncCI(t *testing.T) {
	root := stage(t, false)
	t.Chdir(root)
	keepLocalDeps(t, root)

	require.NoError(t, cmd.Sync.Run(t.Context(),
		[]string{"sync", "-f", "api.dep.yml", "-l", "api.lock.yml"}))

	// The local dep flattens paths to their base name under ./apidep/.
	require.FileExists(t, filepath.Join(root, "apidep", "sample.yml"))
	require.FileExists(t, filepath.Join(root, "apidep", "sample.proto"))
	require.Equal(t,
		readFile(t, filepath.Join(root, "samples", "sample.yml")),
		readFile(t, filepath.Join(root, "apidep", "sample.yml")))

	require.NoError(t, cmd.CI.Run(t.Context(),
		[]string{"ci", "-f", "api.dep.yml", "-l", "api.lock.yml"}))
}

func TestCIDetectsTampering(t *testing.T) {
	root := stage(t, false)
	t.Chdir(root)
	keepLocalDeps(t, root)

	require.NoError(t, cmd.Sync.Run(t.Context(),
		[]string{"sync", "-f", "api.dep.yml", "-l", "api.lock.yml"}))

	require.NoError(t, os.WriteFile(
		filepath.Join(root, "apidep", "sample.yml"), []byte("tampered\n"), 0o644))

	err := cmd.CI.Run(t.Context(), []string{"ci", "-f", "api.dep.yml", "-l", "api.lock.yml"})
	require.Error(t, err)
}

// files match the remote but the dep is absent from the lock.
func TestCIMissingFromLock(t *testing.T) {
	root := stage(t, false)
	t.Chdir(root)
	keepLocalDeps(t, root)

	require.NoError(t, cmd.Sync.Run(t.Context(),
		[]string{"sync", "-f", "api.dep.yml", "-l", "api.lock.yml"}))

	err := cmd.CI.Run(t.Context(),
		[]string{"ci", "-f", "api.dep.yml", "-l", "empty.lock.yml"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing from lock file")
}

func TestInit(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeFile(t, dir, "api.yaml", validOpenAPI)
	writeFile(t, dir, "grpc/svc.proto", "syntax = \"proto3\";\nmessage M {}\n")

	require.NoError(t, cmd.Init.Run(t.Context(), []string{"init", "-o", "api.ref.yml"}))

	ref, err := file.ReadRef(filepath.Join(dir, "api.ref.yml"))
	require.NoError(t, err)
	require.Len(t, ref.Refs, 2)

	byPath := map[string]file.Type{}
	for _, r := range ref.Refs {
		byPath[r.Path] = r.Type
	}
	require.Equal(t, file.TypeOpenapi, byPath["api.yaml"])
	require.Equal(t, file.TypeGrpc, byPath[filepath.Join("grpc", "svc.proto")])
}

// nothing to scan: init succeeds without writing a ref.
func TestInitNoFiles(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	require.NoError(t, cmd.Init.Run(t.Context(), []string{"init", "-o", "api.ref.yml"}))
	require.NoFileExists(t, filepath.Join(dir, "api.ref.yml"))
}

func TestValidateInvalidFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeFile(t, dir, "bad.yaml", "openapi: 3.0.0\n") // missing info/paths
	writeFile(t, dir, "api.ref.yml",
		"version: 1\nrefs:\n  - path: bad.yaml\n    type: openapi\n")

	err := cmd.Validate.Run(t.Context(), []string{"validate", "-r", "api.ref.yml"})
	require.Error(t, err)
}

func TestValidateRejectsGlob(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeFile(t, dir, "api.ref.yml",
		"version: 1\nrefs:\n  - path: '*.yaml'\n    type: openapi\n")

	err := cmd.Validate.Run(t.Context(), []string{"validate", "-r", "api.ref.yml"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "glob patterns are not allowed")
}

func TestSyncRejectsGlobInRefFile(t *testing.T) {
	remote := t.TempDir()
	writeFile(t, remote, "a.yaml", validOpenAPI)
	writeFile(t, remote, "api.ref.yml",
		"version: 1\nrefs:\n  - path: '*.yaml'\n    type: openapi\n")

	work := t.TempDir()
	t.Chdir(work)
	writeFile(t, work, "api.dep.yml",
		"version: 1\ndeps:\n  - source: "+remote+"\n    ref: api.ref.yml\n")

	err := cmd.Sync.Run(t.Context(),
		[]string{"sync", "-f", "api.dep.yml", "-l", "api.lock.yml"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "glob patterns are not allowed")
}

func TestSyncRefAndRefsMutuallyExclusive(t *testing.T) {
	work := t.TempDir()
	t.Chdir(work)
	writeFile(t, work, "api.dep.yml",
		"version: 1\ndeps:\n  - source: "+work+"\n    ref: api.ref.yml\n"+
			"    refs:\n      - path: a.yaml\n")

	err := cmd.Sync.Run(t.Context(),
		[]string{"sync", "-f", "api.dep.yml", "-l", "api.lock.yml"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "mutually exclusive")
}
