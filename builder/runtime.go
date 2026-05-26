package builder

import "runtime"

// Wrapped so tests can swap them out via the package-level vars in builder.go.
func goosFromRuntime() string   { return runtime.GOOS }
func goarchFromRuntime() string { return runtime.GOARCH }
