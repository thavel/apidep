//go:build integration

// Usage go test -tags integration ./tests/...
package tests

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/thavel/apidep/cmd"
)

// full api.dep.yml (local + remote git) through sync, then ci.
func TestFullSync(t *testing.T) {
	root := stage(t, true)
	t.Chdir(root)

	require.NoError(t, cmd.Sync.Run(t.Context(),
		[]string{"sync", "-f", "api.dep.yml", "-l", "api.lock.yml"}))

	// Remote git dependencies.
	require.FileExists(t, filepath.Join(root, "apidep", "openapi", "api-example.yaml"))
	require.FileExists(t, filepath.Join(root, "apidep", "grpc", "geobuf.proto"))
	// Local dependency.
	require.FileExists(t, filepath.Join(root, "apidep", "sample.yml"))
	require.FileExists(t, filepath.Join(root, "apidep", "sample.proto"))

	require.NoError(t, cmd.CI.Run(t.Context(),
		[]string{"ci", "-f", "api.dep.yml", "-l", "api.lock.yml"}))
}
