# Velox - RoadRunner Build System

This file gives guidance to Claude Code (claude.ai/code) when it works with code in this repository.

## Project Overview

Velox builds a custom RoadRunner binary from the plugin list in `velox.toml`. It edits the downloaded RoadRunner `go.mod` through `go mod edit`, supports `[[replaces]]` and `[[excludes]]` directives, and gives plugins deterministic prefixes for reproducible artifacts.

**Pipeline:**

1. Download the RoadRunner source archive from GitHub (tag, branch, or 40-char SHA) into a temp directory.
2. Keep the upstream `go.mod` as-is: it already pins informer, resetter, and the core dependencies.
3. Read the upstream `go.mod` with `go mod edit -json` before any edit: module path, informer module, resetter module.
4. Render `container/plugins.go` from the user plugin set with one parameterized template. `NewBuilder` drops bundled informer and resetter entries and sorts the rest by module path. The template uses the informer and resetter paths from step 3.
5. Apply the user `require`, `replace`, and `exclude` directives in one `go mod edit` call.
6. Run `go mod tidy -e`. Check that each plugin pinned to a semver tag resolved to that tag, or fail with an actionable error.
7. Run `go build` with `-o <output>/.rr-build-*/rr`, in a new temporary directory inside the output directory. Pass `-trimpath` and one `-ldflags` value that sets `version` and `buildTime` in `<upstream module>/internal/meta`. Add `-s -w` for a normal build, debug flags when `[debug] enabled` is set, and `-race` when `[debug] race` is set.
8. When the host GOOS and GOARCH match the target, run `rr --version` on the new binary with a 5-second timeout. The output must contain the requested ref, without a leading `v` before a digit. Then move the binary to `<output>/rr` and delete the temporary directory.

**Interface:** CLI only (`vx build`), driven by `velox.toml`.

**Key technologies:**

- Go 1.27 (module path: `github.com/roadrunner-server/velox/v3`)
- `golang.org/x/mod` for semver checks, module path parsing, and the `[[replaces]]` and `[[excludes]]` validation
- `encoding/json/v2` + `encoding/json/jsontext` for go tool JSON output
- `log/slog` (stdlib) for structured logging, no third-party logger
- Cobra CLI + Viper config

**Not supported in v3:** Windows targets. Releases ship `vx` for linux and darwin only.

## Release lines

- `master` is velox v3, module `github.com/roadrunner-server/velox/v3`. It builds RoadRunner v3 (module `/v3`) with the `/v6` plugins.
- `stable` is velox v2025, module `github.com/roadrunner-server/velox/v2025`. It builds RoadRunner `v2025.x` with the `/v5` plugins. The `v2025.1.x` tags are on `stable`, not on `master`.
- `pre-release/**` branches get the same CI as `master`.
- The builder reads the RoadRunner module path from the downloaded `go.mod`, so the code has no RoadRunner major built in. Unit tests use `/v2025` module paths as fixtures.

## Repository structure

Each package has a `doc.go` and `*_test.go` files next to the code. The repository root is package `velox`.

```text
├── .claude/CLAUDE.md           # Instructions for Claude Code
├── .github/workflows/          # linux.yml (tests, sample build), linters.yml (golangci-lint), release.yml (archives, images), release-please.yml (release PR), codeql-analysis.yml
├── .golangci.yml               # golangci-lint v2 configuration
├── builder/
│   ├── builder.go              # Build pipeline (decomposed into named steps)
│   ├── gomod.go                # runGo subprocess runner + `go mod edit/tidy` drivers
│   ├── upstream.go             # Upstream go.mod introspection, ldflags, major version
│   ├── options.go              # Functional options
│   ├── testdata/               # go.mod fixtures for the upstream tests
│   └── templates/
│       └── plugins_template.go # Single parameterized plugins.go template
├── cmd/vx/                     # Main CLI entry point
├── config.go                   # Config, Replace, Exclude, validation
├── github/
│   └── github.go               # Archive download + extraction
├── internal/cli/               # Root cobra command: -c/-o flags, Viper config load, validation, logger setup
│   └── build/                  # `vx build`
├── internal/version/           # velox version + build time, injected via ldflags
├── plugin/                     # Plugin metadata + deterministic prefix
├── logger/                     # slog logger builder: production (JSON), development (default), raw, none/off
├── Dockerfile                  # vx image
├── Dockerfile_sample           # User example: build rr with the vx image
├── docker-compose.yml          # Builds the vx image and runs a sample build
├── Makefile                    # make test
├── README.md                   # User reference
├── CHANGELOG.md
└── velox.toml                  # Sample configuration
```

