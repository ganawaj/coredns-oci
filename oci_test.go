package oci

import (
	"testing"

	"github.com/coredns/caddy"
)

func TestOCIParse(t *testing.T) {
	tests := []struct {
		input     string
		shouldErr bool
		expected  *Artifact
	}{
		// test docker oci registry
		{`oci registry-1.docker.io/ganawaj/demo:0.0.1 {
			path /tmp/git1
		}`, false, &Artifact{
			URL:  "registry-1.docker.io/ganawaj/demo:0.0.1",
			Path: "/tmp/git1",
		}},

		// test ghcr oci registry
		{`oci ghcr.io/ganawaj/coredns:0.0.1 {
			path /tmp/git1
		}`, false, &Artifact{
			URL:  "ghcr.io/ganawaj/coredns:0.0.1",
			Path: "/tmp/git1",
		}},
	}

	for i, test := range tests {

		c := caddy.NewTestController("dns", test.input)

		oci, err := parse(c)

		if !test.shouldErr && err != nil {
			t.Errorf("Test %v should not error but found %v", i, err)
			continue
		}

		if test.shouldErr && err == nil {
			t.Errorf("Test %v should error but found nil", i)
			continue
		}

		repo := oci.Artifact(0)
		if !reposEqual(test.expected, repo) {
			t.Errorf("Test %v expects %v but found %v", i, test.expected, repo)
		}
	}
}

func reposEqual(expected, repo *Artifact) bool {
	if expected == nil {
		return repo == nil
	}
	if expected.Interval != 0 && expected.Interval != repo.Interval {
		return false
	}
	if expected.Path != "" && expected.Path != repo.Path {
		return false
	}
	if expected.URL != "" && expected.URL != repo.URL {
		return false
	}
	return true
}
