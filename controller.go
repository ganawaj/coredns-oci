package oci

import (
	"context"
	"time"
)

func Start(a *Artifact, ctx context.Context) {

	// Login to registries if required
	if a.loginRequired {
		err := a.Login(ctx)
		if err != nil {
			log.Error(err)
		}
	}

	go func(a *Artifact) {
		for {

			err := a.Pull(ctx)

			if err != nil {
				log.Error(err)
				time.Sleep(DefaultInterval) // sleep for a while before retrying
				continue
			}

		}
	}(a)
}
