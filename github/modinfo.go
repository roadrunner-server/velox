package github

import (
	"fmt"
	"regexp"
	"strconv"
	"time"

	m "golang.org/x/mod/module"
)

var vr = regexp.MustCompile(`/v(\\d+)$`)

// here we accept a module name and return the version
// e.g.: github.com/roadrunner-server/logger/v2 => v2
func parseModuleInfo(module string, t time.Time, rev string) string {
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
	_, err := strconv.ParseInt(num, 10, 64)
	return err == nil
}
