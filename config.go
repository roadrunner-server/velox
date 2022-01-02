package rrbuild

// config represents repository configuration
type config struct {
	// version to get from the git
	version string
	// module name, user should specify the module name, because it'll be added to the resulting go.mod
	module string
}
