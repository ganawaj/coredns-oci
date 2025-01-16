package oci

import (
	"context"
	"fmt"
	"os"
	"time"

	"testing"

	"github.com/coredns/caddy"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote/auth"
)

func TestArtifactPull(t *testing.T) {

	path, err := os.MkdirTemp("", "sampledir")
	defer os.RemoveAll(path)
  if err != nil {
		t.Errorf("Error creating temp directory: %v", err)
	}

	given := []struct {
		input     string
		shouldErr bool
	}{
		{
			fmt.Sprintf(
			`oci ghcr.io/ganawaj/demo:0.0.2 {
						path %s
		 		}`, path),
			false,
		},
		// should create the directory if it does not exist
		{`oci ghcr.io/ganawaj/demo:0.0.2 {
				path nonexistent_path
	 		}`,false,
		},
		// should error on permission denied
		{`oci ghcr.io/ganawaj/demo:0.0.2 {
			path /test
		 }`,true,
	},
	}

	for _, test := range given {

		c := caddy.NewTestController("dns", test.input)
		oci, err := parse(c)

		if err != nil {
			t.Errorf("Test should not error but found %v", err)
		}

		repo := oci.Artifact(0)

		ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
		defer cancel()
		err = repo.Pull(ctx)
		if err != nil && !test.shouldErr {
			t.Errorf("Test should not error but found %v", err)
		}

		// Check if the artifact is pulled
		if _, err := os.Stat(repo.Path); os.IsNotExist(err) && !test.shouldErr {
			t.Errorf("Test should pull the artifact but found %v", err)
		}

	}
}

func TestReferenceParse(t *testing.T) {

	given := []struct {
		input     string
		shouldErr bool
		expected  registry.Reference
	}{
		{`oci registry-1.docker.io/ganawaj/demo:0.0.1 example`,
			false,
			registry.Reference{
				Registry:   "registry-1.docker.io",
				Repository: "ganawaj/demo",
				Reference:  "0.0.1",
			},
		},
		{`oci ghcr.io/ganawaj/demo:0.0.2 example`,
			false,
			registry.Reference{
				Registry:   "ghcr.io",
				Repository: "ganawaj/demo",
				Reference:  "0.0.2",
			},
		},
		{`oci localhost:5000/ganawaj/demo:v1.9.3 example`,
			false,
			registry.Reference{
				Registry:   "localhost:5000",
				Repository: "ganawaj/demo",
				Reference:  "v1.9.3",
			},
		},
		{`oci localhost:5000/ganawaj/demo example`,
			false,
			registry.Reference{
				Registry:   "localhost:5000",
				Repository: "ganawaj/demo",
				Reference:  "latest",
			},
		},
	}

	for i, test := range given {

		c := caddy.NewTestController("dns", test.input)
		repo, err := parse(c)

		if !test.shouldErr && err != nil {
			t.Errorf("Test %v should not error but found %v", i, err)
			continue
		}

		if test.shouldErr && err == nil {
			t.Errorf("Test %v should error but found nil", i)
			continue
		}

		a := repo.Artifact(0)
		if a.remote.Reference != test.expected {
			t.Errorf("Test %v expects %v but found %v", i, test.expected, a.Reference)
		}
	}
}

func TestLogin(t *testing.T) {

	given := []struct {
		input     string
		shouldErr bool
	}{
		{`oci registry-1.docker.io/ganawaj/demo:0.0.1 {
				path /tmp/git1
				 username user
				 password DCKR_PAT_sdfsdfasdfasdfasdf
			 }`,
			false,
		},
		{`oci registry-1.docker.io/ganawaj/demo:0.0.1 {
				path /tmp/git1
				 username user
			 }`,
			true,
		},
		{`oci registry-1.docker.io/ganawaj/demo:0.0.1 {
				path /tmp/git1
				password DCKR_PAT_sdfsdfasdfasdfasdf
			 }`,
			true,
		},
	}

	for i, test := range given {

		c := caddy.NewTestController("dns", test.input)
		_, err := parse(c)

		if !test.shouldErr && err != nil {
			t.Errorf("Test %v should not error but found %v", i, err)
			continue
		}

		if test.shouldErr && err == nil {
			t.Errorf("Test %v should error but found nil", i)
			continue
		}

	}
}

func TestLoginRequired(t *testing.T) {

	given := []struct {
		input     string
		shouldErr bool
	}{
		{`oci registry-1.docker.io/ganawaj/demo:0.0.1 {
						path /tmp/git1
		 				username user
		 				password DCKR_PAT_sdfsdfasdfasdfasdf
		 			}`,
			false,
		},
	}

	for _, test := range given {

		c := caddy.NewTestController("dns", test.input)
		oci, err := parse(c)

		if err != nil && !test.shouldErr {
			t.Errorf("Test should not error but found %v", err)
		}

		repo := oci.Artifact(0)
		if !repo.loginRequired {
			t.Errorf("Test should require login but found false")
		}

	}
}

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
