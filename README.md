<p align="center">
 <a href="https://roadrunner.dev" target="_blank">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://github.com/roadrunner-server/.github/assets/8040338/cf1bfcf2-b787-426d-80f5-2862bb2a39b2">
    <img align="center" src="https://github.com/roadrunner-server/.github/assets/8040338/c4b971fd-b84f-406d-b850-0a4f072a5885">
  </picture>
</a>
</p>
<p align="center">
 <a href="https://pkg.go.dev/github.com/roadrunner-server/velox/v3?tab=doc"><img src="https://godoc.org/github.com/roadrunner-server/velox/v3?status.svg"></a>
 <a href="https://github.com/roadrunner-server/velox/actions"><img src="https://github.com/roadrunner-server/velox/workflows/Linters/badge.svg" alt=""></a>
 <a href="https://github.com/roadrunner-server/velox/actions"><img src="https://github.com/roadrunner-server/velox/workflows/Linux/badge.svg" alt=""></a>
 <a href="https://twitter.com/spiralphp"><img src="https://img.shields.io/twitter/follow/spiralphp?style=social"></a>
 <a href="https://discord.gg/TFeEmCs"><img src="https://img.shields.io/badge/discord-chat-magenta.svg"></a>
</p>

# Velox

Velox builds a custom RoadRunner binary from the plugin list in `velox.toml`. It downloads the RoadRunner source for the requested ref, renders `container/plugins.go`, applies the requested plugin versions plus any `replace` and `exclude` directives through `go mod edit`, runs `go mod tidy`, and compiles the binary with `go build`.

Documentation: [docs.roadrunner.dev/customization/build](https://docs.roadrunner.dev/customization/build).

## Installation

```bash
go install github.com/roadrunner-server/velox/v3/cmd/vx@v3.0.0-beta.1
```

Install the exact tag. The `/v3` module line has only pre-release tags for now, so `@latest` has no stable release to resolve to.

Migration: releases before v3 install from `github.com/roadrunner-server/velox/v2025/cmd/vx`.

### Docker

```bash
docker pull ghcr.io/roadrunner-server/velox:3.0.0-beta.1
```

Images are published to `ghcr.io/roadrunner-server/velox` and `spiralscout/velox`; the image tag is the release tag without the leading `v`. The entrypoint is `vx`, and the image ships the sample config at `/etc/velox.toml`.

```bash
docker run --rm \
  -e GITHUB_TOKEN \
  -v "$PWD/velox.toml:/etc/velox.toml" \
  -v "$PWD:/output" \
  ghcr.io/roadrunner-server/velox:3.0.0-beta.1 build -c /etc/velox.toml -o /output
```

The `docker-compose.yml` in this repository builds the image from source and runs `build -c=/etc/velox.toml -o=/tmp/`; uncomment the volume mapping there to copy the produced binary to the host.

## Usage

```bash
vx build -c velox.toml -o .
```

- `-c`, `--config`: path to the configuration file, default `velox.toml`.
- `-o`, `--out`: output directory for the produced `rr` binary, default the current directory.
- `--version`: velox version and build time.

Both flags are persistent, so `vx -c velox.toml build -o .` works as well. The build downloads the RoadRunner source into a temporary directory that is removed when the command returns.

## Minimal `velox.toml`

```toml
[roadrunner]
ref = "master"

[github.token]
token = "${GITHUB_TOKEN}"

[log]
level = "debug"
mode = "production"

[plugins.logger]
tag = "latest"
module_name = "github.com/roadrunner-server/logger/v6"

[plugins.server]
tag = "latest"
module_name = "github.com/roadrunner-server/server/v6"

[plugins.rpc]
tag = "latest"
module_name = "github.com/roadrunner-server/rpc/v6"

[plugins.http]
tag = "latest"
module_name = "github.com/roadrunner-server/http/v6"
```

The `velox.toml` shipped in this repository is the full sample: it lists every plugin that RoadRunner `master` registers.

## Configuration reference

### `[roadrunner]`

- `ref`: tag, branch, or 40-character commit SHA of `roadrunner-server/roadrunner`. Default `master`. A valid semver value (`v2025.1.7`) downloads `refs/tags`, a 40-character hex value downloads that commit, anything else downloads `refs/heads`.

### `[github]` and `[github.token]`

- `base_url`: GitHub host, default `https://github.com`. Set it to a GitHub Enterprise host (`https://ghe.example.com`) to download the source from a private installation; the archive paths and the `Authorization` header are the same as on github.com.
- `token`: personal access token used for the download. `${ENV_VAR}` is expanded from the environment.

### `[target_platform]`

- `os`: target `GOOS`, default the host `GOOS`. `windows` is rejected.
- `arch`: target `GOARCH`, default the host `GOARCH`.

A cross build gets its own `GOPATH` and `GOCACHE` under `~/go/<os>/<arch>` and skips the `rr --version` smoke test. A native build reuses the caller module cache.

### `[log]`

- `level`: `debug`, `info`, `warn`, or `error`.
- `mode`: `production`, `development`, `raw`, or `none` (`off` is an alias). Default when the section is absent: level `debug`, mode `development`.

### `[debug]`

- `enabled`: compiles with `-gcflags "all=-N -l" -tags debug` and keeps the symbol table for a debugger.

### `[plugins.<name>]`

- `module_name`: full Go module path of the plugin, including the major-version suffix.
- `tag`: version to require, or `latest`.

`informer` and `resetter` are compiled in from the versions the downloaded RoadRunner requires. Listing either of them here is skipped with a warning, because a user entry would register the plugin twice.

### `[[replaces]]`

Maps to a go.mod `replace` directive applied before `go mod tidy`.

```toml
[[replaces]]
new = "../local-fork"
old = "github.com/foo/bar"

[[replaces]]
new = "github.com/me/bar-fork@v1.2.3-patched"
old = "github.com/foo/bar@v1.2.3"
```

- Both fields are single strings; embed the version inline with `@`.
- A local path in `new` (`./`, `../`, or absolute) must not carry an `@version`. An `@` inside a directory name is fine: only a trailing `@<semver>` counts as a version.
- Neither field may contain `=`.
- `old` must be unique across the list.

### `[[excludes]]`

Maps to a go.mod `exclude` directive applied before `go mod tidy`.

```toml
[[excludes]]
module = "github.com/redis/go-redis/v9"
version = "v9.15.0"
```

## Plugin compatibility

- Supported: RoadRunner `master` (the `v2025` module line) with the `/v6` beta plugins. RoadRunner `v2025.x` releases pair with the `/v5` plugins.
- All plugins in one build must share the same major version: `http/v6` with `logger/v6`, never `http/v6` with `logger/v5`.
- Do not point plugins at a `master` branch.

## Notes

- Windows build targets are not supported in velox v3.
- Pin plugin tags for reproducible builds. `tag = "latest"` is allowed but skips the post-tidy version check, so the resolved version depends on when the build runs.
- `-trimpath` is always set, and `SOURCE_DATE_EPOCH` is honored for the injected build timestamp.

## Links

- [RoadRunner build documentation](https://docs.roadrunner.dev/customization/build)
- [Discord community](https://discord.gg/TFeEmCs)
