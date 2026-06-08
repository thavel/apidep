package file

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/bufbuild/protocompile/parser"
	"github.com/bufbuild/protocompile/reporter"
	"github.com/getkin/kin-openapi/openapi3"
	"gopkg.in/yaml.v3"
)

const separator = string(os.PathSeparator)

type Type = string

const (
	TypeUnknown Type = "unknown"
	TypeGrpc    Type = "grpc"
	TypeOpenapi Type = "openapi"
)

// File: api.ref.yml
type ApiRef struct {
	Version int   `yaml:"version"`
	Refs    []Ref `yaml:"refs"`
}

type Ref struct {
	Path   string `yaml:"path"`
	Type   Type   `yaml:"type,omitempty"`   // optional
	Output string `yaml:"output,omitempty"` // optional
}

func ReadRef(filePath string) (*ApiRef, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read ref %s: %w", filePath, err)
	}
	return ParseRef(filePath, data)
}

func ParseRef(filePath string, content []byte) (*ApiRef, error) {
	var refs ApiRef
	if err := yaml.Unmarshal([]byte(content), &refs); err != nil {
		return nil, fmt.Errorf("parse ref %s: %w", filePath, err)
	}
	return &refs, nil
}

func WriteRef(filePath string, ref *ApiRef) error {
	if ref.Version == 0 {
		ref.Version = version
	}
	content, err := serialize(ref)
	if err != nil {
		return fmt.Errorf("serialize ref: %w", err)
	}
	if err := os.WriteFile(filePath, content, filePerm); err != nil {
		return fmt.Errorf("write ref %s: %w", filePath, err)
	}
	return nil
}

func Detect(filePath string) (Type, bool) {
	lower := strings.ToLower(filePath)

	if strings.HasSuffix(lower, ".proto") {
		return TypeGrpc, true
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return TypeUnknown, false
	}

	data := make(map[string]interface{})
	if strings.HasSuffix(lower, ".yml") || strings.HasSuffix(lower, ".yaml") {
		yaml.Unmarshal(content, &data)
	}
	if strings.HasSuffix(lower, ".json") {
		json.Unmarshal(content, &data)
	}
	_, isOpenapi := data["openapi"]
	_, isSwagger := data["swagger"]
	if isOpenapi || isSwagger {
		return TypeOpenapi, true
	}

	return TypeUnknown, false
}

func Scan(root string, maxDepth int) ([]Ref, error) {
	var refs []Ref
	rootDepth := strings.Count(filepath.Clean(root), separator)

	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if maxDepth > 0 {
			depth := strings.Count(filepath.Clean(p), separator) - rootDepth
			if d.IsDir() && depth >= maxDepth {
				return filepath.SkipDir
			}
		}
		if d.IsDir() {
			return nil
		}

		relpath, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		if apiType, ok := Detect(relpath); ok {
			refs = append(refs, Ref{Path: relpath, Type: apiType})
		}
		return nil
	})

	return refs, err
}

func Validate(filePath string, apiType string) error {
	t := apiType
	if len(t) == 0 {
		var ok bool
		t, ok = Detect(filePath)
		if !ok {
			return fmt.Errorf("unable to detect api type")
		}
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("unable to read file %s: %w", filePath, err)
	}
	switch apiType {
	case TypeGrpc:
		handler := reporter.NewHandler(nil)
		_, err := parser.Parse(filePath, bytes.NewReader(content), handler)
		if err != nil {
			return fmt.Errorf("proto parse error in %s: %w", filePath, err)
		}
		if err := handler.Error(); err != nil {
			return fmt.Errorf("proto validation error in %s: %w", filePath, err)
		}
		return nil
	case TypeOpenapi:
		loader := openapi3.NewLoader()
		loader.IsExternalRefsAllowed = true
		handler, err := loader.LoadFromData(content)
		if err != nil {
			return fmt.Errorf("openapi parse error in %s: %w", filePath, err)
		}
		if err := handler.Validate(context.Background()); err != nil {
			return fmt.Errorf("openapi validation error in %s: %w", filePath, err)
		}
		return nil
	default:
		return fmt.Errorf("unknown api type %q", apiType)
	}
}

func serialize(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
