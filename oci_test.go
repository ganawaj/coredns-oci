package oci

import (
	// "context"
	"testing"

	"github.com/coredns/caddy"
	"oras.land/oras-go/v2/registry/remote/auth"
)

// func TestLogin(t *testing.T) {
// 	tests := []struct {
// 		input     string
// 		shouldErr bool
// 		expected	*Artifact
// 		}{
// 			// test docker oci registry
// 			// lets also test username and password parsing
// 			{`oci registry-1.docker.io/ganawaj/demo:0.0.1 {
// 				path /tmp/git1
// 				username user
// 				password DCKR_PAT_sdfsdfasdfasdfasdf
// 			}`, false, &Artifact{
// 				URL:  "registry-1.docker.io/ganawaj/demo:0.0.1",
// 				Path: "/tmp/git1",
// 				Credential: auth.Credential{
// 					Username: "user",
// 					Password: "DCKR_PAT_sdfsdfasdfasdfasdf",
// 				},
// 			}},

// 			// test ghcr oci registry
// 			// lets also test username and password parsing
// 			{`oci ghcr.io/ganawaj/coredns:0.0.1 {
// 				path /tmp/git1
// 				username user
// 				password GHCR_PAT_sdfsdfasdfasdfasdf
// 			}`, false, &Artifact{
// 				URL:  "ghcr.io/ganawaj/coredns:0.0.1",
// 				Path: "/tmp/git1",
// 				Credential: auth.Credential{
// 					Username: "user",
// 					Password: "GHCR_PAT_sdfsdfasdfasdfasdf",
// 				},
// 			}},

// 			// test no credentials are provided
// 			{`oci ghcr.io/ganawaj/coredns:0.0.1 {
// 						path /tmp/git1
// 			}`, false, &Artifact{
// 				URL:  "ghcr.io/ganawaj/coredns:0.0.1",
// 				Path: "/tmp/git1",
// 				Credential: auth.Credential{},
// 			}},
// 		}

// 		for i, test := range tests {

// 			c := caddy.NewTestController("dns", test.input)

// 			oci, _ := parse(c)

// 			ctx := context.Background()

// 			for _, repo := range oci {

// 				err := repo.Login(ctx)

// 				if !test.shouldErr && err != nil {
// 					t.Errorf("Test %v should not error but found %v", i, err)
// 					continue
// 				}

// 				if test.shouldErr && err == nil {
// 					t.Errorf("Test %v should error but found nil", i)
// 					continue
// 				}

// 				if repo.
// 			}

// 		}
// 	}

func TestOCIParse(t *testing.T) {
	tests := []struct {
		input     string
		shouldErr bool
		expected  *Artifact
	}{
		// test docker oci registry
		// lets also test username and password parsing
		{`oci registry-1.docker.io/ganawaj/demo:0.0.1 {
			path /tmp/git1
			username user
			password DCKR_PAT_sdfsdfasdfasdfasdf
		}`, false, &Artifact{
			URL:  "registry-1.docker.io/ganawaj/demo:0.0.1",
			Path: "/tmp/git1",
			Credential: auth.Credential{
				Username: "user",
				Password: "DCKR_PAT_sdfsdfasdfasdfasdf",
			},
		}},

		// test ghcr oci registry
		// lets also test username and password parsing
		{`oci ghcr.io/ganawaj/coredns:0.0.1 {
			path /tmp/git1
			username user
			password GHCR_PAT_sdfsdfasdfasdfasdf
		}`, false, &Artifact{
			URL:  "ghcr.io/ganawaj/coredns:0.0.1",
			Path: "/tmp/git1",
			Credential: auth.Credential{
				Username: "user",
				Password: "GHCR_PAT_sdfsdfasdfasdfasdf",
			},
		}},

		// test no credentials are provided
		{`oci ghcr.io/ganawaj/coredns:0.0.1 {
					path /tmp/git1
		}`, false, &Artifact{
			URL:        "ghcr.io/ganawaj/coredns:0.0.1",
			Path:       "/tmp/git1",
			Credential: auth.Credential{},
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

		for _, repo := range oci {
			if !reposEqual(test.expected, repo) {
				t.Errorf("Test %v expects %v but found %v", i, test.expected, repo)
			}
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
	if expected.Credential != (auth.Credential{}) && expected.Credential != repo.Credential {
		return false
	}
	return true
}
