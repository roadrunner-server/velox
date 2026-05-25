package builder

import (
	"go.uber.org/zap"

	"github.com/roadrunner-server/velox/v3"
	"github.com/roadrunner-server/velox/v3/plugin"
)

// Option configures a Builder. Pass these to NewBuilder.
type Option func(*Builder)

// WithLogger sets the zap logger used for builder diagnostics.
func WithLogger(log *zap.Logger) Option {
	return func(b *Builder) { b.log = log }
}

// WithPlugins sets the plugins to include in the binary.
func WithPlugins(plugins ...*plugin.Plugin) Option {
	return func(b *Builder) { b.plugins = plugins }
}

// WithReplaces sets the go.mod replace directives to apply before tidy.
func WithReplaces(rs []velox.Replace) Option {
	return func(b *Builder) { b.replaces = rs }
}

// WithExcludes sets the go.mod exclude directives to apply before tidy.
func WithExcludes(es []velox.Exclude) Option {
	return func(b *Builder) { b.excludes = es }
}

// WithOutputDir sets the directory where the final binary is placed.
func WithOutputDir(outputDir string) Option {
	return func(b *Builder) { b.outputDir = outputDir }
}

// WithRRVersion sets the RR ref used to populate the binary's `-X meta.version` ldflag.
func WithRRVersion(rrVersion string) Option {
	return func(b *Builder) { b.rrVersion = rrVersion }
}

// WithGOOS sets the target GOOS for cross-compilation.
func WithGOOS(goos string) Option {
	return func(b *Builder) { b.goos = goos }
}

// WithGOARCH sets the target GOARCH for cross-compilation.
func WithGOARCH(goarch string) Option {
	return func(b *Builder) { b.goarch = goarch }
}

// WithDebug toggles the debug build profile (no inlining, no optimization, debug tag).
func WithDebug(debug bool) Option {
	return func(b *Builder) { b.debug = debug }
}

// WithRace toggles `-race` (forces CGO_ENABLED=1).
func WithRace(race bool) Option {
	return func(b *Builder) { b.race = race }
}
