package oci

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/coredns/coredns/plugin/pkg/log"
	oras "oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/retry"
)

var (
	// ErrInvalidArtifactIndex is returned when accessing an invalid artifact index
	ErrInvalidArtifactIndex = errors.New("invalid artifact index")
	// ErrEmptyURL is returned when the artifact URL is empty
	ErrEmptyURL = errors.New("empty artifact URL")
	// ErrEmptyPath is returned when the artifact path is empty
	ErrEmptyPath = errors.New("empty artifact path")
)

// OCI is a list of artifacts
type OCI []*Artifact

// Artifact returns the artifact at index i
func (o OCI) Artifact(i int) (*Artifact, error) {
	if i < 0 || i >= len(o) {
		return nil, fmt.Errorf("%w: index %d out of bounds [0,%d)", ErrInvalidArtifactIndex, i, len(o))
	}
	return o[i], nil
}

// Artifact represents a single OCI artifact with its configuration and state
type Artifact struct {
	URL      string
	Interval time.Duration
	Path     string

	// file store for the artifact
	fs *file.Store

	// remote registry
	remote *remote.Repository

	// credentials and connection settings
	Credential    auth.Credential
	loginRequired bool
	insecure      bool
}

// Registry returns the registry name from the remote reference
func (a *Artifact) Registry() string {
	if a.remote == nil || a.remote.Reference == nil {
		return ""
	}
	return a.remote.Reference.Registry
}

// Repository returns the repository name from the remote reference
func (a *Artifact) Repository() string {
	if a.remote == nil || a.remote.Reference == nil {
		return ""
	}
	return a.remote.Reference.Repository
}

// Reference returns the tag or digest from the remote reference
func (a *Artifact) Reference() string {
	if a.remote == nil || a.remote.Reference == nil {
		return ""
	}
	return a.remote.Reference.Reference
}

// Pull downloads the artifact from the remote repository to the local file system.
func (a *Artifact) Pull(ctx context.Context) error {
	if ctx == nil {
		return errors.New("context cannot be nil")
	}

	log.Infof("Pulling artifact from %s/%s:%s", a.Registry(), a.Repository(), a.Reference())

	// Create a new file store
	var fsErr error
	a.fs, fsErr = file.New(a.Path)
	if fsErr != nil {
		log.Errorf("Failed to create file store: %v", fsErr)
		return fmt.Errorf("failed to create file store: %w", fsErr)
	}
	defer a.fs.Close()

	// Copy the artifact from the remote repository to the file store
	desc, err := oras.Copy(ctx, a.remote, a.Reference(), a.fs, a.Reference(), oras.DefaultCopyOptions)
	if err != nil {
		log.Errorf("Failed to copy artifact %s/%s:%s: %v", a.Registry(), a.Repository(), a.Reference(), err)
		return fmt.Errorf("failed to copy artifact: %w", err)
	}

	// Verify the copy was successful
	exists, err := a.fs.Exists(ctx, desc)
	if err != nil {
		return fmt.Errorf("failed to verify artifact: %w", err)
	}
	if !exists {
		return errors.New("artifact not found after copy")
	}

	log.Infof("Successfully pulled artifact %s/%s:%s with digest %s", a.Registry(), a.Repository(), a.Reference(), desc.Digest)

	return nil
}

// Prepare initializes the artifact by setting up the repository configuration
// and performing authentication if credentials are provided.
func (a *Artifact) Prepare() error {
	// Create a new remote repository
	r, err := remote.NewRepository(a.URL)
	if err != nil {
		return fmt.Errorf("failed to create repository: %w", err)
	}

	if r.Reference.Reference == "" {
		log.Debugf("No reference specified for %s, using latest", a.URL)
		r, err = remote.NewRepository(fmt.Sprintf("%s:latest", a.URL))
		if err != nil {
			return fmt.Errorf("failed to create repository with latest tag: %w", err)
		}
	}

	r.HandleWarning = func(warning remote.Warning) {
		log.Warningf("Repository %s: %s", r.Reference.Repository, warning.Text)
	}

	a.remote = r

	// if credentials are provided, perform login
	if a.Credential != auth.EmptyCredential {
		// Use a timeout context for login
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := a.Login(ctx); err != nil {
			return fmt.Errorf("login failed: %w", err)
		}
	}

	if a.insecure {
		a.remote.PlainHTTP = true
		log.Warningf("Using insecure plain HTTP connection for %s", a.URL)
	}

	return nil
}

// Login authenticates with the registry if required.
func (a *Artifact) Login(ctx context.Context) error {
	if ctx == nil {
		return errors.New("context cannot be nil")
	}

	if !a.loginRequired {
		return nil
	}

	if a.Credential.Username == "" && a.Credential.Password == "" {
		return errors.New("credentials required but not provided")
	}

	a.remote.Client = &auth.Client{
		Client:     retry.DefaultClient,
		Cache:      auth.NewCache(),
		Credential: auth.StaticCredential(a.Registry(), a.Credential),
	}

	log.Infof("Successfully logged in to %s as %s", a.Registry(), a.Credential.Username)

	return nil
}
