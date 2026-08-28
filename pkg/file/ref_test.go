package file

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func TestParseRef_RejectsGlob(t *testing.T) {
	patterns := []string{
		"openapi/*.yaml",
		"openapi/api-?.yaml",
		"openapi/api-[abc].yaml",
	}
	for _, p := range patterns {
		t.Run(p, func(t *testing.T) {
			content := []byte("version: 1\nrefs:\n  - path: " + p + "\n    type: openapi\n")
			_, err := ParseRef("api.ref.yml", content)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "glob patterns are not allowed")
		})
	}
}

func TestParseRef_AcceptsLiteralPaths(t *testing.T) {
	content := []byte("version: 1\nrefs:\n  - path: openapi/api.yaml\n    type: openapi\n")
	ref, err := ParseRef("api.ref.yml", content)
	require.NoError(t, err)
	require.Len(t, ref.Refs, 1)
	assert.Equal(t, "openapi/api.yaml", ref.Refs[0].Path)
	assert.Equal(t, TypeOpenapi, ref.Refs[0].Type)
}

func TestParseRef_InvalidYAML(t *testing.T) {
	// version is an int; a non-numeric value fails to unmarshal.
	_, err := ParseRef("api.ref.yml", []byte("version: not-a-number\n"))
	require.Error(t, err)
}

// fakeSource lets us drive Ref.Inputs without touching the real providers.
type fakeSource struct {
	glob map[string][]string
}

func (f fakeSource) Fetch(string) ([]byte, error) { return nil, nil }
func (f fakeSource) Commit() string               { return "" }
func (f fakeSource) Glob(pattern string) ([]string, error) {
	return f.glob[pattern], nil
}

func TestInputs_NilSourceReturnsLiteralPath(t *testing.T) {
	r := Ref{Path: "openapi/*.yaml"}
	got, err := r.Inputs(nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"openapi/*.yaml"}, got)
}

func TestInputs_WithSourceExpandsGlob(t *testing.T) {
	src := fakeSource{glob: map[string][]string{
		"openapi/*.yaml": {"openapi/a.yaml", "openapi/b.yaml"},
	}}
	r := Ref{Path: "openapi/*.yaml"}
	got, err := r.Inputs(src)
	require.NoError(t, err)
	assert.Equal(t, []string{"openapi/a.yaml", "openapi/b.yaml"}, got)
}

func TestDetect(t *testing.T) {
	dir := t.TempDir()

	proto := filepath.Join(dir, "svc.proto")
	require.NoError(t, os.WriteFile(proto, []byte("syntax = \"proto3\";"), 0o644))
	typ, ok := Detect(proto)
	assert.True(t, ok)
	assert.Equal(t, TypeGrpc, typ)

	oapi := filepath.Join(dir, "api.yaml")
	require.NoError(t, os.WriteFile(oapi, []byte("openapi: 3.0.0\n"), 0o644))
	typ, ok = Detect(oapi)
	assert.True(t, ok)
	assert.Equal(t, TypeOpenapi, typ)

	swagger := filepath.Join(dir, "legacy.json")
	require.NoError(t, os.WriteFile(swagger, []byte(`{"swagger":"2.0"}`), 0o644))
	typ, ok = Detect(swagger)
	assert.True(t, ok)
	assert.Equal(t, TypeOpenapi, typ)

	plain := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(plain, []byte("foo: bar\n"), 0o644))
	_, ok = Detect(plain)
	assert.False(t, ok)
}

func TestScan(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "api.yaml"), "openapi: 3.0.0\n")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "nested"), 0o755))
	write(t, filepath.Join(dir, "nested", "svc.proto"), "syntax = \"proto3\";")
	write(t, filepath.Join(dir, "ignore.txt"), "nope")

	// Detect reads contents relative to the CWD, so openapi/swagger
	// detection only works when the scanned dir is the CWD.
	t.Chdir(dir)

	refs, err := Scan(".", 0)
	require.NoError(t, err)
	require.Len(t, refs, 2)

	byPath := map[string]Type{}
	for _, r := range refs {
		byPath[r.Path] = r.Type
	}
	assert.Equal(t, TypeOpenapi, byPath["api.yaml"])
	assert.Equal(t, TypeGrpc, byPath[filepath.Join("nested", "svc.proto")])
}

func TestScan_MaxDepth(t *testing.T) {
	// .proto is detected by extension (no CWD dependency), isolating depth.
	dir := t.TempDir()
	write(t, filepath.Join(dir, "top.proto"), "syntax = \"proto3\";")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "nested"), 0o755))
	write(t, filepath.Join(dir, "nested", "deep.proto"), "syntax = \"proto3\";")

	refs, err := Scan(dir, 1)
	require.NoError(t, err)
	require.Len(t, refs, 1)
	assert.Equal(t, "top.proto", refs[0].Path)
}

func TestValidate(t *testing.T) {
	dir := t.TempDir()

	validOApi := filepath.Join(dir, "api.yaml")
	write(t, validOApi, "openapi: 3.0.0\ninfo:\n  title: t\n  version: \"1\"\npaths: {}\n")
	assert.NoError(t, Validate(t.Context(), validOApi, TypeOpenapi))

	badOApi := filepath.Join(dir, "bad.yaml")
	write(t, badOApi, "openapi: 3.0.0\n")
	assert.Error(t, Validate(t.Context(), badOApi, TypeOpenapi))

	validProto := filepath.Join(dir, "svc.proto")
	write(t, validProto, "syntax = \"proto3\";\nmessage M {}\n")
	assert.NoError(t, Validate(t.Context(), validProto, TypeGrpc))

	badProto := filepath.Join(dir, "bad.proto")
	write(t, badProto, "this is not proto")
	assert.Error(t, Validate(t.Context(), badProto, TypeGrpc))
}

// empty apiType falls back to Detect instead of erroring as unknown.
func TestValidate_DetectsType(t *testing.T) {
	dir := t.TempDir()

	oapi := filepath.Join(dir, "api.yaml")
	write(t, oapi, "openapi: 3.0.0\ninfo:\n  title: t\n  version: \"1\"\npaths: {}\n")
	assert.NoError(t, Validate(t.Context(), oapi, ""))

	proto := filepath.Join(dir, "svc.proto")
	write(t, proto, "syntax = \"proto3\";\nmessage M {}\n")
	assert.NoError(t, Validate(t.Context(), proto, ""))
}

func TestValidate_UnknownType(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "x.txt")
	write(t, f, "hello")
	assert.Error(t, Validate(t.Context(), f, ""))
}

func TestReadWriteRef_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "api.ref.yml")

	in := &ApiRef{Refs: []Ref{
		{Path: "openapi/api.yaml", Type: TypeOpenapi},
		{Path: "grpc/svc.proto", Type: TypeGrpc},
	}}
	require.NoError(t, WriteRef(path, in))

	out, err := ReadRef(path)
	require.NoError(t, err)
	assert.Equal(t, version, out.Version)
	assert.Equal(t, in.Refs, out.Refs)
}