## Common commands

```bash
make test          # go test -v -race ./...

go build -o vx ./cmd/vx
./vx build -c velox.toml -o ./output

go test -v ./builder/        # builder package only
go test -cover ./...         # coverage
golangci-lint run --timeout=10m --build-tags=race
```

Use golangci-lint v2, as the CI lint job does.

## Core architecture

### Build pipeline (`builder/builder.go:Build`)

```text
validateInputs -> ResolvePrefixCollisions -> upstreamModule -> writePluginsGo
  -> applyGoModEdits -> goModTidy -> verifyResolvedVersions
  -> compile -> smokeTest -> publish
```

`ResolvePrefixCollisions` lives in the `plugin` package (it operates on the plugin slice, not on the Builder). Every other step is a method on `*Builder`. Every step that runs the go tool takes a `context.Context` and surfaces the last 8 KB of stderr in the returned error; `validateInputs`, `writePluginsGo`, and `publish` touch the filesystem only, and `smokeTest` runs the built binary and reports its combined output in the returned error.

`NewBuilder` runs `userPlugins`, which drops the bundled informer/resetter entries and sorts the rest by module path, so every step sees the same set. `upstreamModule` must run before `applyGoModEdits`, while the upstream `go.mod` is still pristine. The `upstreamModule` result is passed explicitly into `writePluginsGo` and `compile`. `Build` creates a temporary directory `.rr-build-*` in the output directory and compiles to `rr` in that directory. It smoke-tests the binary there, and `publish` renames it to `rr` in the output directory. A deferred `os.RemoveAll` deletes the temporary directory when `Build` returns, on success and on failure.

### Upstream introspection (`builder/upstream.go`)

`go mod edit -json` in the RR source tree yields the module path plus the require list. Informer and resetter are the direct requires whose path is `github.com/roadrunner-server/informer` or `.../resetter`, or is nested under that path (for example `/v6`). Indirect requires are ignored. If either module is missing, `upstreamModule` returns an error. `ldflags` builds `-X <module>/internal/meta.version=... -X <module>/internal/meta.buildTime=...` from the discovered module path, so version injection tracks whatever major the downloaded ref uses. `majorVersion` reports the path major (`v1` when absent) for the log line.

### Version verification (`builder/builder.go:verifyResolvedVersions`)

One batched `go list -m -e -json <modules...>` over the plugins pinned to a semver tag, decoded as a stream. A tag that is not a semver version (`latest`, a branch, a commit) is skipped, and an empty operand list returns early (bare `go list -m` would describe the main module). A module replaced by itself is compared against `Replace.Version`; a module replaced by a different module or a local directory is skipped. `-e` exits 0 and reports per-module failures inside the payload, so `Error` is decoded too.

### Plugin prefixing (`plugin/plugin.go`)

Every plugin gets a deterministic 5-letter lowercase prefix. The prefix comes from sha256 of the module path followed by a 2-byte big-endian salt, starting at salt 0. Each letter is 'a' plus one hash byte modulo 26. A candidate that is a Go keyword is re-salted like a collision. Collisions across a single build are resolved by `ResolvePrefixCollisions`, which re-salts conflicting prefixes. `userPlugins` sorts the set by module path, so two builds with the same plugin set produce bit-identical `plugins.go`.

### Subprocess execution (`builder/gomod.go:runGo`)

