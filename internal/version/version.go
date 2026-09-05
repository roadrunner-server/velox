package version

import (
	"strings"
)

var (
	version   = "local"
	buildTime = "development" //nolint:gochecknoglobals // the linker sets both vars through -X at build time
)

// Version returns the ldflags version and drops a leading "v" or "V" that precedes a digit.
func Version() string {
	v := strings.TrimSpace(version)

	if len(v) > 1 && ((v[0] == 'v' || v[0] == 'V') && (v[1] >= '0' && v[1] <= '9')) {
		return v[1:]
	}

	return v
}

// BuildTime returns the build timestamp set through ldflags.
func BuildTime() string {
	return buildTime
}
