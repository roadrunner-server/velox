# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

# Velox - RoadRunner Build System

## Project Overview

Velox is an automated build system for RoadRunner server and its plugins. The v3 milestone (current) drives the build through `go mod edit` rather than a hand-written `go.mod` template, supports `[[replaces]]` and `[[excludes]]` directives, and ships with deterministic plugin prefixes for reproducible artifacts.

**Pipeline:**

1. Download the RoadRunner source archive from GitHub (tag, branch, or 40-char SHA). Archive bytes go through an in-process LRU (32 entries), so a single run refetches a repeated ref only once.
2. Preserve the upstream `go.mod` as-is: it already pins informer/resetter and the core deps.
3. Introspect the upstream `go.mod` with `go mod edit -json` before any edit: module path, informer module, resetter module.
4. Render `container/plugins.go` from a single parameterized template, using the informer/resetter paths read in step 3, so one template covers every RR major.
5. Apply user-supplied `require`, `replace`, and `exclude` directives via `go mod edit`.
6. Run `go mod tidy -e`. Verify each pinned user plugin resolved to the requested tag (else fail with an actionable error).
7. Run `go build` with `-trimpath`, version ldflags derived from the upstream module path, and (optionally) `-race` / debug flags.
8. Smoke-test the binary (`./rr --version`) when the host platform matches the target platform.

**Interface:** CLI only (`vx build`), driven by `velox.toml`.

**Key technologies:**

- Go 1.27 (module path: `github.com/roadrunner-server/velox/v3`)
- `golang.org/x/mod` for semver checks and module path parsing
- `hashicorp/golang-lru/v2` for caches
- `encoding/json/v2` + `encoding/json/jsontext` for go tool JSON output
- `log/slog` (stdlib) for structured logging, no third-party logger
- Cobra CLI + Viper config

**Not supported in v3:** Windows targets.

## Repository structure

```text
├── builder/
│   ├── builder.go              # Build pipeline (decomposed into named steps)
│   ├── gomod.go                # runGo subprocess runner + `go mod edit/tidy` drivers
│   ├── upstream.go             # Upstream go.mod introspection, ldflags, major version
│   ├── options.go              # Functional options
│   ├── gomod_test.go
│   ├── upstream_test.go
│   ├── testdata/               # go.mod fixtures for the upstream tests
│   └── templates/
│       ├── plugins_template.go # Single parameterized plugins.go template
│       └── template_test.go
├── cmd/vx/                     # Main CLI entry point
├── config.go                   # Config, Replace, Exclude, validation
├── github/
│   ├── github.go               # Archive download + extraction
│   └── cache.go                # LRU-backed Cache implementation
├── internal/cli/               # cobra wiring
│   └── build/                  # `vx build`
├── internal/version/           # velox version + build time, injected via ldflags
├── plugin/                     # Plugin metadata + deterministic prefix
├── logger/                     # slog logger builder (production / development / raw / off)
├── CHANGELOG.md
└── velox.toml                  # Sample configuration
```

## Common commands

```bash
make test          # go test -v -race ./...

go build -o vx ./cmd/vx
./vx -c velox.toml build -o ./output

go test -cover ./...
golangci-lint run
```

## Core architecture

### Build pipeline (`builder/builder.go:Build`)

```text
validateInputs -> ResolvePrefixCollisions -> upstreamModule -> writePluginsGo
  -> applyRequires -> applyReplaces -> applyExcludes
  -> goModTidy -> verifyResolvedVersions
  -> compile -> relocate -> smokeTest
```

`ResolvePrefixCollisions` lives in the `plugin` package (it operates on the plugin slice, not on the Builder). Every other step is a method on `*Builder`. Each step propagates `context.Context` and surfaces the last 8 KB of stderr in any returned error.

`upstreamModule` must run before `applyRequires`, while the upstream `go.mod` is still pristine. Its result is passed explicitly into `writePluginsGo` and `compile`.

### Upstream introspection (`builder/upstream.go`)

`go mod edit -json` in the RR source tree yields the module path plus the require list. Informer and resetter are selected by module-path prefix match among the direct requires only. `ldflags` builds `-X <module>/internal/meta.version=... -X <module>/internal/meta.buildTime=...` from the discovered module path, so version injection tracks whatever major the downloaded ref uses. `majorVersion` reports the path major (`v1` when absent) for the log line.

