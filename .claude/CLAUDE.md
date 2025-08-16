# Velox - RoadRunner Build System

## Project Overview

Velox is an automated build system for RoadRunner server and its plugins. It's part of the Spiral/RoadRunner ecosystem and provides tools for building, testing, and managing RoadRunner plugin builds.

**Key Technologies:**

- Go 1.24+
- Protocol Buffers (protobuf) with buf
- GitHub/GitLab API integration
- gRPC and Connect
- Cobra CLI framework

## Repository Structure

```
├── api/                    # Protocol buffer definitions
├── builder/               # Core build logic
├── cmd/vx/               # Main CLI entry point
├── gen/                  # Generated protobuf code
├── github/               # GitHub API integration
├── gitlab/               # GitLab API integration
├── internal/cli/         # CLI command implementations
├── v2/                   # Version 2 refactored components
└── velox.toml           # Configuration file
```

## Common Commands

### Building and Testing

```bash
# Run tests
go test -v -race ./...
make test

# Regenerate protobuf code
make regenerate
# Or manually:
rm -rf ./gen && buf generate && buf format -w

# Build the velox binary
go build -o vx ./cmd/vx

# Run velox (requires config)
./vx -c velox.toml build
./vx -c velox.toml server -a 127.0.0.1:8080
```

### Development Tools

```bash
# Format Go code
go fmt ./...

# Run linter (if available)
golangci-lint run

# Check dependencies
go mod tidy
go mod verify
```

## Core Files and Components

### Main Entry Points

- `cmd/vx/main.go` - CLI application entry point
- `internal/cli/root.go:18` - Root command setup with configuration

### Build System

- `builder/builder.go` - Core build logic
- `builder/templates/` - Build templates for different versions
- `v2/builder/` - Refactored v2 build system

### Configuration

- `config.go` - Configuration structure and validation
- `velox.toml` - Default configuration file format

### API Integration

- `github/repo.go` - GitHub repository management
- `gitlab/repo.go` - GitLab repository management
- `api/` - Protocol buffer service definitions

## Code Style Guidelines

### Go Conventions

- Follow standard Go formatting (`gofmt`)
- Use meaningful variable names
- Prefer explicit error handling
- Use context.Context for cancellation
- Follow Go module structure with `github.com/roadrunner-server/velox/v2025`

### Protocol Buffers

- Use buf for generation and formatting
- Service definitions in `api/service/v1/`
- Generate code with `buf generate`

### Error Handling

- Use `github.com/pkg/errors` for error wrapping
- Always handle errors explicitly
- Use structured logging with zap

## Testing Instructions

```bash
# Run all tests with race detection
make test
# or
go test -v -race ./...

# Run specific package tests
go test -v ./builder/
go test -v ./github/

# Run with coverage
go test -cover ./...
```

## Repository Etiquette

### Branching

- Main branch: `master`
- Feature branches: `feature/description`
- Use conventional commit messages

### Plugin Compatibility

⚠️ **Important Plugin Guidelines:**

- Do not use plugin's `master` branch
- Use tags with the **same major version**
- Currently supported plugins version: `v5.x.x`
- Currently supported RR version: `>=v2024.x.x`

### Commit Guidelines

- Use descriptive commit messages
- Reference issues when applicable
- Keep commits focused and atomic

## Developer Environment Setup

### Prerequisites

- Go 1.24+ (toolchain: go1.24.0)
- buf CLI for protocol buffer generation
- Git for version control

### Setup Steps

1. Clone repository
2. Install dependencies: `go mod download`
3. Generate protobuf code: `make regenerate`
4. Run tests: `make test`
5. Build: `go build ./cmd/vx`

### Configuration

Create `velox.toml` based on project requirements. The CLI expects:

```bash
./vx -c velox.toml build    # Build mode
./vx -c velox.toml server   # Server mode
```

## Unexpected Project Behaviors

### Version Management

- Project uses `v2025` module path
- Replace directive: `github.com/roadrunner-server/velox/v2025/gen => ./gen`
- Multiple template versions in `builder/templates/`

### Build System

- Templates are versioned (V2, V2023, V2024, V2025)
- Build process involves GitHub/GitLab API calls
- Server mode provides build-as-a-service functionality

### Protobuf Generation

- Uses buf instead of protoc directly
- Generated code goes into `gen/` directory
- Must regenerate after proto changes

## Useful Development Patterns

### Adding New Build Templates

1. Create new template in `builder/templates/`
2. Update `builder.go` to reference new template
3. Add corresponding tests in `builder_test.go`

### GitHub/GitLab Integration

- Use existing pool patterns in `github/pool.go`
- Implement repository interface consistently
- Handle rate limiting and authentication

### CLI Commands

- Follow cobra patterns in `internal/cli/`
- Use persistent flags for common options
- Implement proper validation in `PersistentPreRunE`

## Links and Documentation

- [RoadRunner Docs](https://docs.roadrunner.dev/customization/build)
- [Project Repository](https://github.com/roadrunner-server/velox)
- [Discord Community](https://discord.gg/TFeEmCs)