`runGo` wraps `exec.CommandContext` with: `cmd.Cancel` sending SIGINT and `cmd.WaitDelay` of 15 s before SIGKILL, full stdout capture, bounded ring-buffer stderr capture (last 8 KB), and a stderr tee to the debug logger. A clean exit code outranks `exec.ErrWaitDelay`, because `go build` grandchildren inherit the stderr pipe and can hold it open past their parent exit. On cancellation the returned error joins `ctx.Err()` with the stderr tail. `runGo` is a `Builder` method: it runs in the RR source dir with the environment computed once in `NewBuilder`. `applyGoModEdits` passes every `-require`, `-replace`, and `-exclude` operand to one `go mod edit` call, because the go tool writes a non-semver version such as `latest` as given and a later `go mod edit` call cannot parse it.

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

[debug]
enabled = false  # true: -v -gcflags "all=-N -l" -tags debug, and no -s -w
race = false     # true: -race and CGO_ENABLED=1

[github]
# Optional. Set for GitHub Enterprise.
# base_url = "https://ghe.example.com"

[github.token]
token = "${GITHUB_TOKEN}"

[target_platform]
os = "linux"   # defaults to runtime.GOOS; "windows" is rejected
arch = "amd64" # defaults to runtime.GOARCH

[log]
level = "debug"      # debug | info | warn | error; empty: info in production and raw, debug in development
mode = "production"  # production | development | raw | none (alias: off); an unknown mode falls back to development

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

Without `race`, `vx` sets `CGO_ENABLED=0` and overrides the value from the environment. The `[log]` table configures the `vx` logger, not the built RoadRunner binary.

`Config.Validate()` expands `${ENV}` in the GitHub token. It sets the ref to `master` when the `ref` key is absent, and it rejects an empty ref or a ref with characters outside `a-z A-Z 0-9 . _ / + -`. It fills a missing target `os` or `arch` from the host and rejects `windows` in any letter case. It sets `base_url` to `https://github.com` when it is empty. It requires at least one plugin, a `module_name` and a `tag` for each plugin, and a different `module_name` for each plugin. It requires `new` and `old` in each `[[replaces]]` entry, rejects `=` in them, rejects `@version` on a local `new` path, and rejects a repeated `old`. It resolves a relative local `new` against the working directory. It requires `module` and a canonical semver `version` in each `[[excludes]]` entry, with a major version that matches the module path. It sets the `[log]` default to level debug and mode development.

The sample `velox.toml` tracks RoadRunner `master`: every plugin is on the `/v6` line with `tag = "latest"`. The release archive and the image (`/etc/velox.toml`) ship it, and CI builds RoadRunner `master` from it. Keep it buildable against RoadRunner `master`.

## CI

- `linux.yml` runs on push and pull request to `master` and `pre-release/**`, and daily at 05:30 UTC. Job `golang` runs `make test`. Job `build-sample-rr` installs `vx`, builds RoadRunner `master` from `velox.toml` with `latest` plugins, and runs `./rr --version`. A failure there can come from a RoadRunner or plugin change, not from velox.
- `linters.yml` runs on every push and pull request with the latest stable Go.
- `codeql-analysis.yml` runs on `master`, `stable`, and `pre-release/**`, and weekly.
- Unit tests run the go tool against a local `file://` GOPROXY and `httptest` servers. They need `go` on `PATH` but no network. No unit test runs a full RoadRunner build.

## Lint

- `.golangci.yml` sets `default: none` and enables a fixed list. The list includes `gochecknoglobals`, `gochecknoinits`, `gosec`, `noctx`, `errorlint`, `exhaustive`, `prealloc`, and `revive`.
- `nolintlint` requires a linter name in each directive, for example `//nolint:gochecknoglobals // <reason>`.
- The formatters are `gofmt` and `goimports`.

## Releases and Docker

