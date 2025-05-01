package builder

import (
	"os"
	"path"
	"testing"

	"github.com/roadrunner-server/velox/v2025"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

const (
	rights                  = 0700
	dummyPackage            = "github.com/dummy/package"
	dummyPackageOne         = "github.com/dummy/package_one"
	dummyPackageTwo         = "github.com/dummy/package_two"
	remotePackageOne        = "https://github.com/my/package_one"
	remotePackageTwo        = "https://github.com/my/package_two"
	replaceGoModOneRelative = `module go.dev/my/module
go 1.20

require (
	github.com/fatih/color v1.13.0
)

replace github.com/dummy/package => ./something
`
	replaceGoModOneAbsolute = `module go.dev/my/module
go 1.20

require (
	github.com/fatih/color v1.13.0
)

replace github.com/dummy/package => /tmp/dummy
`
	replaceGoModMultipleRelative = `module go.dev/my/module
go 1.20

require (
	github.com/fatih/color v1.13.0
)

replace (
	github.com/dummy/package => ./something
	github.com/dummy/another => ../../another
)
`
	replaceGoModMultipleAbsolute = `module go.dev/my/module
go 1.20

require (
	github.com/fatih/color v1.13.0
)

replace (
	github.com/dummy/package_one => /tmp/dummy_one
	github.com/dummy/package_two => /tmp/dummy_two
)
`
	replaceGoModMultipleRemote = `module go.dev/my/module
go 1.20

require (
	github.com/fatih/color v1.13.0
)

replace (
	github.com/dummy/package_one => https://github.com/my/package_one
	github.com/dummy/package_two => https://github.com/my/package_two
)
`
)

func setup(version string) *Builder {
	associated := map[string][]byte{
		"dummy_one_relative":             []byte(replaceGoModOneRelative),
		"dummy_one_absolute":             []byte(replaceGoModOneAbsolute),
		"dummy_multiple_relative":        []byte(replaceGoModMultipleRelative),
		"dummy_multiple_absolute":        []byte(replaceGoModMultipleAbsolute),
		"dummy_multiple_absolute_remote": []byte(replaceGoModMultipleRemote),
	}

	l, _ := zap.NewDevelopment()
	b := NewBuilder("/tmp", []*velox.ModulesInfo{}, "", version, false, l)

	b.modules = []*velox.ModulesInfo{
		{
			Version:    "master",
			ModuleName: "dummy_one_relative",
			Replace:    "/tmp/dummy_one_relative",
		},
		{
			Version:    "master",
			ModuleName: "dummy_one_absolute",
			Replace:    "/tmp/dummy_one_absolute",
		},
		{
			Version:    "master",
			ModuleName: "dummy_multiple_relative",
			Replace:    "/tmp/dummy_multiple_relative",
		},
		{
			Version:    "master",
			ModuleName: "dummy_multiple_absolute",
			Replace:    "/tmp/dummy_multiple_absolute",
		},
		{
			Version:    "master",
			ModuleName: "dummy_multiple_absolute_remote",
			Replace:    "/tmp/dummy_multiple_absolute_remote",
		},
	}

	for _, v := range b.modules {
		_ = os.Mkdir(v.Replace, rights)
		_ = os.WriteFile(path.Join(v.Replace, goModStr), associated[v.ModuleName], rights)
	}

	return b
}

func clean(b *Builder) {
	for _, v := range b.modules {
		_ = os.RemoveAll(v.Replace)
	}
}

// Helper function to run the getDepsReplace tests
func runGetDepsReplaceTest(t *testing.T, b *Builder, modulePath string, expectedCount int, expectedReplacements map[string]string) {
	t.Helper()
	toReplace := b.getDepsReplace(modulePath)
	assert.Len(t, toReplace, expectedCount, "%s must have %d elements to replace", modulePath, expectedCount)

	actualReplacements := make(map[string]string)
	for _, r := range toReplace {
		actualReplacements[r.Module] = r.Replace
	}

	assert.Equal(t, expectedReplacements, actualReplacements, "Replacements do not match for %s", modulePath)
}

func Test_Builder_getDepsReplace_multipleAbsolute_V2024(t *testing.T) {
	b := setup("v2024.1.0")
	defer clean(b)
	runGetDepsReplaceTest(t, b, "/tmp/dummy_multiple_absolute", 2, map[string]string{
		dummyPackageOne: "/tmp/dummy_one",
		dummyPackageTwo: "/tmp/dummy_two",
	})
}

func Test_Builder_getDepsReplace_multipleRelative_V2024(t *testing.T) {
	b := setup("v2024.1.0")
	defer clean(b)
	runGetDepsReplaceTest(t, b, "/tmp/dummy_multiple_relative", 2, map[string]string{
		dummyPackage:               "/tmp/dummy_multiple_relative/something",
		"github.com/dummy/another": "/another",
	})
}

func Test_Builder_getDepsReplace_oneAbsolute_V2024(t *testing.T) {
	b := setup("v2024.1.0")
	defer clean(b)
	runGetDepsReplaceTest(t, b, "/tmp/dummy_one_absolute", 1, map[string]string{
		dummyPackage: "/tmp/dummy",
	})
}

func Test_Builder_getDepsReplace_oneRelative_V2024(t *testing.T) {
	b := setup("v2024.1.0")
	defer clean(b)
	runGetDepsReplaceTest(t, b, "/tmp/dummy_one_relative", 1, map[string]string{
		dummyPackage: "/tmp/dummy_one_relative/something",
	})
}

func Test_Builder_getDepsReplace_multipleAbsoluteRemote_V2024(t *testing.T) {
	b := setup("v2024.1.0")
	defer clean(b)
	runGetDepsReplaceTest(t, b, "/tmp/dummy_multiple_absolute_remote", 2, map[string]string{
		dummyPackageOne: remotePackageOne,
		dummyPackageTwo: remotePackageTwo,
	})
}

// --- V2025 Tests ---

func Test_Builder_getDepsReplace_multipleAbsolute_V2025(t *testing.T) {
	b := setup("v2025.0.0") // Use a v2025 version
	defer clean(b)
	runGetDepsReplaceTest(t, b, "/tmp/dummy_multiple_absolute", 2, map[string]string{
		dummyPackageOne: "/tmp/dummy_one",
		dummyPackageTwo: "/tmp/dummy_two",
	})
}

func Test_Builder_getDepsReplace_multipleRelative_V2025(t *testing.T) {
	b := setup("v2025.0.0")
	defer clean(b)
	runGetDepsReplaceTest(t, b, "/tmp/dummy_multiple_relative", 2, map[string]string{
		dummyPackage:               "/tmp/dummy_multiple_relative/something",
		"github.com/dummy/another": "/another",
	})
}

func Test_Builder_getDepsReplace_oneAbsolute_V2025(t *testing.T) {
	b := setup("v2025.0.0")
	defer clean(b)
	runGetDepsReplaceTest(t, b, "/tmp/dummy_one_absolute", 1, map[string]string{
		dummyPackage: "/tmp/dummy",
	})
}

func Test_Builder_getDepsReplace_oneRelative_V2025(t *testing.T) {
	b := setup("v2025.0.0")
	defer clean(b)
	runGetDepsReplaceTest(t, b, "/tmp/dummy_one_relative", 1, map[string]string{
		dummyPackage: "/tmp/dummy_one_relative/something",
	})
}

func Test_Builder_getDepsReplace_multipleAbsoluteRemote_V2025(t *testing.T) {
	b := setup("v2025.0.0")
	defer clean(b)
	runGetDepsReplaceTest(t, b, "/tmp/dummy_multiple_absolute_remote", 2, map[string]string{
		dummyPackageOne: remotePackageOne,
		dummyPackageTwo: remotePackageTwo,
	})
}
