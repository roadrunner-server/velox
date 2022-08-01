package builder

import (
	"os"
	"path"
	"testing"

	"github.com/roadrunner-server/velox/common"
	"go.uber.org/zap"
)

const (
	replaceGoModOneRelative = `module go.dev/my/module
go 1.18

require (
	github.com/fatih/color v1.13.0
)

replace github.com/dummy/package => ./something
`
	replaceGoModOneAbsolute = `module go.dev/my/module
go 1.18

require (
	github.com/fatih/color v1.13.0
)

replace github.com/dummy/package => /tmp/dummy
`
	replaceGoModMultipleRelative = `module go.dev/my/module
go 1.18

require (
	github.com/fatih/color v1.13.0
)

replace (
	github.com/dummy/package => ./something
	github.com/dummy/another => ../../another
)
`
	replaceGoModMultipleAbsolute = `module go.dev/my/module
go 1.18

require (
	github.com/fatih/color v1.13.0
)

replace (
	github.com/dummy/package_one => /tmp/dummy_one
	github.com/dummy/package_two => /tmp/dummy_two
)
`
	replaceGoModMultipleRemote = `module go.dev/my/module
go 1.18

require (
	github.com/fatih/color v1.13.0
)

replace (
	github.com/dummy/package_one => https://github.com/my/package_one
	github.com/dummy/package_two => https://github.com/my/package_two
)
`
)

var associated = map[string][]byte{
	"dummy_one_relative":             []byte(replaceGoModOneRelative),
	"dummy_one_absolute":             []byte(replaceGoModOneAbsolute),
	"dummy_multiple_relative":        []byte(replaceGoModMultipleRelative),
	"dummy_multiple_absolute":        []byte(replaceGoModMultipleAbsolute),
	"dummy_multiple_absolute_remote": []byte(replaceGoModMultipleRemote),
}

func Test_Builder_getDepsReplace(t *testing.T) {
	b := NewBuilder("/tmp", []*common.ModulesInfo{}, "", zap.NewNop(), []string{})

	b.modules = []*common.ModulesInfo{
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
		_ = os.Mkdir(v.Replace, 0777)
		_ = os.WriteFile(path.Join(v.Replace, goModStr), associated[v.ModuleName], 0777)
	}

	defer func() {
		for _, v := range b.modules {
			_ = os.RemoveAll(v.Replace)
		}
	}()

	toReplace := b.getDepsReplace("/tmp/dummy_multiple_absolute")
	if len(toReplace) != 2 {
		t.Error("/tmp/dummy_multiple_absolute must have 2 elements to replace")
	}
	if toReplace[0].Module != "github.com/dummy/package_one" || toReplace[0].Replace != "/tmp/dummy_one" {
		t.Error("The first module to replace must be github.com/dummy/package_one with the replacer /tmp/dummy_one")
	}
	if toReplace[1].Module != "github.com/dummy/package_two" || toReplace[1].Replace != "/tmp/dummy_two" {
		t.Error("The first module to replace must be github.com/dummy/package_two with the replacer /tmp/dummy_two")
	}

	toReplace = b.getDepsReplace("/tmp/dummy_multiple_relative")
	if len(toReplace) != 2 {
		t.Error("/tmp/dummy_multiple_relative must have 2 elements to replace")
	}
	if toReplace[0].Module != "github.com/dummy/package" || toReplace[0].Replace != "/tmp/dummy_multiple_relative/something" {
		t.Error("The first module to replace must be github.com/dummy/package with the replacer /tmp/dummy_multiple_relative/something")
	}
	if toReplace[1].Module != "github.com/dummy/another" || toReplace[1].Replace != "/another" {
		t.Error("The first module to replace must be github.com/dummy/another with the replacer /another")
	}

	toReplace = b.getDepsReplace("/tmp/dummy_one_absolute")
	if len(toReplace) != 1 {
		t.Error("/tmp/dummy_one_absolute must have 1 element to replace")
	}
	if toReplace[0].Module != "github.com/dummy/package" || toReplace[0].Replace != "/tmp/dummy" {
		t.Error("The module to replace must be github.com/dummy/package with the replacer /tmp/dummy")
	}

	toReplace = b.getDepsReplace("/tmp/dummy_one_relative")
	if len(toReplace) != 1 {
		t.Error("/tmp/dummy_one_relative must have 1 element to replace")
	}
	if toReplace[0].Module != "github.com/dummy/package" || toReplace[0].Replace != "/tmp/dummy_one_relative/something" {
		t.Error("The module to replace must be github.com/dummy/package with the replacer /tmp/dummy_one_relative/something")
	}

	toReplace = b.getDepsReplace("/tmp/dummy_multiple_absolute_remote")
	if len(toReplace) != 2 {
		t.Error("/tmp/dummy_multiple_relative must have 2 elements to replace")
	}
	if toReplace[0].Module != "github.com/dummy/package_one" || toReplace[0].Replace != "https://github.com/my/package_one" {
		t.Error("The first module to replace must be github.com/dummy/package_one with the replacer https://github.com/my/package_one")
	}
	if toReplace[1].Module != "github.com/dummy/package_two" || toReplace[1].Replace != "https://github.com/my/package_two" {
		t.Error("The first module to replace must be github.com/dummy/package_two with the replacer https://github.com/my/package_two")
	}
}
