package github

import (
	"testing"
	"time"

	"github.com/roadrunner-server/velox/v2025"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type tm struct {
	module string
	tm     time.Time
	sha    string
	expect string
}

func TestParse(t *testing.T) {
	tn, err := time.Parse("20060102150405", "20231008162055")
	require.NoError(t, err)

	tests := []tm{
		{
			module: "github.com/roadrunner-server/logger/va",
			tm:     tn,
			sha:    "1234567890",
			expect: "v0.0.0-20231008162055-1234567890",
		},
		{
			module: "github.com/roadrunner-server/logger/v2",
			tm:     tn,
			sha:    "1234567890",
			expect: "v2.0.0-20231008162055-1234567890",
		},
		{
			module: "github.com/roadrunner-server/logger/v2222222222222",
			tm:     tn,
			sha:    "1234567890",
			expect: "v2222222222222.0.0-20231008162055-1234567890",
		},
		{
			module: "github.com/roadrunner-server/logger",
			tm:     tn,
			sha:    "1234567890",
			expect: "v0.0.0-20231008162055-1234567890",
		},
		{
			module: "github.com/roadrunner-server/logger/v2",
			tm:     tn,
			sha:    "",
			expect: "v2.0.0-20231008162055-",
		},
		{
			expect: "v0.0.0-00010101000000-",
		},
	}

	for _, tt := range tests {
		out := velox.ParseModuleInfo(tt.module, tt.tm, tt.sha)
		assert.Equal(t, tt.expect, out)
	}
}
