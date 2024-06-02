package oci

import (
	"context"
)

func Start(a *Artifact, ctx context.Context) {

	go func(a *Artifact) {
		for {

			err := a.Pull(ctx)

			if err != nil {
				log.Error(err)
				return
			}

		}
	}(a)
}