- Release Please (`.github/workflows/release-please.yml`) keeps a release PR on `master` from Conventional Commits and writes `CHANGELOG.md` and `.release-please-manifest.json`. Merging the release PR creates the tag and the GitHub release with the GitHub App token, which starts `release.yml`. The GitHub release notes come from the release PR description, not from `CHANGELOG.md`. A push to `master` regenerates an open release PR and drops manual edits to it.
- `.github/workflows/release.yml` runs when a GitHub release is published. It runs no tests. The version is the tag without the leading `v`.
- Job `build` compiles `vx` for linux and darwin on amd64 and arm64 with `CGO_ENABLED=0`, `-trimpath`, and the version ldflags for `github.com/roadrunner-server/velox/v3/internal/version`. Each release asset is `velox-<version>-<os>-<arch>.tar.gz` with `vx`, `README.md`, `LICENSE`, and `velox.toml`.
- Job `docker` builds `Dockerfile` for linux/amd64 and linux/arm64 and pushes `spiralscout/velox:<version>` and `ghcr.io/roadrunner-server/velox:<version>`. A release that is not a prerelease also moves `:latest`, the major tag (`:3`), and the minor tag (`:3.0`) on both registries.
- A release runs the `release.yml` of the tagged commit. The `stable` copy moves `:latest` on every release, so a v2025 release moves `:latest` to velox v2025.
- `release.yml` and `Dockerfile` hard-code the package path `github.com/roadrunner-server/velox/v3/internal/version`. Change both when the module path changes.
- `Dockerfile` builds `vx` on `golang:1.27-alpine`. The final image is also `golang:1.27-alpine`, because `vx` runs the go tool. It adds `gcc` and `musl-dev` for `[debug] race = true`. A Go version change touches `go.mod` (`go` and `toolchain`) and both `FROM` lines in `Dockerfile`.
- `Dockerfile_sample` and `README.md` pin the image tag `3.0.0`. Update the pins after a release.

## User documentation

- `README.md` is the user reference: installation, Docker use, CLI flags, and every `velox.toml` key. Update it when a config key, a flag, or build behavior changes.
- The RoadRunner guide is `customization/build.md` in `https://github.com/roadrunner-server/docs`, branch `release/v3`, published at `https://docs.roadrunner.dev/customization/build`. A user-facing change needs a separate PR there.

## Plugin compatibility

- **Do not use `master` branch** for plugins.
- **All plugins must share a major version** (e.g., http/v6 + logger/v6, never http/v6 + logger/v5). RR v3 (module path `/v3`) pairs with `/v6`; RR `v2025.x.x` releases pair with `/v5` and velox `v2025`.
- **A tag that is not a semver version** (`latest`, a branch, a commit) skips post-tidy version verification: pin semver tags for reproducible builds.
- informer and resetter are bundled from the upstream `go.mod`. Entries for them in `velox.toml` are dropped with a warning to avoid a double registration.

## Implementation notes

### Reproducible builds

- `-trimpath` is always set.
- `SOURCE_DATE_EPOCH` is honored for the `meta.buildTime` ldflag injection.
- Plugins are sorted by module path and prefixes are deterministic, so `plugins.go` is bit-identical across builds with the same plugin set and the same RoadRunner ref.
- Remaining non-determinism: a plugin tag that is `latest` or a branch, a RoadRunner `ref` that is a branch (the default is `master`), and the build time when `SOURCE_DATE_EPOCH` is not set. For fully reproducible builds, pin semver tags or commit SHAs and set `SOURCE_DATE_EPOCH`.

### Cross-platform builds

- The build environment is computed once in `NewBuilder` (`newEnv`) and reused by every `go` subprocess (`runGo`). The smoke test runs with the parent environment.
- Every build reuses the caller `GOPATH`, `GOMODCACHE`, and `GOCACHE`; Go keys build cache entries by GOOS/GOARCH.
- `CGO_ENABLED` is 1 with `[debug] race = true` (`-race`) and 0 otherwise.
- `GOPROXY` / `GOPRIVATE` / `GOFLAGS` are inherited from the calling process (don't override unless you know why).
- The smoke test is skipped when the target platform differs from the host.

### GitHub Enterprise

- `[github] base_url` switches the archive download host. GHE archive paths follow the same `/{owner}/{repo}/archive/...` shape under the GHE base. Only the base URL is configurable. The owner and repository are fixed, so the mirror must live at `<base_url>/roadrunner-server/roadrunner`.
- The token goes into the bearer `Authorization` header of the archive request, the same as for GitHub.com. On a redirect, net/http keeps the header when the target is the same domain or a subdomain, for example github.com to codeload.github.com. It drops the header for any other domain.

## Links

- [RoadRunner docs](https://docs.roadrunner.dev/customization/build)
- [Project repository](https://github.com/roadrunner-server/velox)
- [Discord community](https://discord.gg/TFeEmCs)
