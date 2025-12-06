# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

# Velox - RoadRunner Build System

## Project Overview

Velox is an automated build system for RoadRunner server and its plugins. It dynamically compiles custom RoadRunner binaries by:
1. Downloading RoadRunner template from GitHub
2. Generating `container/plugins.go` with plugin imports and registration
3. Generating `go.mod` with plugin dependencies
4. Running `go build` with cross-compilation support

**Two modes:**
- **CLI (`vx`)**: Local builds with custom plugin selection
- **Build Server**: gRPC/Connect API for remote builds with LRU caching

**Key Technologies:**

- Go 1.25+ (module: `github.com/roadrunner-server/velox/v2025`)
- Protocol Buffers with buf (not protoc)
- Connect RPC (connectrpc.com/connect) and gRPC
- GitHub API with OAuth2 token support
- Cobra CLI framework

## Repository Structure

```
├── api/                    # Protocol buffer definitions (BuildService RPC)
├── builder/               # Core build logic
│   └── templates/        # Versioned templates (V2025, V2024, V2023, V2)
├── cmd/vx/               # Main CLI entry point
├── gen/                  # Generated protobuf code (buf generate output)
├── github/               # GitHub API integration (template downloads)
├── internal/cli/         # CLI command implementations
│   ├── build/           # Build command
│   └── server/          # gRPC server with LRU cache
├── plugin/               # Plugin metadata with import collision avoidance
├── cache/                # Thread-safe RRCache for downloaded templates
├── logger/               # Zap logger builder
└── velox.toml           # Configuration file
```

**Note:** `gen/` directory has replace directive in `go.mod:28`: `github.com/roadrunner-server/velox/v2025/gen => ./gen`

## Common Commands

### Building and Testing

```bash
# Run tests with race detection
make test                # Runs: go test -v -race ./...

# Regenerate protobuf code (after .proto changes)
make regenerate          # Runs: rm -rf ./gen && buf generate && buf format -w

# Build the velox CLI
go build -o vx ./cmd/vx

# Use velox to build custom RoadRunner
./vx -c velox.toml build -o ./output/rr

# Run velox as build server
./vx -c velox.toml server -a 127.0.0.1:8080

# Test specific packages
go test -v ./builder/
go test -v ./github/
go test -cover ./...

# Linting (35+ linters configured in .golangci.yml)
golangci-lint run
```

## Core Architecture

### Build Process Flow (builder/builder.go:61-203)

1. **Download Template**: GitHub API downloads RoadRunner template (cached by version in `cache/cache.go`)
   - Supports: tags (`v2025.1.2`), branches (`master`), commit SHAs (40-char)
   - URL patterns: `/archive/refs/tags/*.zip`, `/archive/refs/heads/*.zip`, `/archive/{sha}.zip`
   - CWE-22 protection: Rejects paths with `..` in zip extraction

2. **Generate Plugin Registration**: `builder/templates/compile.go` compiles `container/plugins.go`
   - Random 5-letter prefix per plugin to avoid import collisions (`plugin/plugin.go:13-59`)
   - Injects imports, requires, and plugin initialization code

3. **Generate go.mod**: Template compilation creates module file with plugin dependencies
   - Version detection: "master" → v2025, semantic version → extract major version

4. **Build Binary**: Executes `go mod download && go mod tidy && go build`
   - Cross-compilation: Sets GOOS/GOARCH/CGO_ENABLED=0 from config
   - Custom GOPATH per platform: `~/go/{goos}/{goarch}`
   - Ldflags inject version info from `internal/version/version.go`

### Template Versioning (builder/builder.go:77-90, :137-150)

- **V2025**: Current production template (`builder/templates/templateV2025.go`)
- **V2024**: Backward compatibility (`builder/templates/templateV2024.go`)
- **V2023**, **V2**: Legacy templates

**Adding new templates:**
1. Create `builder/templates/templateVXXXX.go` with `goModTemplate` and `pluginTemplate`
2. Update switch cases in `builder.go` (lines 77-90 for module, 137-150 for plugins)
3. Add constant to `config.go:15-16` (e.g., `V2026 = "v2026"`)

### Server Mode Caching (internal/cli/server/server.go:33-177)

- **Binary Cache**: LRU (100 entries, 30min TTL) stores built binaries
- **Processing Lock**: LRU (100 entries, 5min TTL) prevents duplicate concurrent builds
- **Cache Key**: FNV hash of protobuf-marshaled BuildRequest
- **Eviction**: Removes binary file + temp directory on eviction

