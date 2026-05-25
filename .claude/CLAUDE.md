# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

# Velox - RoadRunner Build System

## Project Overview

Velox is an automated build system for RoadRunner server and its plugins. The v3 milestone (current) drives the build through `go mod edit` rather than a hand-written `go.mod` template, supports `[[replaces]]` and `[[excludes]]` directives, and ships with deterministic plugin prefixes for reproducible artifacts.

**Pipeline:**

1. Download RoadRunner source archive from GitHub (tag, branch, or 40-char SHA). Archive bytes are cached in-process (LRU, 32 entries) so repeat builds of the same ref skip the network.
2. Preserve the upstream `go.mod` as-is — it already pins informer/resetter and the core deps.
3. Render `container/plugins.go` from a single parameterized template. The informer/resetter major version is read out of the upstream `go.mod` at build time, so one template covers every RR major.
4. Apply user-supplied `require`, `replace`, and `exclude` directives via `go mod edit`.
5. Run `go mod tidy`. Verify each user plugin resolved to the requested tag (else fail with an actionable error).
6. Run `go build` with `-trimpath`, version ldflags, and (optionally) `-race` / debug flags.
7. Smoke-test the binary (`./rr --version`) when host platform == target platform.

**Two modes:**

- **CLI (`vx build`)**: local builds driven by `velox.toml`.
- **Build server (`vx server`)**: Connect/gRPC service with LRU caching of built binaries.

**Key technologies:**

