# Repository Guidelines

## Project Structure & Module Organization

- `cmd/vx/` — CLI entrypoint (`vx`).
- `internal/cli/` — commands: `build` and `server`.
- `builder/` — build engine and `templates/`.
- `plugin/`, `logger/`, `cache/`, `github/` — helpers and clients.
- `api/` — protobuf sources; generated code lives in `gen/` (do not edit `gen/` directly).
- Root configs: `velox.toml`, `.golangci.yml`, `buf.yaml`, `buf.gen.yaml`.

## Build, Test, and Development Commands

- Install toolchain: Go 1.25+, `buf` CLI, optional `golangci-lint`.
- Run tests: `make test` (alias for `go test -v -race ./...`).
- Lint: `golangci-lint run` (config in `.golangci.yml`).
- Regenerate protobufs: `make regenerate` (removes `gen/`, runs `buf generate` and `buf format -w`).
- Build and run CLI locally:
  - `go run ./cmd/vx --help`
  - or `go install github.com/roadrunner-server/velox/v2025/cmd/vx@latest`
- Build RoadRunner with plugins: `vx build -c velox.toml -o .`
- Run server mode: `vx server -a 127.0.0.1:8080` (uses `GITHUB_TOKEN` if set).

## Coding Style & Naming Conventions

- Use `gofmt`/`goimports`; 1‑tab indentation; keep lines ≤120 chars.
- Follow `.golangci.yml`: avoid globals/`init`, handle errors, no unused code, preallocate where helpful.
- Package names are lowercase; exported identifiers use PascalCase; tests in `*_test.go`.

## Testing Guidelines

- Frameworks: standard `testing`, `testify/assert` and `testify/require`.
- Name tests `TestXxx`; add `t.Parallel()` when safe; include race‑safe checks.
- Avoid network in unit tests; if required, guard with env vars (e.g., `GITHUB_TOKEN`) or skip.

## Commit & Pull Request Guidelines

- Prefer Conventional Commits (e.g., `feat:`, `fix:`, `chore(deps):`).
- Sign commits: `git commit -s`.
- PRs: clear description, linked issues, screenshots/logs when relevant, updated docs, and added/updated tests.
- Ensure CI passes: run `make test` and `golangci-lint run` locally before opening.

## Security & Configuration Tips

- Do not commit secrets; use env vars (e.g., `export GITHUB_TOKEN=...`).
- Cross‑compiling is supported via flags in CLI; binaries are built with `CGO_ENABLED=0`.
