module github.com/roadrunner-server/velox/v2025

go 1.25

toolchain go1.25.0

require (
	buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go v1.36.7-20250717185734-6c6e0d3c608e.1
	buf.build/go/protovalidate v0.14.0
	github.com/bufbuild/connect-go v1.10.0
	github.com/fatih/color v1.18.0
	github.com/google/go-github/v74 v74.0.0
	github.com/hashicorp/go-version v1.7.0
	github.com/hashicorp/golang-lru/v2 v2.0.7
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v1.9.1
	github.com/spf13/viper v1.20.1
	github.com/stretchr/testify v1.10.0
	gitlab.com/gitlab-org/api/client-go v0.139.2
	go.uber.org/zap v1.27.0
	golang.org/x/mod v0.27.0
	golang.org/x/net v0.43.0
	golang.org/x/oauth2 v0.30.0
	google.golang.org/grpc v1.74.2
	google.golang.org/protobuf v1.36.7
)

replace github.com/roadrunner-server/velox/v2025/gen => ./gen

require (
	cel.dev/expr v0.24.0 // indirect
	github.com/antlr4-go/antlr/v4 v4.13.1 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/google/cel-go v0.26.0 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.8 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/pelletier/go-toml/v2 v2.2.4 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/sagikazarmark/locafero v0.10.0 // indirect
	github.com/sourcegraph/conc v0.3.1-0.20240121214520-5f936abd7ae8 // indirect
	github.com/spf13/afero v1.14.0 // indirect
	github.com/spf13/cast v1.9.2 // indirect
	github.com/spf13/pflag v1.0.7 // indirect
	github.com/stoewer/go-strcase v1.3.1 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/exp v0.0.0-20250813145105-42675adae3e6 // indirect
	golang.org/x/sys v0.35.0 // indirect
	golang.org/x/text v0.28.0 // indirect
	golang.org/x/time v0.12.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20250811230008-5f3141c8851a // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250811230008-5f3141c8851a // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
