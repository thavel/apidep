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

type FileStatus uint8

const (
	FileNew FileStatus = iota
	FileUnchanged
	FileUpdated
)

// File: api.dep.yml
type ApiDep struct {
	Version int    `yaml:"version"`
	Deps    []Dep  `yaml:"deps"`
	Output  string `yaml:"output"` // optional
}

type Dep struct {
	Source  string `yaml:"source"`
	Version string `yaml:"version"` // optional
	Ref     string `yaml:"ref"`     // optional
	Refs    []Ref  `yaml:"refs"`    // optional
	Output  string `yaml:"output"`  // optional
}

type Provider interface {
	Match(uri string) bool
	Parse(uri, version string) (Source, error)
}

type Source interface {
	Fetch(filePath string) ([]byte, error)
	Commit() string
}

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

func Output(filePath, root, dep, ref, def string) string {
	fileName := filepath.Base(filePath)
	switch {
	case len(ref) > 0:
		if strings.HasSuffix(ref, `/`) || strings.HasSuffix(ref, `\`) {
			return path.Join(path.Dir(ref), fileName)
		}
		// Not necessary, it's just to make sure the path is properly formatted
		return path.Join(path.Dir(ref), filepath.Base(ref))
	case len(dep) > 0:
		return path.Join(path.Dir(dep), fileName)
	case len(root) > 0:
		return path.Join(path.Dir(root), fileName)
	default:
		return path.Join(path.Dir(def), fileName)
	}
}
