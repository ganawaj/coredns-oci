package oci

import (
	"fmt"
	"strconv"
	"time"

	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
)

const (
	// DefaultInterval is the default interval between artifact pulls (1 hour)
	DefaultInterval = 3600 * time.Second
	// MinimumInterval is the minimum allowed interval between pulls (5 seconds)
	MinimumInterval = 5 * time.Second
)

func init() { plugin.Register("oci", setup) }

func setup(c *caddy.Controller) error {
	// Parse Corefile arguments and create a new OCI plugin instance
	o, err := parse(c)
	if err != nil {
		return plugin.Error("oci", err)
	}

	// Get the DNS server configuration
	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		o.Next = next
		return o
	})

	// Register startup function to initialize the plugin
	c.OnStartup(func() error {
		log.Infof("Starting OCI plugin for zones: %v", o.zones)
		if err := o.init(); err != nil {
			return plugin.Error("oci", fmt.Errorf("failed to initialize: %w", err))
		}
		return nil
	})

	// Register shutdown function
	c.OnShutdown(func() error {
		log.Info("Shutting down OCI plugin")
		return o.shutdown()
	})

	return nil
}

// parse parses the Corefile and returns an OCI plugin instance.
func parse(c *caddy.Controller) (*OCI, error) {
	config := dnsserver.GetConfig(c)

	// Create new OCI instance with defaults
	o := &OCI{
		interval: DefaultInterval,
		zones:    []string{config.Zone},
	}

	// There should be at least one token (the "oci" directive)
	if !c.Next() {
		return nil, c.ArgErr()
	}

	// Get remaining arguments on the same line
	args := c.RemainingArgs()
	if len(args) == 0 {
		return nil, fmt.Errorf("repository reference is required")
	}

	// First argument is the repository reference (required)
	o.repo = args[0]

	// Parse optional block configuration
	for c.NextBlock() {
		switch c.Val() {
		case "username":
			if !c.NextArg() {
				return nil, c.ArgErr()
			}
			o.username = c.Val()

		case "password":
			if !c.NextArg() {
				return nil, c.ArgErr()
			}
			o.password = c.Val()

		case "interval":
			if !c.NextArg() {
				return nil, c.ArgErr()
			}
			seconds, err := strconv.Atoi(c.Val())
			if err != nil {
				return nil, fmt.Errorf("invalid interval value: %w", err)
			}
			if seconds > 0 {
				o.interval = time.Duration(seconds) * time.Second
			}

		case "require_signed":
			o.requireSigned = true

		case "fallthrough":
			o.Fall.SetZonesFromArgs(c.RemainingArgs())

		default:
			return nil, fmt.Errorf("unknown property '%s'", c.Val())
		}
	}

	// Validate configuration
	if err := validateConfig(o); err != nil {
		return nil, err
	}

	return o, nil
}

// validateConfig validates the OCI plugin configuration.
func validateConfig(o *OCI) error {
	if o.repo == "" {
		return fmt.Errorf("repository reference is required")
	}

	// Validate that both username and password are provided if one is
	if (o.username == "" && o.password != "") || (o.username != "" && o.password == "") {
		return fmt.Errorf("both username and password must be provided")
	}

	// Validate interval
	if o.interval < MinimumInterval {
		log.Warningf("Interval %s is below minimum %s, using minimum", o.interval, MinimumInterval)
		o.interval = MinimumInterval
	}

	// Validate zones
	if len(o.zones) == 0 {
		return fmt.Errorf("no zones configured")
	}

	return nil
}
