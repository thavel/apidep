package file

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

const (
	version  = 1
	filePerm = 0o644
	dirPerm  = 0o755
)

// FileStatus reports what WriteDep did to an output file.
type FileStatus uint8

const (
	FileNew       FileStatus = iota // output did not exist
	FileUnchanged                   // output already matched
	FileUpdated                     // output existed and was overwritten
)

// ApiDep mirrors the api.dep.yml file.
type ApiDep struct {
	Version int    `yaml:"version"`
	Deps    []Dep  `yaml:"deps"`
	Output  string `yaml:"output"` // optional
}

// Dep is a single dependency declared in api.dep.yml.
type Dep struct {
	Source  string `yaml:"source"`
	Version string `yaml:"version"` // optional
	Ref     string `yaml:"ref"`     // optional
	Refs    []Ref  `yaml:"refs"`    // optional
	Output  string `yaml:"output"`  // optional
}

// Provider matches a dependency source and opens it for reading.
type Provider interface {
	Match(uri string) bool
	Parse(uri, version string) (Source, error)
}

// Source reads files from a resolved dependency at a fixed revision.
type Source interface {
	Fetch(filePath string) ([]byte, error)
	Commit() string
	Glob(pattern string) ([]string, error)
}

// ReadDep loads and parses an api.dep.yml file.
func ReadDep(filePath string) (*ApiDep, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read dep %s: %w", filePath, err)
	}
	var deps ApiDep
	if err := yaml.Unmarshal(data, &deps); err != nil {
		return nil, fmt.Errorf("parse dep %s: %w", filePath, err)
	}
	return &deps, nil
}

// WriteDep writes content to dest, reporting whether it was new, unchanged
// or updated.
func WriteDep(mu *sync.Mutex, dest string, content []byte) (FileStatus, error) {
	dir := path.Dir(dest)
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return FileNew, err
	}

	var status FileStatus
	existing, err := os.ReadFile(dest)
	switch {
	case err != nil:
		status = FileNew
	case bytes.Equal(existing, content):
		status = FileUnchanged
	default:
		status = FileUpdated
	}

	mu.Lock()
	defer mu.Unlock()
	return status, os.WriteFile(dest, content, filePerm)
}

// Output resolves the local destination for filePath, using the first of
// ref, dep, root or def that is set.
func Output(filePath, root, dep, ref, def string) string {
	fileName := filepath.Base(filePath)
	switch {
	case len(ref) > 0:
		if strings.HasSuffix(ref, `/`) || strings.HasSuffix(ref, `\`) {
			return path.Join(path.Dir(ref), fileName)
		}
		if strings.HasSuffix(ref, `/.`) || strings.HasSuffix(ref, `\.`) {
			return path.Join(path.Dir(ref), filePath)
		}
		// ref is an explicit output file
		return path.Join(path.Dir(ref), filepath.Base(ref))
	case len(dep) > 0:
		if strings.HasSuffix(dep, `/.`) || strings.HasSuffix(dep, `\.`) {
			return path.Join(path.Dir(dep), filePath)
		}
		return path.Join(path.Dir(dep), fileName)
	case len(root) > 0:
		return path.Join(path.Dir(root), fileName)
	default:
		return path.Join(path.Dir(def), fileName)
	}
}
