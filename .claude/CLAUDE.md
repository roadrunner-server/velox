# Velox Project Documentation

## Overview
Velox is a RoadRunner plugin builder that supports downloading and building plugins from GitHub and GitLab repositories. It provides both CLI and gRPC server interfaces for building custom RoadRunner distributions.

## Repository Structure

```
velox/
├── .claude/                    # Claude documentation
├── .git/                       # Git repository data
├── .github/                    # GitHub workflows and templates
├── .gitignore                  # Git ignore patterns
├── .golangci.yml              # Go linter configuration
├── .idea/                      # JetBrains IDE configuration
├── .mypy_cache/               # Python MyPy cache
├── .venv/                     # Python virtual environment
├── .vscode/                   # VS Code configuration
├── Dockerfile                 # Docker container definition
├── Dockerfile_sample          # Sample Docker configuration
├── LICENSE                    # Project license
├── Makefile                   # Build automation
├── README.md                  # Project documentation
├── SECURITY.md               # Security policy
├── api/                      # Protocol Buffer definitions
│   ├── request/v1/           # Request message definitions
│   │   └── request.proto
│   ├── response/v1/          # Response message definitions
│   │   └── response.proto
│   └── service/v1/           # Service definitions
│       └── service.proto
├── buf.gen.yaml              # Buf code generation configuration
├── buf.yaml                  # Buf build configuration
├── builder/                  # Build engine implementation
│   ├── builder.go            # Main builder logic
│   ├── builder_test.go       # Builder tests
│   ├── template_test.go      # Template tests
│   └── templates/            # Go template files for different RR versions
│       ├── entry.go          # Template entry point
│       ├── templateV2.go     # RoadRunner v2 template
│       ├── templateV2023.go  # RoadRunner v2023 template
│       ├── templateV2024.go  # RoadRunner v2024 template
│       └── templateV2025.go  # RoadRunner v2025 template
├── cmd/                      # Command line applications
│   └── vx/                   # Velox CLI application
│       └── main.go
├── config.go                 # Configuration types and validation
├── config_test.go           # Configuration tests
├── docker-compose.yml       # Docker Compose configuration
├── gen/                     # Generated code from Protocol Buffers
│   └── go/                  # Go generated code
│       └── api/
│           ├── request/v1/
│           │   └── request.pb.go
│           ├── response/v1/
│           │   └── response.pb.go
│           └── service/v1/
│               ├── service.pb.go
│               ├── service_grpc.pb.go
│               └── serviceV1connect/
│                   └── service.connect.go
├── github/                   # GitHub repository integration
│   ├── parse_test.go        # GitHub parsing tests
│   ├── pool.go              # GitHub connection pool
│   └── repo.go              # GitHub repository operations
├── gitlab/                   # GitLab repository integration
│   └── repo.go              # GitLab repository operations
├── go.mod                   # Go module definition
├── go.sum                   # Go module checksums
├── internal/                # Internal packages
│   ├── cli/                 # CLI command implementations
│   │   ├── root.go          # Root command
│   │   ├── build/           # Build command
│   │   │   └── build.go
│   │   └── server/          # Server command
│   │       ├── command.go   # Server CLI command
│   │       └── server.go    # gRPC server implementation
│   └── version/             # Version information
│       └── version.go
├── logger/                  # Logging utilities
│   └── logger.go
├── modulesInfo.go          # Module information handling
├── update_plugins.py       # Python script for plugin updates
└── velox.toml             # Project configuration file
```

## Core Types and Structures

### Configuration Types

#### `Config`
Main configuration structure for the velox application:
```go
type Config struct {
    Roadrunner map[string]string `mapstructure:"roadrunner"` // RoadRunner version configuration
    Debug      *Debug            `mapstructure:"debug"`      // Debug settings
    GitHub     *CodeHosting      `mapstructure:"github"`     // GitHub configuration
    GitLab     *CodeHosting      `mapstructure:"gitlab"`     // GitLab configuration
    Log        map[string]string `mapstructure:"log"`        // Logging configuration
}
```

#### `Debug`
Debug configuration:
```go
type Debug struct {
    Enabled bool `mapstructure:"enabled"`
}
```

#### `Token`
Authentication token configuration:
```go
type Token struct {
    Token string `mapstructure:"token"`
}
```

#### `Endpoint`
API endpoint configuration:
```go
type Endpoint struct {
    BaseURL string `mapstructure:"endpoint"`
}
```

#### `CodeHosting`
Generic code hosting platform configuration (GitHub/GitLab):
```go
type CodeHosting struct {
    BaseURL *Endpoint                `mapstructure:"endpoint"` // API base URL
    Token   *Token                   `mapstructure:"token"`    // Authentication token
    Plugins map[string]*PluginConfig `mapstructure:"plugins"`  // Plugin configurations
}
```

#### `PluginConfig`
Individual plugin configuration:
```go
type PluginConfig struct {
    Ref     string `mapstructure:"ref"`        // Git reference (branch/tag/commit)
    Owner   string `mapstructure:"owner"`      // Repository owner
    Repo    string `mapstructure:"repository"` // Repository name
    Folder  string `mapstructure:"folder"`     // Specific folder in repository
    Replace string `mapstructure:"replace"`    // Local replacement path
}
```