### Key Files

- `builder/builder.go:32-319` - Main Builder with template compilation and go build
- `builder/options.go:10-66` - Functional options (WithPlugins, WithGOOS, WithGOARCH, etc.)
- `internal/cli/server/server.go` - Build-as-a-service with gRPC/Connect
- `github/github.go:36-290` - GitHub template downloads with OAuth2 support
- `config.go:19-98` - Config validation with environment variable expansion
- `api/service/v1/service.proto:10-12` - BuildService RPC definition

## Configuration (velox.toml)

```toml
[roadrunner]
ref = "v2025.1.2"  # Tag, branch, or 40-char commit SHA

[github.token]
token = "${GITHUB_TOKEN}"  # Environment variable expansion

[target_platform]
os = "linux"       # Defaults to runtime.GOOS
arch = "amd64"     # Defaults to runtime.GOARCH

[log]
level = "debug"
mode = "production"  # Options: production, development, raw, none

[plugins.http]
module_name = "github.com/roadrunner-server/http/v5"
tag = "v5.1.0"  # Must match major version with other plugins (v5.x.x)
```

**Config validation** (`config.go:56-98`): Expands env vars, validates required fields, checks plugin version compatibility.

## Protocol Buffers

- **buf.yaml**: Dependencies include `buf.build/bufbuild/protovalidate`
- **buf.gen.yaml**: Generates Go (protobuf + Connect + gRPC stubs)
- **Regeneration**: `make regenerate` or `rm -rf ./gen && buf generate && buf format -w`
- **Files always modified**: `api/service/v1/`, `api/request/v1/`, `api/response/v1/`

## Testing

**Unit tests:**
```bash
make test                    # Race detection enabled
go test -v ./builder/        # Specific package
go test -cover ./...         # With coverage
```

**CI/CD** (`.github/workflows/linux.yml`):
- Runs on push, PR, daily at 5:30 AM UTC
- Job 1 (`golang`): `make test`
- Job 2 (`build-sample-rr`): Integration test - builds velox, uses it to build RoadRunner, runs `./rr --version`

## Plugin Compatibility (CRITICAL)

⚠️ **Plugin version rules:**
- **Never use `master` branch** for plugins
- **All plugins must use same major version** (e.g., `logger` v5.0.3 + `amqp` v5.0.5 ✓, but `logger` v6.0.0 + `amqp` v5.0.5 ✗)
- **Currently supported:** Plugins v5.x.x, RoadRunner >=v2024.x.x
- Mixing major versions will cause build failures

**Version detection logic** (`builder/builder.go:214-227`): "master" → v2025, semantic version → extract major version

## Setup

```bash
# Prerequisites: Go 1.25+, buf CLI
go mod download
make regenerate     # Generate protobuf code
make test           # Verify setup
go build -o vx ./cmd/vx
```

## Important Implementation Details

### Module Versioning (Non-semantic)
- Module path uses **year-based versioning**: `github.com/roadrunner-server/velox/v2025`
- This is NOT semantic versioning (v2.x.x) - it's a calendar year indicator
- Replace directive required: `github.com/roadrunner-server/velox/v2025/gen => ./gen`

### GitHub Template Downloads
- **Caching**: Downloaded templates cached in `cache/cache.go` (thread-safe RRCache)
- **URL formats**:
  - Tags: `https://github.com/{owner}/{repo}/archive/refs/tags/{tag}.zip`
  - Branches: `https://github.com/{owner}/{repo}/archive/refs/heads/{branch}.zip`
  - Commits: `https://github.com/{owner}/{repo}/archive/{sha}.zip` (40-char SHA)
- **Security**: Path traversal check rejects `..` in zip entry names (`github/github.go:211-218`)

### Import Collision Avoidance
- Each plugin gets random 5-letter prefix (`plugin/plugin.go:13-59`)
- Generated code: `import prefix "github.com/roadrunner-server/http/v5"`
- Plugin registration: `prefix.Plugin{}`

### Cross-Platform Builds
- Custom GOPATH per platform: `~/go/{goos}/{goarch}` prevents module cache conflicts
- Environment: `GOOS={os} GOARCH={arch} CGO_ENABLED=0`
- Build command: `go build -trimpath -ldflags="-s -w -X version=..."`

## Links and Documentation

- [RoadRunner Docs](https://docs.roadrunner.dev/customization/build)
- [Project Repository](https://github.com/roadrunner-server/velox)
- [Discord Community](https://discord.gg/TFeEmCs)
