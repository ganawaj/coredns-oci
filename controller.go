package oci

import (
	"context"
	"time"
)

var (
	DefaultBackoff = time.Second * 30
)

// Start starts the artifact pull process in the background.
//
// To attempt to respect the rate limits of the registry, a backoff of 30 seconds is used between retries
// if the pull fails or returns any errors.
func Start(a *Artifact, ctx context.Context) {

	go func(a *Artifact) {
		for {

			err := a.Pull(ctx)

			if err != nil {
				log.Error(err)

				time.Sleep(DefaultBackoff) // sleep for a while before retrying
				
				continue
			}

		}
	}(a)
}
