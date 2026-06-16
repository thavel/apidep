# apidep

apidep is a command-line tool for managing API definition file dependencies. It fetches OpenAPI and gRPC (Protocol Buffers) specification files from remote sources, writes them to a local directory, and keeps them in sync across machines and CI pipelines.


## Background

Sharing API definitions between teams and repositories is a recurring problem. The usual answer in the gRPC ecosystem is [Buf](https://buf.build), which provides a centralised schema registry. You publish your schemas to buf.build, and consumers pull from there. That model works well when everyone is willing to adopt a single registry and the toolchain that comes with it.

apidep takes a different approach: it is fully decentralised and relies entirely on Git. There is no registry to sign up for, no account to manage, and no vendor lock-in. If you can clone a repository, you can depend on its API files. This also means the tooling integrates naturally with the access controls you already have in place through your Git hosting provider.


## Installation

### curl

```shell
curl -fsSL https://raw.githubusercontent.com/thavel/apidep/main/install.sh | sh
```

Downloads the latest pre-built binary for your OS/arch and installs to `/usr/local/bin`.

### Go

```shell
go install github.com/thavel/apidep@latest
```

### Docker

```shell
docker pull ghcr.io/thavel/apidep:latest
```


## Configuration files

### `api.ref.yml` (for API providers)

This manifest should be versionned alongside your API definitions to advertise which files are available for consumers. apidep reads this file from the remote source when no inline `refs` are provided.

```yaml
version: 1
refs:
  - path: samples/sample.proto
    type: grpc
  - path: samples/sample.yml
    type: openapi
```

### `api.dep.yml` (for API consumers)

```yaml
version: 1
output: ./apidep/
deps:
  # Remote source
  - source: git@github.com:teamdigitale/api-openapi-samples.git
    version: v1.0.1            # optional (default: main branch)
    output: ./apidep/openapi/  # optional
    # optional, if the repo doesn't provide a api.ref.yml file
    refs:
      - path: openapi-v3/api-example.yaml
        output: ./apidep/openapi/api-example.yaml  # optional
        type: openapi
  # Local path (for dev and mono-repo)
  - source: ./
    ref: api.ref.yml
```

The file supports a top-level `output` field and per-dependency fields:

| Field     | Level | Required | Description |
|-----------|-------|----------|-------------|
| `output`  | root  | no       | Default output directory for all downloaded files. Defaults to `apidep/`. |
| `source`  | dep   | yes      | Git URL (SSH or HTTPS) or local path |
| `version` | dep   | no       | Branch name, tag, or commit SHA. Defaults to HEAD. |
| `ref`     | dep   | no       | Path to a remote `api.ref.yml` file describing which files to fetch. Mutually exclusive with `refs`. |
| `refs`    | dep   | no       | Inline list of files to fetch. Mutually exclusive with `ref`. |
| `output`  | dep   | no       | Output directory for all files from this source. Overrides the root `output`. |

Each item in `refs` supports:

| Field    | Required | Description |
|----------|----------|-------------|
| `path`   | yes      | Path to the file within the source repository |
| `type`   | no       | `openapi` or `grpc`. Inferred from the file extension and content if omitted. |
| `output` | no       | Output path for this specific file. Overrides both dep-level and root-level `output`. If it ends with `/` it is treated as a directory and the original filename is appended. Otherwise it is used as the exact destination path, allowing the file to be renamed. |

Output resolution priority (highest wins): `refs[].output` > `deps[].output` > root `output` > `apidep/`.

### `api.lock.yml`

Generated automatically by `sync`. Records the resolved commit and a content hash for each dependency source, so that unintended upstream changes are detected in CI.


## Commands

### `init`

Scans a directory for API definition files and generates an `api.ref.yml`. Intended for API producers that want to publish a reference manifest alongside their definitions.

### `sync`

Fetches all dependencies declared in `api.dep.yml` and writes them to the local output directory. Validates each file after writing unless `--no-validate` is set. Updates `api.lock.yml` with the resolved commits and content hashes.

### `validate`

Validates the refs declared in `api.ref.yml`. Each file is validated against its declared or inferred type. OpenAPI files are validated with kin-openapi. Proto files are parsed with protocompile.

### `ci`

Intended for use in CI pipelines. For each dependency, ensures that the file exists, the format is valid, and the lock is up-to-date.


## Typical workflow

**For API producers** (publishing an `api.ref.yml` in their repository):

```shell
apidep init
git add api.ref.yml
git commit -m "add api ref manifest"
```

**For API consumers** (depending on remote API files):

```shell
# Add your api.dep.yml file, then:
apidep sync

# In CI:
apidep ci
```

## Supported source types

| Source | Example |
|--------|---------|
| SSH Git URL | `git@github.com:org/repo.git` |
| HTTPS Git URL | `https://github.com/org/repo.git` |
| Local path | `./`, `../other-service`, `/absolute/path` |

SSH authentication uses the system agent if available, falling back to key files in `~/.ssh` (`id_ed25519`, `id_ecdsa`, `id_rsa`).

## Supported API types

| Type | Extensions | Validation |
|------|-----------|------------|
| `openapi` | `.yml`, `.yaml` | Parsed and validated with [kin-openapi](https://github.com/getkin/kin-openapi) (OpenAPI 2 and 3) |
| `grpc` | `.proto` | Parsed with [protocompile](https://github.com/bufbuild/protocompile) |
