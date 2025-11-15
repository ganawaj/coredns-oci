package oci

import (
	"context"
	"sync"
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/fall"
	clog "github.com/coredns/coredns/plugin/pkg/log"
)

var log = clog.NewWithPlugin("oci")

// OCI is the main plugin struct that implements the CoreDNS plugin.Handler interface.
type OCI struct {
	// Next is the next plugin in the chain
	Next plugin.Handler

	// zones is the list of zone origins this plugin is authoritative for
	zones []string

	// Configuration
	repo          string
	username      string
	password      string
	interval      time.Duration
	requireSigned bool

	// Runtime state
	store         *ZoneStore
	currentDigest string

	// Fallthrough configuration
	Fall fall.F

	// Lifecycle management
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// Name returns the name of this plugin as it appears in metrics and logs.
func (o *OCI) Name() string {
	return "oci"
}

// Ready returns true if the plugin has loaded at least one zone.
func (o *OCI) Ready() bool {
	return o.store != nil && o.store.Count() > 0
}

// init initializes the plugin by performing the initial pull.
func (o *OCI) init() error {
	// Initialize metrics
	initMetrics()

	// Initialize the zone store
	o.store = NewZoneStore()

	// Create context for lifecycle management
	o.ctx, o.cancel = context.WithCancel(context.Background())

	// Perform initial pull with a timeout
	initCtx, cancel := context.WithTimeout(o.ctx, 60*time.Second)
	defer cancel()

	if err := o.pullAndParse(initCtx); err != nil {
		return err
	}

	// Start background update loop
	o.wg.Add(1)
	go o.updateLoop()

	return nil
}

// updateLoop periodically checks for and pulls new zone versions.
func (o *OCI) updateLoop() {
	defer o.wg.Done()

	if o.interval <= 0 {
		log.Info("Background updates disabled (interval <= 0)")
		return
	}

	ticker := time.NewTicker(o.interval)
	defer ticker.Stop()

	log.Infof("Started background update loop with interval %s for %s", o.interval, o.repo)

	for {
		select {
		case <-o.ctx.Done():
			log.Info("Stopping background update loop")
			return
		case <-ticker.C:
			log.Debugf("Checking for zone updates for %s", o.repo)

			pullCtx, cancel := context.WithTimeout(o.ctx, 60*time.Second)
			if err := o.pullAndParse(pullCtx); err != nil {
				log.Errorf("Failed to pull zone update: %v", err)
				// Continue serving stale data
			}
			cancel()
		}
	}
}

// shutdown gracefully stops the plugin.
func (o *OCI) shutdown() error {
	if o.cancel != nil {
		o.cancel()
	}
	o.wg.Wait()
	return nil
}
