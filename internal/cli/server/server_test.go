package server

import (
	"testing"

	requestV1 "github.com/roadrunner-server/velox/v3/gen/go/api/request/v1"
)

// hashOf computes the cache key for req. generateCacheHash does not touch any
// BuildServer state, so a zero-value server is sufficient.
func hashOf(t *testing.T, req *requestV1.BuildRequest) string {
	t.Helper()
	h, err := (&BuildServer{}).generateCacheHash(req)
	if err != nil {
		t.Fatalf("generateCacheHash: %v", err)
	}
	return h
}

// sampleRequest returns a representative request with multiple plugins,
// replaces, and excludes so the sorting helpers have something to reorder.
func sampleRequest() *requestV1.BuildRequest {
	return &requestV1.BuildRequest{
		RequestId:      "req-1",
		RrVersion:      "v2025.1.0",
		TargetPlatform: &requestV1.Platform{Os: "linux", Arch: "amd64"},
		Plugins: []*requestV1.Plugin{
			{ModuleName: "github.com/roadrunner-server/http/v6", Tag: "v6.1.0"},
			{ModuleName: "github.com/roadrunner-server/logger/v6", Tag: "v6.1.0"},
			{ModuleName: "github.com/roadrunner-server/rpc/v6", Tag: "v6.0.0"},
		},
		Replaces: []*requestV1.Replace{
			{Old: "github.com/foo/bar", New: "../bar"},
			{Old: "github.com/baz/qux", New: "../qux"},
		},
		Excludes: []*requestV1.Exclude{
			{Module: "github.com/redis/go-redis/v9", Version: "v9.15.0"},
			{Module: "github.com/aaa/bbb", Version: "v1.0.0"},
		},
		Race:  false,
		Debug: false,
	}
}

func TestGenerateCacheHash_OrderIndependent(t *testing.T) {
	a := sampleRequest()

	// b is semantically identical but with every repeated list reversed.
	b := sampleRequest()
	b.Plugins = []*requestV1.Plugin{b.Plugins[2], b.Plugins[1], b.Plugins[0]}
	b.Replaces = []*requestV1.Replace{b.Replaces[1], b.Replaces[0]}
	b.Excludes = []*requestV1.Exclude{b.Excludes[1], b.Excludes[0]}

	if hashOf(t, a) != hashOf(t, b) {
		t.Fatalf("reordered-but-equal requests produced different hashes:\n a=%s\n b=%s",
			hashOf(t, a), hashOf(t, b))
	}
}

func TestGenerateCacheHash_IgnoresRequestId(t *testing.T) {
	a := sampleRequest()
	b := sampleRequest()
	b.RequestId = "a-completely-different-request-id"

	if hashOf(t, a) != hashOf(t, b) {
		t.Fatalf("RequestId must not affect the cache hash, but hashes differ:\n a=%s\n b=%s",
			hashOf(t, a), hashOf(t, b))
	}
}

func TestGenerateCacheHash_DistinguishesRealFields(t *testing.T) {
	base := hashOf(t, sampleRequest())

	cases := map[string]func(*requestV1.BuildRequest){
		"race":        func(r *requestV1.BuildRequest) { r.Race = true },
		"debug":       func(r *requestV1.BuildRequest) { r.Debug = true },
		"rr_version":  func(r *requestV1.BuildRequest) { r.RrVersion = "v3.0.0" },
		"platform_os": func(r *requestV1.BuildRequest) { r.TargetPlatform.Os = "darwin" },
		"plugin_tag":  func(r *requestV1.BuildRequest) { r.Plugins[0].Tag = "v6.9.9" },
		"added_exclude": func(r *requestV1.BuildRequest) {
			r.Excludes = append(r.Excludes, &requestV1.Exclude{Module: "github.com/x/y", Version: "v1.2.3"})
		},
		"replace_new_path": func(r *requestV1.BuildRequest) { r.Replaces[0].New = "../somewhere-else" },
	}

	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			req := sampleRequest()
			mutate(req)
			if got := hashOf(t, req); got == base {
				t.Fatalf("changing %s must change the hash, but it stayed %s", name, base)
			}
		})
	}
}
