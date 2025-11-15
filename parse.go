package oci

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/miekg/dns"
)

// parseZoneMetadata parses the OCI config descriptor as JSON.
func parseZoneMetadata(data []byte) (*ZoneMetadata, error) {
	var metadata ZoneMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse zone metadata: %w", err)
	}
	return &metadata, nil
}

// parseZoneFile parses an RFC 1035 zone file into a Zone structure.
// The metadata parameter is optional and provides additional context.
func parseZoneFile(data []byte, origin string, metadata *ZoneMetadata) (*Zone, error) {
	// Ensure origin is fully qualified
	if !strings.HasSuffix(origin, ".") {
		origin = origin + "."
	}

	zone := &Zone{
		Origin:   origin,
		Records:  make(map[string][]dns.RR),
		Metadata: metadata,
		LoadedAt: time.Now(),
	}

	// Parse the zone file using miekg/dns
	zp := dns.NewZoneParser(strings.NewReader(string(data)), origin, "")

	var recordCount int
	for rr, ok := zp.Next(); ok; rr, ok = zp.Next() {
		if rr == nil {
			continue
		}

		// Extract SOA record
		if soa, ok := rr.(*dns.SOA); ok {
			zone.SOA = soa
			zone.Serial = soa.Serial
		}

		// Index record by its fully qualified name
		name := rr.Header().Name
		zone.Records[name] = append(zone.Records[name], rr)
		recordCount++
	}

	// Check for parsing errors
	if err := zp.Err(); err != nil {
		return nil, fmt.Errorf("zone file parse error: %w", err)
	}

	// Validate that SOA exists
	if zone.SOA == nil {
		return nil, fmt.Errorf("zone file missing SOA record for %s", origin)
	}

	// Update serial from SOA if not already set
	if zone.Serial == 0 {
		zone.Serial = zone.SOA.Serial
	}

	return zone, nil
}

// extractOrigin determines the zone origin from metadata or falls back to parsing.
func extractOrigin(metadata *ZoneMetadata, fallback string) string {
	if metadata != nil && metadata.Zone.Origin != "" {
		origin := metadata.Zone.Origin
		if !strings.HasSuffix(origin, ".") {
			origin = origin + "."
		}
		return origin
	}

	// Use fallback, ensuring it's fully qualified
	if !strings.HasSuffix(fallback, ".") {
		fallback = fallback + "."
	}
	return fallback
}

// validateZone performs additional validation on a parsed zone.
func validateZone(zone *Zone) error {
	if zone == nil {
		return fmt.Errorf("zone is nil")
	}

	if zone.Origin == "" {
		return fmt.Errorf("zone origin is empty")
	}

	if !strings.HasSuffix(zone.Origin, ".") {
		return fmt.Errorf("zone origin must be fully qualified: %s", zone.Origin)
	}

	if zone.SOA == nil {
		return fmt.Errorf("zone missing SOA record")
	}

	if len(zone.Records) == 0 {
		return fmt.Errorf("zone has no records")
	}

	// Verify SOA matches origin
	if zone.SOA.Header().Name != zone.Origin {
		return fmt.Errorf("SOA name %s does not match zone origin %s", zone.SOA.Header().Name, zone.Origin)
	}

	return nil
}
