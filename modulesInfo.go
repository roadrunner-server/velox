package velox

import (
	"fmt"
	"regexp"
	"strconv"
	"time"

	m "golang.org/x/mod/module"
)

// ModulesInfo represents single go module
type ModulesInfo struct {
	// Version - commit sha or tag
	Version string
	// PseudoVersion - Go pseudo version
	PseudoVersion string
	// module name - eg: github.com/roadrunner-server/logger/v2
	ModuleName string
	// Replace (for the local dev)
	Replace string
}

var vr = regexp.MustCompile("/v(\\d+)$") //nolint:gosimple

// ParseModuleInfo here we accept a module name and return the version
// e.g.: github.com/roadrunner-server/logger/v2 => v2
func ParseModuleInfo(module string, t time.Time, rev string) string {
	match := vr.FindStringSubmatch(module)
	var version string
	if len(match) > 1 {
		if !IsDigit(match[1]) {
			return m.PseudoVersion("", "", t, rev)
		}

		version = fmt.Sprintf("v%s", match[1])
	}

	return m.PseudoVersion(version, "", t, rev)
}

func IsDigit(num string) bool {
	if num == "" {
		return false
	}
	_, err := strconv.ParseInt(num, 10, 64)
	return err == nil
}
