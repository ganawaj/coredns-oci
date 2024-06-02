package oci

import (
	"net"
	"net/http"
	"time"

	"oras.land/oras-go/v2/registry/remote/retry"
)

// override the default predicate to include 404 status code
var DefaultPredicate retry.Predicate = func(resp *http.Response, err error) (bool, error) {
	if err != nil {

		if err, ok := err.(net.Error); ok && err.Timeout() {
			return true, nil
		}
		return false, err
	}

	if resp.StatusCode == http.StatusRequestTimeout || resp.StatusCode == http.StatusTooManyRequests {
		return true, nil
	}

	if resp.StatusCode == 0 || resp.StatusCode >= 500 {
		return true, nil
	}

	if resp.StatusCode == http.StatusNotFound {
		return true, nil
	}

	return false, nil
}

// DefaultPolicy is the default retry policy for OCI operations
var DefaultPolicy retry.Policy = &retry.GenericPolicy{
	Retryable: DefaultPredicate,
	Backoff:   retry.DefaultBackoff,
	MinWait:   200 * time.Millisecond,
	MaxWait:   3 * time.Second,
	MaxRetry:  3,
}


