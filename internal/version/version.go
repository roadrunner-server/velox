package version

import (
	"strings"
)

var (
	version   = "local"
	buildTime = "development" //nolint:gochecknoglobals
)

// Version returns the version string with leading 'v' or 'V' prefix stripped if followed by a digit.
// The version is set via ldflags during build.
func Version() string {
	v := strings.TrimSpace(version)

	if len(v) > 1 && ((v[0] == 'v' || v[0] == 'V') && (v[1] >= '0' && v[1] <= '9')) {
		return v[1:]
	}

	return v
}

// BuildTime returns the build timestamp string set via ldflags during build.
func BuildTime() string {
	return buildTime
}