### Module Information Types

#### `ModulesInfo`
Represents Go module information:
```go
type ModulesInfo struct {
    Version       string // Commit SHA or tag
    PseudoVersion string // Go pseudo version
    ModuleName    string // Module name (e.g., github.com/roadrunner-server/logger/v2)
    Replace       string // Local development replacement path
}
```

### Protocol Buffer Types (API)

#### Request Types (`api/request/v1`)

##### `HostingPlatformType` (Enum)
```go
const (
    HostingPlatformType_HOSTING_PLATFORM_TYPE_UNSPECIFIED HostingPlatformType = 0
    HostingPlatformType_HOSTING_PLATFORM_TYPE_TYPE_GITHUB HostingPlatformType = 1
    HostingPlatformType_HOSTING_PLATFORM_TYPE_TYPE_GITLAB HostingPlatformType = 2
)
```

##### `HostingPlatform`
```go
type HostingPlatform struct {
    HostingPlatform HostingPlatformType `json:"hosting_platform,omitempty"`
}
```

##### `BuildRequest`
Main build request message:
```go
type BuildRequest struct {
    RrVersion   string                 `json:"rr_version,omitempty"`   // RoadRunner version
    PluginsInfo map[string]*PluginInfo `json:"plugins_info,omitempty"` // Plugin information by platform
}
```

##### `PluginInfo`
Plugin information for a specific hosting platform:
```go
type PluginInfo struct {
    HostingPlatform *HostingPlatform `json:"hosting_platform,omitempty"` // Platform type
    Plugins         []*Plugin        `json:"plugins,omitempty"`          // List of plugins
}
```

##### `Plugin`
Individual plugin specification:
```go
type Plugin struct {
    Name       string `json:"name,omitempty"`       // Optional plugin name
    Ref        string `json:"ref,omitempty"`        // Git reference
    Owner      string `json:"owner,omitempty"`      // Repository owner
    Repository string `json:"repository,omitempty"` // Repository name
}
```

#### Response Types (`api/response/v1`)

##### `BuildResponse`
Build operation response:
```go
type BuildResponse struct {
    Path string `json:"path,omitempty"` // Path to built binary
}
```

### Builder Types

#### `Builder`
Main builder structure:
```go
type Builder struct {
    rrTempPath string            // Temporary path for RoadRunner
    out        string            // Output path
    modules    []*velox.ModulesInfo // Module information
    log        *zap.Logger       // Logger instance
    debug      bool              // Debug mode flag
    rrVersion  string            // RoadRunner version
}
```

### Repository Integration Types

#### GitHub Integration (`github/`)

##### `GHRepo`
GitHub repository handler:
```go
type GHRepo struct {
    client *github.Client // GitHub API client
    config *velox.Config  // Velox configuration
    log    *zap.Logger    // Logger instance
}
```

#### GitLab Integration (`gitlab/`)

##### `GLRepo`
GitLab repository handler:
```go
type GLRepo struct {
    client *gitlab.Client // GitLab API client
    config *velox.Config  // Velox configuration
    log    *zap.Logger    // Logger instance
}
```

## Constants and Default Values

### Version Constants
```go
const (
    V2025 string = "v2025"
    V2024 string = "v2024"
    V2023 string = "v2023"
    V2    string = "v2"
)
```

### Default Configuration
```go
var DefaultConfig = &Config{
    Roadrunner: map[string]string{
        "ref": "master",
    },
    Debug: &Debug{
        Enabled: false,
    },
    Log: map[string]string{
        "level": "debug", 
        "mode": "development",
    },
}
```

### Builder Constants
```go
const (
    pluginsPath        = "/container/plugins.go"
    goModStr          = "go.mod"
    pluginStructureStr = "Plugin{}"
    rrMainGo          = "cmd/rr/main.go"
    executableName    = "rr"
    cleanupPattern    = "roadrunner-server*"
)
```

## Key Features

1. **Multi-Platform Support**: Supports both GitHub and GitLab as plugin sources
2. **Version Management**: Handles multiple RoadRunner versions (v2, v2023, v2024, v2025)
3. **Template System**: Uses Go templates to generate build configurations
4. **gRPC API**: Provides Connect-compatible gRPC API for remote building
5. **CLI Interface**: Command-line interface for local building
6. **Plugin Management**: Automatic plugin discovery and integration
7. **Docker Support**: Containerized building environment
8. **Configuration Validation**: Comprehensive configuration validation
9. **Logging**: Structured logging with zap
10. **Module Handling**: Advanced Go module version management

## API Endpoints

### gRPC Service (`api/service/v1`)
- `Build(BuildRequest) returns (BuildResponse)` - Build RoadRunner with specified plugins

### Connect HTTP API
- `POST /api.service.v1.BuildService/Build` - HTTP/JSON equivalent of gRPC Build method

This documentation provides a comprehensive overview of the velox project structure, types, and functionality for easy navigation and understanding.
