# Changelog

## 3.0.0 (2026-10-09)


### Features

* remove the Connect/gRPC build server ([c4123c1](https://github.com/roadrunner-server/velox/commit/c4123c1affe4b534e2289665f6093f5c7bb97c23))
* v3 beta ([6b71101](https://github.com/roadrunner-server/velox/commit/6b71101ce0080143b4927cf2d84ab0ba02189b67))
* v3 beta ([67911be](https://github.com/roadrunner-server/velox/commit/67911be8d5ee53c01685ed2e9508d312b2c64a8e))
* v3 modernization — replace directives, go-mod-edit driven build ([72ced78](https://github.com/roadrunner-server/velox/commit/72ced78ddbc0ba473c2cd39d47c8a025d2b99cf2))


### Bug Fixes

* **ci:** default sample to host platform; harden temp dir; drop uuid dep ([61f62e8](https://github.com/roadrunner-server/velox/commit/61f62e8ac7584cfa55c243fefc8e0935723ebfb6))
* isolate build outputs and support Docker race builds ([c7df00d](https://github.com/roadrunner-server/velox/commit/c7df00db7a661707b8006b0fe9eaaa31a82b2eed))
* **v3:** review-comment fixes, bump sample plugins to v6, swap zap for log/slog ([547e636](https://github.com/roadrunner-server/velox/commit/547e63636f29b06a68336fa215345e98fdb1cedf))
* **v3:** second round of PR review fixes ([272a06c](https://github.com/roadrunner-server/velox/commit/272a06cdc60a4c19909a009382bbeb6b147fa778))
* **v3:** third-round PR review fixes (oauth redirect, cache copy, nil guard) ([82a7c74](https://github.com/roadrunner-server/velox/commit/82a7c74a5b3ae6e4caa29e2d7b528ac440bdc646))

## v3.0.0

### Breaking

- Velox v3 builds RoadRunner v3 (the `/v3` module line) with the `/v6` plugins. Build RoadRunner `v2025.x` with velox `v2025`.
- Module path moved to `github.com/roadrunner-server/velox/v3`; the CLI installs from `github.com/roadrunner-server/velox/v3/cmd/vx`.
- The Connect/gRPC build server is removed. Velox is a CLI only (`vx build`), driven by `velox.toml`.
- Windows build targets are rejected: `target_platform.os = "windows"` fails configuration validation, and no Windows binary is released.

### Added

- `[[replaces]]` and `[[excludes]]` sections map to go.mod `replace` and `exclude` directives, applied before `go mod tidy`. A relative local path in `[[replaces]].new` resolves against the working directory.
- Deterministic 5-letter plugin import prefixes and module-path ordering, so the same plugin set renders a bit-identical `container/plugins.go`.
- `[debug] race = true` builds with `-race`.
- A stable release also pushes the major and minor image tags (`3`, `3.0`) next to the version tag and `latest`.

### Changed

- The go.mod of the downloaded RoadRunner source is kept as-is and edited through `go mod edit` plus `go mod tidy`, in place of a templated go.mod. All require, replace, and exclude directives go into one `go mod edit` call.
- The bundled informer and resetter module paths come from `go mod edit -json` on the upstream go.mod, in place of regexp scraping. A `velox.toml` entry for either of them is dropped before every pipeline step.
- Post-tidy version verification runs as a single batched `go list -m -e -json` over the plugins pinned to a semver tag, and understands `replace` directives. A tag that is not a semver version (`latest`, a branch, a commit) is not checked.
- Subprocess cancellation uses `cmd.Cancel` and `cmd.WaitDelay`: the go tool gets SIGINT and 15 seconds before SIGKILL.
- The binary is compiled in a temporary `.rr-build-*` directory inside the output directory and moved to `rr` after the smoke test, which now fails when `rr --version` does not print the requested ref.
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
