# Changelog

## v3.0.0-beta.1

### Breaking

- Module path moved to `github.com/roadrunner-server/velox/v3`; the CLI installs from `github.com/roadrunner-server/velox/v3/cmd/vx`.
- The Connect/gRPC build server is removed. Velox is a CLI only (`vx build`), driven by `velox.toml`.
- Windows build targets are rejected: `target_platform.os = "windows"` fails configuration validation, and no Windows binary is released.

### Added

- `[[replaces]]` and `[[excludes]]` sections map to go.mod `replace` and `exclude` directives, applied before `go mod tidy`.
- Deterministic 5-letter plugin import prefixes, so the same plugin set renders a bit-identical `container/plugins.go`.

### Changed

- The go.mod of the downloaded RoadRunner source is kept as-is and edited through `go mod edit` plus `go mod tidy`, in place of a templated go.mod.
- The bundled informer and resetter module paths come from `go mod edit -json` on the upstream go.mod, in place of regexp scraping.
- Post-tidy version verification runs as a single batched `go list -m -e -json` over the pinned plugins, and understands `replace` directives.
- Subprocess cancellation uses `cmd.Cancel` and `cmd.WaitDelay`: the go tool gets SIGINT and 15 seconds before SIGKILL.
- `GOPATH` and `GOCACHE` are redirected to `~/go/<os>/<arch>` only for cross-compilation; a native build reuses the caller module cache.

### Fixed

- Version ldflags derive the meta package path from the module path in the downloaded RoadRunner go.mod. The hardcoded path matched no buildable ref, so version injection was inert and `rr --version` reported `local`.
