package oci

// ZoneMetadata represents the config descriptor for a DNS zone OCI artifact.
// This follows the DNS Zone File OCI Standard schema.
type ZoneMetadata struct {
	SchemaVersion string `json:"schemaVersion"`
	Kind          string `json:"kind"`
	Metadata      struct {
		Name        string `json:"name"`
		Version     string `json:"version"`
		Created     string `json:"created"`
		Author      string `json:"author,omitempty"`
		Description string `json:"description,omitempty"`
	} `json:"metadata"`
	Zone struct {
		Origin       string `json:"origin"`
		TTL          uint32 `json:"ttl,omitempty"`
		Serial       uint32 `json:"serial"`
		Refresh      uint32 `json:"refresh,omitempty"`
		Retry        uint32 `json:"retry,omitempty"`
		Expire       uint32 `json:"expire,omitempty"`
		Minimum      uint32 `json:"minimum,omitempty"`
		RecordCount  int    `json:"recordCount,omitempty"`
		DNSSECSigned bool   `json:"dnssecSigned"`
	} `json:"zone"`
	Validation struct {
		Syntax      string   `json:"syntax,omitempty"`
		Validated   bool     `json:"validated"`
		ValidatedAt string   `json:"validatedAt,omitempty"`
		Validator   string   `json:"validator,omitempty"`
		Warnings    []string `json:"warnings,omitempty"`
	} `json:"validation,omitempty"`
	Deployment struct {
		Environments    []string `json:"environments,omitempty"`
		RequiredPlugins []string `json:"requiredPlugins,omitempty"`
		Compatibility   struct {
			CoreDNS string `json:"coredns,omitempty"`
		} `json:"compatibility,omitempty"`
	} `json:"deployment,omitempty"`
}

// Media types for DNS zone OCI artifacts
const (
	// MediaTypeZoneConfig is the media type for the config descriptor
	MediaTypeZoneConfig = "application/vnd.dns.zonefile.config.v1+json"

	// MediaTypeZoneRFC1035 is the media type for RFC 1035 zone files
	MediaTypeZoneRFC1035 = "application/vnd.dns.zonefile.rfc1035.v1"

	// MediaTypeZoneRFC1035Gzip is the media type for compressed RFC 1035 zone files
	MediaTypeZoneRFC1035Gzip = "application/vnd.dns.zonefile.rfc1035.v1+gzip"

	// MediaTypeZoneArtifact is the artifact type for DNS zone artifacts
	MediaTypeZoneArtifact = "application/vnd.dns.zonefile.v1"
)
