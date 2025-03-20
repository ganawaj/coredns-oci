package oci

import (
	"context"
	"time"

	b "github.com/cenkalti/backoff/v5"
)

var (
	// DefaultDeadline is the maximum time allowed for a single pull operation
	DefaultDeadline = 60 * time.Second
	// DefaultRetryInterval is the time between retry attempts
	DefaultRetryInterval = 10 * time.Second
	// DefaultRetryCount is the maximum number of retry attempts
	DefaultRetryCount = 3
)

// Todo: the interval shouldn't be lower than the sum of the retry interval and the deadline

// Start starts the artifact pull process in the background.
//
// To attempt to respect the rate limits of the registry:
// - the pull operation has 60(DefaultDeadline) seconds to complete.
// - the pull operation will retry every 10 seconds.
// - the pull operation will try no more than 3 times.
// The next pull operation is scheduled after a successful pull at the interval specified in the artifact.
func Start(a *Artifact, ctx context.Context) {
	// Start the background process
	go func() {
		// Pull the artifact immediately
		PullWithRetry(ctx, a)

		ticker := time.NewTicker(a.Interval)
		defer ticker.Stop()

		// Schedule the next pull operation
		for {
			select {
			case <-ticker.C:
				PullWithRetry(ctx, a)
			case <-ctx.Done():
				return // context is cancelled
			}
		}
	}()
}

func PullWithRetry(ctx context.Context, a *Artifact) {
	start := time.Now()

	// Create a new context with a timeout of DefaultDeadline seconds
	log.Debugf("creating context with timeout of %s seconds for artifact %s", DefaultDeadline, a.Reference())
	pullCtx, cancel := context.WithTimeout(ctx, DefaultDeadline)
	defer cancel()

	operation := func() (string, error) {
		err := a.Pull(pullCtx)
		return "", err
	}

	// Pull the artifact with retry and backoff
	_, err := b.Retry(pullCtx, operation,
		b.WithBackOff(b.NewConstantBackOff(DefaultRetryInterval)),
		b.WithMaxTries(uint(DefaultRetryCount)),
	)

	if err != nil {
		log.Errorf("Failed to pull artifact %s: %v", a.Reference(), err)
		return
	}

	log.Infof("Successfully pulled artifact %s/%s:%s in %s", a.Registry(), a.Repository(), a.Reference(), time.Since(start))
}
