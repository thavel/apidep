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

// Type is an api definition kind (grpc, openapi).
type Type = string

const (
	TypeUnknown Type = "unknown" // type could not be detected
	TypeGrpc    Type = "grpc"    // protobuf / gRPC
	TypeOpenapi Type = "openapi" // OpenAPI / Swagger
)

// ApiRef mirrors the api.ref.yml manifest published by a provider.
type ApiRef struct {
	Version int   `yaml:"version"`
	Refs    []Ref `yaml:"refs"`
}

// Ref is a single api definition entry in a ref manifest.
type Ref struct {
	Path   string `yaml:"path"`
	Type   Type   `yaml:"type,omitempty"`   // optional
	Output string `yaml:"output,omitempty"` // optional
}

// ReadRef loads and parses an api.ref.yml file.
func ReadRef(filePath string) (*ApiRef, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read ref %s: %w", filePath, err)
	}
	return ParseRef(filePath, data)
}

// Inputs expands the ref path against src, or returns it verbatim when src is
// nil (globs are only resolved against a source).
func (r Ref) Inputs(src Source) ([]string, error) {
	if src == nil {
		return []string{r.Path}, nil
	}
	return src.Glob(r.Path)
}

// ParseRef parses ref manifest content and rejects glob patterns in paths.
func ParseRef(filePath string, content []byte) (*ApiRef, error) {
	var refs ApiRef
	if err := yaml.Unmarshal([]byte(content), &refs); err != nil {
		return nil, fmt.Errorf("parse ref %s: %w", filePath, err)
	}
	for _, ref := range refs.Refs {
		if strings.ContainsAny(ref.Path, "*?[") {
			return nil, fmt.Errorf(
				"parse ref %s: glob patterns are not allowed, list files explicitly: %q",
				filePath, ref.Path,
			)
		}
	}
	return &refs, nil
}

// WriteRef serializes ref to filePath, defaulting the version when unset.
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

// Detect infers the api type from a file's extension and contents. The bool
// is false when the type can't be determined.
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

// Scan walks root and returns a Ref for each detected api file. maxDepth of 0
// means unlimited. Detection reads file contents relative to the CWD.
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

// Validate parses filePath as apiType, detecting the type when apiType is empty.
func Validate(ctx context.Context, filePath, apiType string) error {
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
	switch t {
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
		if err := handler.Validate(ctx); err != nil {
			return fmt.Errorf("openapi validation error in %s: %w", filePath, err)
		}
		return nil
	default:
		return fmt.Errorf("unknown api type %q", t)
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