- Go 1.26+ (module path: `github.com/roadrunner-server/velox/v3`)
- Protocol Buffers via [buf](https://buf.build/)
- Connect RPC (`connectrpc.com/connect`) and gRPC
- `hashicorp/golang-lru/v2` for caches
- Cobra CLI

**Not supported in v3:** Windows targets.

## Repository structure

```text
├── api/                         # Protocol Buffers (BuildService RPC)
├── builder/
│   ├── builder.go              # Build pipeline (decomposed into named steps)
│   ├── gomod.go                # `go mod edit/tidy` driver + runCmd helper
│   ├── options.go              # Functional options
│   ├── runtime.go              # GOOS/GOARCH indirection for tests
│   └── templates/
│       ├── plugins_template.go # Single parameterized plugins.go template
│       └── template_test.go
├── cmd/vx/                     # Main CLI entry point
├── config.go                   # Config, Replace, Exclude, validation
├── gen/                        # buf-generated protobuf code
├── github/
│   ├── github.go               # Archive download + extraction
│   └── cache.go                # LRU-backed Cache implementation
├── internal/cli/               # cobra wiring
│   ├── build/                  # `vx build`
│   └── server/                 # `vx server` (Connect + gRPC reflection)
├── plugin/                     # Plugin metadata + deterministic prefix
├── logger/                     # zap logger builder
└── velox.toml                  # Sample configuration
```

`go.mod` has the replace directive `github.com/roadrunner-server/velox/v3/gen => ./gen` so the generated protobuf code is consumed as a local module.

## Common commands

```bash
make test          # go test -v -race ./...
make regenerate    # rm -rf ./gen && buf generate && buf format -w

go build -o vx ./cmd/vx
./vx -c velox.toml build -o ./output
./vx -c velox.toml server -a 127.0.0.1:8080

go test -cover ./...
golangci-lint run
```

## Core architecture

### Build pipeline (`builder/builder.go:Build`)

```text
prepareWorkspace → writePluginsGo → applyRequires → applyReplaces
  → applyExcludes → goModTidy → verifyResolvedVersions
  → compile → relocate → smokeTest
```

Each step propagates `context.Context` and surfaces the last 8 KB of stderr in any returned error.

### Plugin prefixing (`plugin/plugin.go`)

Every plugin gets a deterministic 5-letter alpha-lowercase prefix derived from `sha256(moduleName)`. Collisions across a single build are resolved by `ResolvePrefixCollisions`, which re-salts conflicting prefixes. Two builds with the same plugin set produce bit-identical `plugins.go`.

### Subprocess execution (`builder/gomod.go:runCmd`)

`runCmd` wraps `exec.Command` with: context propagation (SIGINT then SIGKILL after 15s on `ctx.Done()`), full stdout capture, bounded ring-buffer stderr capture (last 8 KB), and stderr tee to the debug logger.

### Server cache key

`server.go:generateCacheHash` produces a deterministic FNV-64a hash over a sorted `BuildRequest` (plugins by module, replaces by `old`, excludes by `module+version`). The `RequestId` field is excluded.

### Key files

- `builder/builder.go` — pipeline orchestration
- `builder/gomod.go` — `go mod edit/tidy` + stderr-bounded runCmd
- `builder/templates/plugins_template.go` — sole template + upstream `go.mod` parser
- `config.go` — `Config`, `Replace`, `Exclude`, validation (incl. Windows rejection)
- `plugin/plugin.go` — deterministic prefix + collision resolver
- `github/github.go` — archive download (GHE-aware) + zip extraction with CWE-22 guard
- `internal/cli/server/server.go` — build-as-a-service with sorted-key caching

## Configuration (`velox.toml`)

```toml
[roadrunner]
ref = "v3.0.0"  # tag, branch, or 40-char commit SHA

[github]
# Optional. Set for GitHub Enterprise.
# base_url = "https://ghe.example.com"

[github.token]
token = "${GITHUB_TOKEN}"

[target_platform]
os = "linux"   # defaults to runtime.GOOS; "windows" is rejected
arch = "amd64" # defaults to runtime.GOARCH

[log]
level = "debug"
mode = "production"  # production | development | raw | none

[plugins.http]
module_name = "github.com/roadrunner-server/http/v6"
tag = "v6.0.0"

# Optional: go.mod replace directives. `new` listed first; embed @version inline.
[[replaces]]
new = "../local-fork"
old = "github.com/foo/bar"

[[replaces]]
new = "github.com/me/bar-fork@v1.2.3-patched"
old = "github.com/foo/bar@v1.2.3"

# Optional: go.mod exclude directives.
[[excludes]]
module = "github.com/redis/go-redis/v9"
version = "v9.15.0"
```

`Config.Validate()` expands `${ENV}` in the GitHub token, defaults `base_url` to `https://github.com`, defaults target platform to host, and rejects `windows`.

## Protocol buffers

- `buf.yaml` pulls `buf.build/bufbuild/protovalidate`.
- `buf.gen.yaml` produces Go (protobuf + Connect + gRPC stubs) into `gen/go`.
- After editing `.proto` files: `make regenerate`.
- `BuildRequest` now carries `repeated Replace replaces`, `repeated Exclude excludes`, `bool race`, `bool debug`.

## Testing

```bash
make test                    # race-enabled
go test -v ./builder/        # builder package only
go test -cover ./...         # coverage
```

CI (`.github/workflows/linux.yml`):

- Job `golang`: `make test`.
- Job `build-sample-rr`: installs `vx`, builds RoadRunner from `velox.toml`, runs `./rr --version`.

## Plugin compatibility

- **Do not use `master` branch** for plugins.
- **All plugins must share a major version** (e.g., http/v6 + logger/v6, never http/v6 + logger/v5).
- **`tag = "latest"`** is permitted but skips post-tidy version verification — pin tags for reproducible builds.

## Implementation notes

### Reproducible builds

- `-trimpath` is always set.
- `SOURCE_DATE_EPOCH` is honored for the `meta.buildTime` ldflag injection.
- Plugin prefixes are deterministic, so `plugins.go` is bit-identical across builds with the same plugin set.
- Remaining non-determinism: `go mod tidy` resolution for unpinned (`latest`) plugins — pin tags for fully reproducible builds.

### Cross-platform builds

- Per-platform `GOPATH`: `~/go/{goos}/{goarch}` keeps module caches separate.
- `GOPROXY` / `GOPRIVATE` / `GOFLAGS` are inherited from the calling process (don't override unless you know why).
- Smoke test is skipped when target platform != host platform.

### GitHub Enterprise

- `[github] base_url` switches the archive download host. GHE archive paths follow the same `/{owner}/{repo}/archive/...` shape under the GHE base.
- `Authorization: token …` is sent identically to github.com.

## Links

- [RoadRunner docs](https://docs.roadrunner.dev/customization/build)
- [Project repository](https://github.com/roadrunner-server/velox)
- [Discord community](https://discord.gg/TFeEmCs)
