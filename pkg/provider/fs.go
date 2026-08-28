package provider

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/thavel/apidep/pkg/file"
)

// FS is a file.Provider for local filesystem paths.
type FS struct{}

// Match reports whether uri is a local path or file:// URL.
func (*FS) Match(uri string) bool {
	return strings.HasPrefix(uri, "/") ||
		strings.HasPrefix(uri, "./") ||
		strings.HasPrefix(uri, "../") ||
		strings.HasPrefix(uri, "file://")
}

// Parse opens uri as a local source; version is ignored.
func (*FS) Parse(uri, version string) (file.Source, error) {
	base := strings.TrimPrefix(uri, "file://")
	return &fsSource{base: base}, nil
}

type fsSource struct {
	base string
}

func (s *fsSource) Fetch(filePath string) ([]byte, error) {
	full := filepath.Join(s.base, filePath)
	content, err := os.ReadFile(full)
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", full, err)
	}
	return content, nil
}

func (s *fsSource) Commit() string {
	return ""
}

func (s *fsSource) Glob(pattern string) ([]string, error) {
	full := filepath.Join(s.base, pattern)
	matches, err := filepath.Glob(full)
	if err != nil {
		return nil, err
	}
	// base-relative, like the git provider: else Fetch re-joins base and doubles prefix
	out := make([]string, len(matches))
	for i, m := range matches {
		rel, err := filepath.Rel(s.base, m)
		if err != nil {
			return nil, err
		}
		out[i] = rel
	}
	return out, nil
}
