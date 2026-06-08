package provider

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/thavel/apidep/pkg/file"
)

type FS struct{}

func (*FS) Match(uri string) bool {
	return strings.HasPrefix(uri, "/") ||
		strings.HasPrefix(uri, "./") ||
		strings.HasPrefix(uri, "../") ||
		strings.HasPrefix(uri, "file://")
}

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
