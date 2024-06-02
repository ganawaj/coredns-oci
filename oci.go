package oci

import (
	"context"
	"fmt"
	"net/http"

	// "os"
	"time"

	// _ "crypto/sha256"

	// "github.com/opencontainers/go-digest"
	// "oras.land/oras-go/v2/registry"
	oras "oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
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
	fs 				*file.Store

	// remote registry
	remote *remote.Repository

	// registry, repository, and reference of the artifact
	Registry   string
	Repository string

	// tag or digest of the artifact
	// see https://pkg.go.dev/oras.land/oras-go/v2@v2.5.0/registry#ParseReference
	// for the format of the reference
	Reference string

	pulled   bool
	lastPull time.Time
}

func (a *Artifact) Pull(c context.Context) error {

	// If the artifact has already been pulled and the interval has not passed, return
	if time.Since(a.lastPull) < a.Interval {
		return nil
	}

	// Create a new context with a timeout of 10 seconds
	// TODO: Make this adjustable
	ctx, cancel := context.WithTimeout(c, 10*time.Second)
	defer cancel()

	retries := make(chan int)
	done := make(chan error)

	var err error

	// start a goroutine to pull the artifact
	go a.pull(ctx, retries, done)

	// wait for the artifact to be pulled
	for {
		select {
		case <-ctx.Done():
			switch ctx.Err() {

			// If the context was canceled or exceeded the deadline, return nil
			case context.DeadlineExceeded:
				return fmt.Errorf("failed to pull artifact %v:%v - %v", a.Repository, a.Reference, ctx.Err())

			// If the context was canceled, return nil
			// We don't care about the error since it would be caught by the done channel
			case context.Canceled:
				return nil
			}

		// If the number of retries exceeds the max number of retries, return an error
		// and cancel the context
		case i := <-retries:
			if i >= numRetries {
				cancel()
				return fmt.Errorf("failed to pull artifact %v:%v - max retries exceeded", a.Repository, a.Reference)
			}

		// If the artifact was pulled successfully, return nil
		// and cancel the context
		case err = <-done:
			if err != nil {
				return fmt.Errorf("failed to pull artifact %v:%v - %v", a.Repository, a.Reference, err)
			}
			cancel()
			return nil

		}

	}

}

func (a *Artifact) pull(c context.Context, r chan int, done chan error) {

	log.Infof("pulling artifact %s:%s from %s\n", a.Repository, a.Reference, a.Registry)

	attempts := 1
	for {

		log.Infof("attempt %d: attempting pulling artifact %s:%s from %s\n", attempts, a.Repository, a.Reference, a.Registry)

		// Send the number of attempts to the retries channel
		r <- attempts

		desc, err := oras.Copy(c, a.remote, a.Reference, a.fs, a.Reference, oras.DefaultCopyOptions)
		if err != nil {
			log.Errorf("%s:%s: attempt %d - %v\n", a.Repository, a.Reference, attempts)
		}

		// Check if the pull was successful
		exists, err := a.fs.Exists(c, desc)
		if exists && err == nil{

			// Update the last pull time
			a.lastPull = time.Now()
			a.pulled = true

			// TODO: replace with log message
			log.Infof("pulled artifact %s:%s with %s from %s\n", a.Repository, a.Reference, desc.Digest, a.Registry)

			// Send a nil value to the done channel to indicate the pull was successful
			// TODO: should return the SHA of the pulled artifact and error
			done <- nil
			break // exit the loop
		}

		// For now, assume the pull was unsuccessful for the first 5 attempts
		// and then successful on the 6th attempt
		attempts++

		// renew the file store
		a.fs, err = file.New(a.Path)
		defer a.fs.Close()
	}
}

func (a *Artifact) Setup() error {


	// Create a new remote repository
	r, err := remote.NewRepository(a.URL)
	if err != nil {
		return err
	}

	a.remote = r

	a.Registry = a.remote.Reference.Registry
	a.Repository = a.remote.Reference.Repository
	a.Reference = a.remote.Reference.Reference

	return nil
}

func (a *Artifact) Prepare() error {

	// Setup the artifact
	if err := a.Setup(); err != nil {
		return err
	}

	log.Infof("creating file store for artifact %s:%s at %s\n", a.Repository, a.Reference, a.Path)

	// Create a new file store
	fs, err  := file.New(a.Path)
	if err != nil {
		// log.Infof("failed to create file store for artifact %s:%s at %s\n", a.Repository, a.Reference, a.Path)
		return err
	}
	a.fs = fs
	defer a.fs.Close()

	return nil

}
