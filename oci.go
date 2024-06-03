package oci

import (
	"context"
	"fmt"
	// "net/http"

	// "os"
	"time"

	// _ "crypto/sha256"

	// "github.com/opencontainers/go-digest"
	// "oras.land/oras-go/v2/registry"
	oras "oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/content/memory"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	// "oras.land/oras-go/v2/registry/remote/credentials"
	"oras.land/oras-go/v2/registry/remote/retry"
	// ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	numRetries = 10
	latestTag  = "{latest}"
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

	pulled        bool
	lastPull      time.Time
	loginRequired bool
}

func (a *Artifact) Pull(c context.Context) error {

	// If the artifact has already been pulled and the interval has not passed, return
	if time.Since(a.lastPull) < a.Interval {
		// log.Debug("artifact has already been pulled")
		return nil
	}

	// Create a new context with a timeout of 10 seconds
	ctx, cancel := context.WithTimeout(c, 30*time.Second)
	defer cancel()

	retryCount := 0
	maxRetries := numRetries

	var err error

	for {
		select {
		case <-ctx.Done():

			// If the context is done, return the context's error
			return ctx.Err()

		default:

			// Try to pull the artifact
			err = a.pull(ctx)

			if err == nil {

				// Update the last pull time
				a.lastPull = time.Now()
				a.pulled = true

				// If the pull was successful, return nil
				return nil
			} else {
				log.Errorf("failed to pull artifact %s:%s from %s: %v\n", a.Repository, a.Reference, a.Registry, err)
			}

			// If the pull failed, increment the retry count
			retryCount++
			if retryCount >= maxRetries {
				// If the maximum number of retries has been reached, cancel the context and return the error
				cancel()
				return fmt.Errorf("maximum number of retries reached: %w", err)
			}

			// If the pull failed and the maximum number of retries has not been reached, sleep for a while before retrying
			time.Sleep(time.Second)
		}
	}
}

func (a *Artifact) pull(c context.Context) error {

	log.Infof("pulling artifact %s:%s from %s\n", a.Repository, a.Reference, a.Registry)

	// create a temporary memory store
	memoryStore := memory.New()

	// Copy the artifact from the remote repository to the memory store
	desc, err := oras.Copy(c, a.remote, a.Reference, memoryStore, a.Reference, oras.DefaultCopyOptions)
	if err != nil {
		return err
	}

	log.Debugf("copied artifact %s:%s to memory store\n", a.Repository, a.Reference)

	// Create a new file store
	a.fs, err = file.New(a.Path)
	if err != nil {
		return err
	}

	log.Debugf("created file store at %s\n", a.Path)
	defer a.fs.Close()

	// Copy the artifact from the memory store to the file store
	desc, err = oras.Copy(c, memoryStore, a.Reference, a.fs, a.Reference, oras.DefaultCopyOptions)
	if err != nil {
		log.Errorf("%s:%s: %v\n", a.Repository, a.Reference, err)
		return err
	}

	// Check if the copy from memory store to file store was successful
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

	r.HandleWarning = func(warning remote.Warning) {
		log.Infof("Warning from %s: %s\n", r.Reference.Repository, warning.Text)
	}

	a.remote = r

	a.Registry = a.remote.Reference.Registry
	a.Repository = a.remote.Reference.Repository
	a.Reference = a.remote.Reference.Reference

	return nil
}

func (a *Artifact) Login(c context.Context) error {

	if a.loginRequired == false {
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

	return nil // return nil if the artifact was successfully prepared
}
