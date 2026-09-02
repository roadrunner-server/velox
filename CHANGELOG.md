# Changelog

## v3.0.0-beta.1

### Breaking

- Module path moved to `github.com/roadrunner-server/velox/v3`; the CLI installs from `github.com/roadrunner-server/velox/v3/cmd/vx`.
- The Connect/gRPC build server is removed. Velox is a CLI only (`vx build`), driven by `velox.toml`.
- Windows build targets are rejected: `target_platform.os = "windows"` fails configuration validation, and no Windows binary is released.

### Added

- `[[replaces]]` and `[[excludes]]` sections map to go.mod `replace` and `exclude` directives, applied before `go mod tidy`. A relative local path in `[[replaces]].new` resolves against the working directory.
- Deterministic 5-letter plugin import prefixes and module-path ordering, so the same plugin set renders a bit-identical `container/plugins.go`.
- `[debug] race = true` builds with `-race`.

### Changed

- The go.mod of the downloaded RoadRunner source is kept as-is and edited through `go mod edit` plus `go mod tidy`, in place of a templated go.mod. All require, replace, and exclude directives go into one `go mod edit` call.
- The bundled informer and resetter module paths come from `go mod edit -json` on the upstream go.mod, in place of regexp scraping. A `velox.toml` entry for either of them is dropped before every pipeline step.
- Post-tidy version verification runs as a single batched `go list -m -e -json` over the plugins pinned to a semver tag, and understands `replace` directives. A tag that is not a semver version (`latest`, a branch, a commit) is not checked.
- Subprocess cancellation uses `cmd.Cancel` and `cmd.WaitDelay`: the go tool gets SIGINT and 15 seconds before SIGKILL.
- The binary is compiled straight into the output directory as `rr.tmp` and renamed to `rr` after the smoke test, which now fails when `rr --version` does not print the requested ref.
- Every build reuses the caller `GOPATH`, `GOMODCACHE`, and `GOCACHE`; the per-target redirect to `~/go/<os>/<arch>` is gone.
- The in-process archive cache is removed; a `vx build` process downloads its ref once.
- The archive download lets net/http follow redirects and accepts a direct 200. The GitHub token is a bearer `Authorization` header on the request, dropped on a redirect to another host, so the `oauth2` dependency is gone.
- `builder.Build` takes only a context; the ref comes from `WithRRVersion`.
- `go build -v` is passed only for a debug build.
- The release workflow packs both darwin assets as `.tar.gz`.

### Fixed

- Version ldflags derive the meta package path from the module path in the downloaded RoadRunner go.mod. The hardcoded path matched no buildable ref, so version injection was inert and `rr --version` reported `local`.
- `[[replaces]]` or `[[excludes]]` together with a `latest` plugin tag aborted the build: `go mod edit` writes `latest` as given, and a second `go mod edit` call could not parse it.
- An output directory on a different filesystem than the temp download directory (for example a bind mount in Docker, or a tmpfs `/tmp`) failed at the final rename.
- A relative `-o` path such as `./` broke the smoke test.
- A plugin tag that is a branch or a commit always failed the version check against the resolved pseudo-version.
- `[target_platform]` with only `os` or only `arch` left the other key empty.
- Two `[plugins.*]` entries naming the same module were accepted; an `[[excludes]]` version is now validated as canonical semver at configuration time.
- A generated import prefix could be a Go keyword.
- A GitHub Enterprise host or proxy that served the archive with 200 instead of a redirect failed the download.
- The sample `velox.toml` lists `tcp/v6` again.
