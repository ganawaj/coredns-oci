package oci

import (
	"context"
	b "github.com/cenkalti/backoff/v5"
	"time"
)

var (
	DefaltDeadline       = 60 * time.Second
	DefaultRetryInterval = 10 * time.Second
	DefaultRetryCount    = 3
)

// Start starts the artifact pull process in the background.
//
// To attempt to respect the rate limits of the registry:
// - the pull operation has 60(DefaltDeadline) seconds to complete.
// - the pull operation will retry every 10 seconds.
// - the pull operation will try no more than 3 times.
// The next pull operation is scheduled after a successful pull at the interval specified in the artifact.
func Start(a *Artifact, ctx context.Context) {

	// Pull the artifact immediately
	go PullWithRetry(ctx, a)

	// Schedule the next pull operation
	for {
		select {

		case <-time.After(a.Interval):
			go PullWithRetry(ctx, a)

		case <-ctx.Done():
			return // context is cancelled
		}
	}
}

func PullWithRetry(ctx context.Context, a *Artifact) {

	// Create a new context with a timeout of DefaltDeadline seconds
	log.Debugf("creating context with timeout of %s seconds for artifact %s\n", DefaltDeadline, a.Reference)
	pullCtx, cancel := context.WithTimeout(ctx, DefaltDeadline)
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
		log.Errorf("failed to pull artifact %s: %v\n", a.Reference, err)
		return
	}

	log.Infof("successfully pulled artifact %s\n", a.Reference)

}
