package common

// ModulesInfo represents single go module
type ModulesInfo struct {
	// Version - commit sha or tag
	Version string
	// module name - eg: github.com/roadrunner-server/logger/v2
	ModuleName string
	// Replace (for the local dev)
	Replace string
}
