package version

import (
	"strings"
)

var (
	version   = "local"
	buildTime = "development" //nolint:gochecknoglobals
)

func Version() string {
	v := strings.TrimSpace(version)

	if len(v) > 1 && ((v[0] == 'v' || v[0] == 'V') && (v[1] >= '0' && v[1] <= '9')) {
		return v[1:]
	}

	return v
}

func BuildTime() string {
	return buildTime
}