### Version verification (`builder/builder.go:verifyResolvedVersions`)

One batched `go list -m -e -json <modules...>` over the pinned plugins, decoded as a stream. Plugins tagged `latest` are skipped, and an empty operand list returns early (bare `go list -m` would describe the main module). A module replaced by itself is compared against `Replace.Version`; a module replaced by a different module or a local directory is skipped. `-e` exits 0 and reports per-module failures inside the payload, so `Error` is decoded too.

### Plugin prefixing (`plugin/plugin.go`)

Every plugin gets a deterministic 5-letter alpha-lowercase prefix derived from `sha256(moduleName)`. Collisions across a single build are resolved by `ResolvePrefixCollisions`, which re-salts conflicting prefixes. Two builds with the same plugin set produce bit-identical `plugins.go`.

### Subprocess execution (`builder/gomod.go:runGo`)

`runGo` wraps `exec.CommandContext` with: `cmd.Cancel` sending SIGINT and `cmd.WaitDelay` of 15 s before SIGKILL, full stdout capture, bounded ring-buffer stderr capture (last 8 KB), and a stderr tee to the debug logger. A clean exit code outranks `exec.ErrWaitDelay`, because `go build` grandchildren inherit the stderr pipe and can hold it open past their parent exit. On cancellation the returned error joins `ctx.Err()` with the stderr tail. `b.runGo` is the method wrapper that supplies the RR source dir and the builder environment.

### Key files

- `builder/builder.go` - pipeline orchestration
- `builder/upstream.go` - upstream `go.mod` introspection + ldflags
- `builder/gomod.go` - `runGo` + `go mod edit/tidy`
- `builder/templates/plugins_template.go` - sole template, rendered through `go/format`
- `config.go` - `Config`, `Replace`, `Exclude`, validation (incl. Windows rejection)
- `plugin/plugin.go` - deterministic prefix + collision resolver
- `github/github.go` - archive download (GHE-aware) + zip extraction with CWE-22 guard

## Configuration (`velox.toml`)

```toml
[roadrunner]
ref = "master"  # tag, branch, or 40-char commit SHA

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
tag = "latest"  # or pin to v6.x.x for reproducible builds

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

`Config.Validate()` expands `${ENV}` in the GitHub token, defaults the ref to `master` and validates its characters, defaults target platform to host, defaults `base_url` to `https://github.com`, and rejects `windows`.

The sample `velox.toml` tracks RoadRunner `master`: every plugin is on the `/v6` line with `tag = "latest"`.

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
- **All plugins must share a major version** (e.g., http/v6 + logger/v6, never http/v6 + logger/v5). RR `v2025.x.x` releases pair with `/v5`; the `/v6` beta plugin line pairs with RR `master`, whose module path stays year-based (`/v2025`).
- **`tag = "latest"`** is permitted but skips post-tidy version verification: pin tags for reproducible builds.
- informer and resetter are bundled from the upstream `go.mod`. Entries for them in `velox.toml` are dropped with a warning to avoid a double registration.

## Implementation notes

### Reproducible builds

- `-trimpath` is always set.
- `SOURCE_DATE_EPOCH` is honored for the `meta.buildTime` ldflag injection.
- Plugin prefixes are deterministic, so `plugins.go` is bit-identical across builds with the same plugin set.
- Remaining non-determinism: `go mod tidy` resolution for unpinned (`latest`) plugins. Pin tags for fully reproducible builds.

### Cross-platform builds

- The build environment is computed once in `NewBuilder` (`newEnv`) and reused by every subprocess.
- `GOPATH` and `GOCACHE` are redirected to `~/go/{goos}/{goarch}` only for a cross build; a native build keeps the caller caches.
- `CGO_ENABLED` is 1 with `-race` and 0 otherwise.
- `GOPROXY` / `GOPRIVATE` / `GOFLAGS` are inherited from the calling process (don't override unless you know why).
- The smoke test is skipped when the target platform differs from the host.

### GitHub Enterprise

- `[github] base_url` switches the archive download host. GHE archive paths follow the same `/{owner}/{repo}/archive/...` shape under the GHE base.
- The token is attached by an `oauth2` transport (bearer `Authorization` header), identically to GitHub.com.

## Links

- [RoadRunner docs](https://docs.roadrunner.dev/customization/build)
- [Project repository](https://github.com/roadrunner-server/velox)
- [Discord community](https://discord.gg/TFeEmCs)
