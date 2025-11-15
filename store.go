package oci

import (
	"sync"
	"time"

	"github.com/miekg/dns"
)

// Zone represents a parsed DNS zone loaded from an OCI artifact.
type Zone struct {
	// Origin is the zone origin (e.g., "example.org.")
	Origin string

	// Records maps FQDNs to their DNS records for O(1) lookup
	Records map[string][]dns.RR

	// SOA is the zone's SOA record
	SOA *dns.SOA

	// Metadata from the OCI config descriptor
	Metadata *ZoneMetadata

	// Digest is the OCI artifact digest for version tracking
	Digest string

	// Serial is the SOA serial number
	Serial uint32

	// LoadedAt is when this zone was loaded
	LoadedAt time.Time
}

// Lookup finds records matching the given name and type in the zone.
// Returns nil if no records are found.
func (z *Zone) Lookup(name string, qtype uint16) []dns.RR {
	records, ok := z.Records[name]
	if !ok {
		return nil
	}

	// If ANY query, return all records
	if qtype == dns.TypeANY {
		return records
	}

	// Filter by type
	var result []dns.RR
	for _, rr := range records {
		if rr.Header().Rrtype == qtype {
			result = append(result, rr)
		}
	}
	return result
}

// HasName checks if the zone has any records for the given name.
func (z *Zone) HasName(name string) bool {
	_, ok := z.Records[name]
	return ok
}

// ZoneStore provides thread-safe storage for DNS zones.
type ZoneStore struct {
	mu    sync.RWMutex
	zones map[string]*Zone
}

// NewZoneStore creates a new empty zone store.
func NewZoneStore() *ZoneStore {
	return &ZoneStore{
		zones: make(map[string]*Zone),
	}
}

// Update atomically replaces or adds a zone in the store.
func (s *ZoneStore) Update(origin string, zone *Zone) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.zones[origin] = zone
}

// Get retrieves a zone by origin. Returns nil if not found.
func (s *ZoneStore) Get(origin string) *Zone {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.zones[origin]
}

// List returns all zone origins in the store.
func (s *ZoneStore) List() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	origins := make([]string, 0, len(s.zones))
	for origin := range s.zones {
		origins = append(origins, origin)
	}
	return origins
}

// Remove deletes a zone from the store.
func (s *ZoneStore) Remove(origin string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.zones, origin)
}

// Count returns the number of zones in the store.
func (s *ZoneStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.zones)
}
