package oci

import (
	"context"
	"fmt"

	"time"

	oras "oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/retry"
	"github.com/cenkalti/backoff/v5"

)

type OCI []*Artifact

func (o OCI) Artifact(i int) *Artifact {
	if i < len(o) {
		return o[i]
	}
	return nil
}

type Artifact struct {
	URL      string
	Interval time.Duration
	Path     string

	// file store for the artifact
	fs *file.Store

	// remote registry
	remote *remote.Repository

	// registry, repository, and reference of the artifact
	Registry   string
	Repository string

	// tag or digest of the artifact
	// see https://pkg.go.dev/oras.land/oras-go/v2@v2.5.0/registry#ParseReference
	// for the format of the reference
	Reference string

	// credentials
	Credential auth.Credential
	loginRequired bool
	insecure bool
}

// Pulls the artifact from the remote repository.
//
// The artifact is copied from the remote repository to a memory store and then to a file store.
func (a *Artifact) Pull(c context.Context) error {

	log.Infof("pulling artifact %s:%s from %s\n", a.Repository, a.Reference, a.Registry)

	// Create a new file store
	var fsErr error
	a.fs, fsErr = file.New(a.Path)
	if fsErr != nil {
		log.Errorf("failed to create file store: %v\n", fsErr)
		return backoff.Permanent(fsErr)
	}

	defer a.fs.Close()

	// Copy the artifact from the remote repository to the memory store
	desc, err := oras.Copy(c, a.remote, a.Reference, a.fs, a.Reference, oras.DefaultCopyOptions)
	if err != nil {
		log.Errorf("failed to copy artifact %s:%s from %s: %v\n", a.Repository, a.Reference, a.Registry, err)
		return err
	}

	// Check if the copy to file store was successful
	exists, err := a.fs.Exists(c, desc)
	if exists && err == nil {
		log.Infof("pulled artifact %s:%s with %s from %s\n", a.Repository, a.Reference, desc.Digest, a.Registry)
		return nil
	}

	return err // return the error if the pull was not successful

}

// Parses and creates the remote repository based on the URL
func (a *Artifact) Setup() error {

	// Create a new remote repository
	r, err := remote.NewRepository(a.URL)
	if err != nil {
		return err
	}

	if r.Reference.Reference == "" {
		log.Debugf("no reference specified, using latest\n")
		r, err = remote.NewRepository(fmt.Sprintf("%s:latest", a.URL))
		if err != nil {
			return err
		}
	}

	// show the user the warnings from the repo if any occur
	r.HandleWarning = func(warning remote.Warning) {
		log.Infof("Warning from %s: %s\n", r.Reference.Repository, warning.Text)
	}

	a.remote = r

	// Define reference for ease of use
	a.Registry = a.remote.Reference.Registry
	a.Repository = a.remote.Reference.Repository
	a.Reference = a.remote.Reference.Reference

	return nil
}

// Logs in to the registry if required
func (a *Artifact) Login(c context.Context) error {

	if !a.loginRequired {
		return nil
	}

	a.remote.Client = &auth.Client{
		Client:     retry.DefaultClient,
		Cache:      auth.NewCache(),
		Credential: auth.StaticCredential(a.Registry, a.Credential),
	}

	log.Infof("logged in to %s as %s\n", a.Registry, a.Credential.Username)

	return nil
}

// Prepare prepares the artifact for use
func (a *Artifact) Prepare() error {

	// Setup the artifact
	if err := a.Setup(); err != nil {
		return err
	}

	// Login to the registry if required
	if a.loginRequired {
		err := a.Login(context.Background())
		if err != nil {
			return err
		}
	}

  // Set the remote repository to use plain HTTP
	if a.insecure {
		a.remote.PlainHTTP = true
	}

	return nil
}
