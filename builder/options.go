package builder

import (
	"strings"

	"go.uber.org/zap"
)

// Option represents a configuration option for the Builder
type Option func(*Builder)

// WithDebug sets the debug flag for the builder
func WithDebug(debug bool) Option {
	return func(b *Builder) {
		b.debug = debug
	}
}

// WithGOOS sets the target operating system for the build (e.g., "linux", "windows", "darwin")
func WithGOOS(goos string) Option {
	return func(b *Builder) {
		b.goos = goos
	}
}

// WithGOARCH sets the target architecture for the build (e.g., "amd64", "arm64", "386")
func WithGOARCH(goarch string) Option {
	return func(b *Builder) {
		b.goarch = goarch
	}
}

// WithLogs sets a string builder to capture all logs
func WithLogs(sb *strings.Builder) Option {
	return func(b *Builder) {
		b.sb = sb
	}
}

// WithLogger sets the logger for the builder
func WithLogger(log *zap.Logger) Option {
	return func(b *Builder) {
		b.log = log
	}
}

// WithOutputDir sets the output directory for the builder
func WithOutputDir(outputDir string) Option {
	return func(b *Builder) {
		b.outputDir = outputDir
	}
}

// WithRRVersion sets the RoadRunner version for the builder
func WithRRVersion(rrVersion string) Option {
	return func(b *Builder) {
		b.rrVersion = rrVersion
	}
}
