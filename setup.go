package oci

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"oras.land/oras-go/v2/registry/remote/auth"
)

var log = clog.NewWithPlugin("oci")

const (

	// DefaultInterval is the minimum interval to delay before
	// requesting another oci fetch
	DefaultInterval time.Duration = time.Hour
)

func init() { plugin.Register("oci", setup) }

func setup(c *caddy.Controller) error {

	// parse Corefile arguments and create a new OCI object
	oci, err := parse(c)
	if err != nil {
		return err
	}

	ctx := context.Background()

	var startupFuncs []func() error // functions to execute at startup

	// loop through all repos and and start monitoring
	for i := range oci {
		repo := oci.Artifact(i)

		startupFuncs = append(startupFuncs, func() error {

			// Start service routine in background
			Start(repo, ctx)

			return nil

		})
	}

	// ensure the functions are executed once per server block
	// for cases like server1.com, server2.com { ... }
	c.OncePerServerBlock(func() error {
		for i := range startupFuncs {
			c.OnStartup(startupFuncs[i])
		}
		return nil
	})

	return nil
}

// parse parses the Corefile and returns an OCI object
func parse(c *caddy.Controller) (OCI, error) {

	var oci OCI

	config := dnsserver.GetConfig(c)

	for c.Next() {

		repo := &Artifact{Interval: DefaultInterval, Path: config.Root, loginRequired: false}
		cred := auth.EmptyCredential // create an empty credential

		args := c.RemainingArgs()

		fetchPath := func(s string) string {
			if filepath.IsAbs(s) {
				return filepath.Clean(s)
			}
			return filepath.Join(config.Root, s)
		}

		switch len(args) {
		case 2:
			repo.Path = fetchPath(args[1])
			fallthrough
		case 1:
			repo.URL = args[0]
		}

		for c.NextBlock() {
			switch c.Val() {
			case "url":
				if !c.NextArg() {
					return nil, plugin.Error("oci", c.ArgErr())
				}
				repo.URL = c.Val()
			case "interval":
				if !c.NextArg() {
					return nil, plugin.Error("oci", c.ArgErr())
				}
				t, _ := strconv.Atoi(c.Val())
				if t > 0 {
					repo.Interval = time.Duration(t) * time.Second
				}
			case "path":
				if !c.NextArg() {
					return nil, plugin.Error("git", c.ArgErr())
				}
				repo.Path = fetchPath(c.Val())

			case "username":
				if !c.NextArg() {
					return nil, plugin.Error("oci", c.ArgErr())
				}
				cred.Username = c.Val()

			case "password":
				if !c.NextArg() {
					return nil, plugin.Error("oci", c.ArgErr())
				}
				cred.Password = c.Val()

			default:
				return nil, plugin.Error("oci", c.ArgErr())
			}
		}

		// if repo is not specified, return error
		if repo.URL == "" {
			return nil, plugin.Error("oci", fmt.Errorf("no URL set"))
		}

		// if path is not specified, return error
		if repo.Path == "" {
			return nil, plugin.Error("oci", fmt.Errorf("no path set"))
		}

		// if username and password are not set, set loginRequired to false
		if cred != auth.EmptyCredential {
			repo.Credential = cred
			repo.loginRequired = true
		}

		// prepare repo for use
		if err := repo.Prepare(); err != nil {
			return nil, plugin.Error("oci", err)
		}

		oci = append(oci, repo)
	}

	return oci, nil
}
