package oci

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	"github.com/coredns/caddy"
	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"
	"github.com/miekg/dns"
)

func TestParseCorefileSimple(t *testing.T) {
	tests := []struct {
		input       string
		shouldErr   bool
		expectedErr string
	}{
		// Valid simple configuration
		{
			`oci registry.example.com/zones/example.org:latest`,
			false,
			"",
		},
		// Missing repository
		{
			`oci`,
			true,
			"repository reference is required",
		},
	}

	for i, tc := range tests {
		c := caddy.NewTestController("dns", tc.input)
		_, err := parse(c)

		if tc.shouldErr && err == nil {
			t.Errorf("Test %d: expected error but got none", i)
			continue
		}

		if !tc.shouldErr && err != nil {
			t.Errorf("Test %d: unexpected error: %v", i, err)
			continue
		}
	}
}

func TestParseCorefileAdvanced(t *testing.T) {
	tests := []struct {
		input           string
		shouldErr       bool
		expectedRepo    string
		expectedUser    string
		expectedPass    string
		expectedSigned  bool
		expectedSeconds int
	}{
		// Full configuration
		{
			`oci registry.example.com/zones/example.org:v1.2.3 {
				username admin
				password secret123
				interval 1800
				require_signed
			}`,
			false,
			"registry.example.com/zones/example.org:v1.2.3",
			"admin",
			"secret123",
			true,
			1800,
		},
		// Partial configuration
		{
			`oci ghcr.io/user/zone:latest {
				interval 600
			}`,
			false,
			"ghcr.io/user/zone:latest",
			"",
			"",
			false,
			600,
		},
		// Missing password (should error)
		{
			`oci registry.example.com/zones/example.org:latest {
				username admin
			}`,
			true,
			"",
			"",
			"",
			false,
			0,
		},
		// Missing username (should error)
		{
			`oci registry.example.com/zones/example.org:latest {
				password secret
			}`,
			true,
			"",
			"",
			"",
			false,
			0,
		},
		// Interval too low (should be clamped to minimum)
		{
			`oci registry.example.com/zones/example.org:latest {
				interval 1
			}`,
			false,
			"registry.example.com/zones/example.org:latest",
			"",
			"",
			false,
			5, // MinimumInterval is 5 seconds
		},
		// Unknown property
		{
			`oci registry.example.com/zones/example.org:latest {
				unknown_property value
			}`,
			true,
			"",
			"",
			"",
			false,
			0,
		},
	}

	for i, tc := range tests {
		c := caddy.NewTestController("dns", tc.input)
		o, err := parse(c)

		if tc.shouldErr {
			if err == nil {
				t.Errorf("Test %d: expected error but got none", i)
			}
			continue
		}

		if err != nil {
			t.Errorf("Test %d: unexpected error: %v", i, err)
			continue
		}

		if o.repo != tc.expectedRepo {
			t.Errorf("Test %d: expected repo %s, got %s", i, tc.expectedRepo, o.repo)
		}

		if o.username != tc.expectedUser {
			t.Errorf("Test %d: expected username %s, got %s", i, tc.expectedUser, o.username)
		}

		if o.password != tc.expectedPass {
			t.Errorf("Test %d: expected password %s, got %s", i, tc.expectedPass, o.password)
		}

		if o.requireSigned != tc.expectedSigned {
			t.Errorf("Test %d: expected requireSigned %v, got %v", i, tc.expectedSigned, o.requireSigned)
		}

		expectedDuration := time.Duration(tc.expectedSeconds) * time.Second
		if o.interval != expectedDuration {
			t.Errorf("Test %d: expected interval %v, got %v", i, expectedDuration, o.interval)
		}
	}
}

func TestParseZoneFile(t *testing.T) {
	// Load test zone file
	data, err := os.ReadFile("testdata/zones/example.org.zone")
	if err != nil {
		t.Fatalf("Failed to read test zone file: %v", err)
	}

	zone, err := parseZoneFile(data, "example.org.", nil)
	if err != nil {
		t.Fatalf("Failed to parse zone file: %v", err)
	}

	// Verify SOA exists
	if zone.SOA == nil {
		t.Fatal("Zone missing SOA record")
	}

	// Verify serial
	if zone.Serial != 2025111501 {
		t.Errorf("Expected serial 2025111501, got %d", zone.Serial)
	}

	// Verify origin
	if zone.Origin != "example.org." {
		t.Errorf("Expected origin example.org., got %s", zone.Origin)
	}

	// Test A record lookup
	aRecords := zone.Lookup("www.example.org.", dns.TypeA)
	if len(aRecords) != 1 {
		t.Errorf("Expected 1 A record for www.example.org, got %d", len(aRecords))
	}

	// Test AAAA record lookup
	aaaaRecords := zone.Lookup("www.example.org.", dns.TypeAAAA)
	if len(aaaaRecords) != 1 {
		t.Errorf("Expected 1 AAAA record for www.example.org, got %d", len(aaaaRecords))
	}

	// Test CNAME lookup
	cnameRecords := zone.Lookup("ftp.example.org.", dns.TypeCNAME)
	if len(cnameRecords) != 1 {
		t.Errorf("Expected 1 CNAME record for ftp.example.org, got %d", len(cnameRecords))
	}

	// Test MX records
	mxRecords := zone.Lookup("example.org.", dns.TypeMX)
	if len(mxRecords) != 2 {
		t.Errorf("Expected 2 MX records for example.org, got %d", len(mxRecords))
	}

	// Test NS records
	nsRecords := zone.Lookup("example.org.", dns.TypeNS)
	if len(nsRecords) != 2 {
		t.Errorf("Expected 2 NS records for example.org, got %d", len(nsRecords))
	}

	// Test TXT records
	txtRecords := zone.Lookup("example.org.", dns.TypeTXT)
	if len(txtRecords) != 1 {
		t.Errorf("Expected 1 TXT record for example.org, got %d", len(txtRecords))
	}
}

func TestParseZoneFileNoSOA(t *testing.T) {
	data, err := os.ReadFile("testdata/zones/no-soa.zone")
	if err != nil {
		t.Fatalf("Failed to read test zone file: %v", err)
	}

	_, err = parseZoneFile(data, "nosoa.zone.", nil)
	if err == nil {
		t.Error("Expected error for zone without SOA, got none")
	}
}

func TestParseZoneMetadata(t *testing.T) {
	data, err := os.ReadFile("testdata/metadata/config.json")
	if err != nil {
		t.Fatalf("Failed to read metadata file: %v", err)
	}

	metadata, err := parseZoneMetadata(data)
	if err != nil {
		t.Fatalf("Failed to parse metadata: %v", err)
	}

	if metadata.SchemaVersion != "1.0.0" {
		t.Errorf("Expected schemaVersion 1.0.0, got %s", metadata.SchemaVersion)
	}

	if metadata.Kind != "DNSZoneFile" {
		t.Errorf("Expected kind DNSZoneFile, got %s", metadata.Kind)
	}

	if metadata.Metadata.Name != "example.org" {
		t.Errorf("Expected name example.org, got %s", metadata.Metadata.Name)
	}

	if metadata.Zone.Serial != 2025111501 {
		t.Errorf("Expected serial 2025111501, got %d", metadata.Zone.Serial)
	}

	if metadata.Zone.Origin != "example.org." {
		t.Errorf("Expected origin example.org., got %s", metadata.Zone.Origin)
	}

	if metadata.Validation.Validated != true {
		t.Error("Expected validated to be true")
	}
}

func TestZoneStore(t *testing.T) {
	store := NewZoneStore()

	// Test empty store
	if store.Count() != 0 {
		t.Errorf("Expected empty store, got count %d", store.Count())
	}

	// Create a test zone
	zone := &Zone{
		Origin:   "example.org.",
		Records:  make(map[string][]dns.RR),
		Serial:   2025111501,
		LoadedAt: time.Now(),
	}

	// Test update
	store.Update("example.org.", zone)
	if store.Count() != 1 {
		t.Errorf("Expected 1 zone, got %d", store.Count())
	}

	// Test get
	retrieved := store.Get("example.org.")
	if retrieved == nil {
		t.Error("Expected to retrieve zone, got nil")
	}
	if retrieved.Serial != zone.Serial {
		t.Errorf("Expected serial %d, got %d", zone.Serial, retrieved.Serial)
	}

	// Test get non-existent
	nonExistent := store.Get("nonexistent.org.")
	if nonExistent != nil {
		t.Error("Expected nil for non-existent zone")
	}

	// Test list
	origins := store.List()
	if len(origins) != 1 {
		t.Errorf("Expected 1 origin, got %d", len(origins))
	}

	// Test remove
	store.Remove("example.org.")
	if store.Count() != 0 {
		t.Errorf("Expected 0 zones after removal, got %d", store.Count())
	}
}

func TestZoneLookup(t *testing.T) {
	data, err := os.ReadFile("testdata/zones/example.org.zone")
	if err != nil {
		t.Fatalf("Failed to read test zone file: %v", err)
	}

	zone, err := parseZoneFile(data, "example.org.", nil)
	if err != nil {
		t.Fatalf("Failed to parse zone file: %v", err)
	}

	// Test HasName
	if !zone.HasName("www.example.org.") {
		t.Error("Expected to find www.example.org.")
	}

	if zone.HasName("nonexistent.example.org.") {
		t.Error("Did not expect to find nonexistent.example.org.")
	}

	// Test ANY query returns all types
	anyRecords := zone.Lookup("www.example.org.", dns.TypeANY)
	if len(anyRecords) < 2 { // Should have A and AAAA
		t.Errorf("Expected at least 2 records for ANY query, got %d", len(anyRecords))
	}
}

func TestServeDNS(t *testing.T) {
	// Load test zone
	data, err := os.ReadFile("testdata/zones/example.org.zone")
	if err != nil {
		t.Fatalf("Failed to read test zone file: %v", err)
	}

	zone, err := parseZoneFile(data, "example.org.", nil)
	if err != nil {
		t.Fatalf("Failed to parse zone file: %v", err)
	}

	// Create plugin instance
	o := &OCI{
		zones: []string{"example.org."},
		store: NewZoneStore(),
	}
	o.store.Update("example.org.", zone)

	tests := []struct {
		name        string
		qname       string
		qtype       uint16
		expectRcode int
		expectCount int
	}{
		{
			name:        "A record query",
			qname:       "www.example.org.",
			qtype:       dns.TypeA,
			expectRcode: dns.RcodeSuccess,
			expectCount: 1,
		},
		{
			name:        "AAAA record query",
			qname:       "www.example.org.",
			qtype:       dns.TypeAAAA,
			expectRcode: dns.RcodeSuccess,
			expectCount: 1,
		},
		{
			name:        "MX record query",
			qname:       "example.org.",
			qtype:       dns.TypeMX,
			expectRcode: dns.RcodeSuccess,
			expectCount: 3, // 2 MX records + 1 A glue record for mail.example.org
		},
		{
			name:        "NXDOMAIN",
			qname:       "nonexistent.example.org.",
			qtype:       dns.TypeA,
			expectRcode: dns.RcodeNameError,
			expectCount: 0,
		},
		{
			name:        "NODATA (name exists, no record type)",
			qname:       "www.example.org.",
			qtype:       dns.TypeMX,
			expectRcode: dns.RcodeSuccess,
			expectCount: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := new(dns.Msg)
			req.SetQuestion(tc.qname, tc.qtype)

			rec := dnstest.NewRecorder(&test.ResponseWriter{})
			rcode, err := o.ServeDNS(context.Background(), rec, req)

			if err != nil && tc.expectRcode != dns.RcodeServerFailure {
				t.Errorf("Unexpected error: %v", err)
			}

			if rcode != tc.expectRcode {
				t.Errorf("Expected rcode %d, got %d", tc.expectRcode, rcode)
			}

			if rec.Msg != nil && len(rec.Msg.Answer) != tc.expectCount {
				t.Errorf("Expected %d answers, got %d", tc.expectCount, len(rec.Msg.Answer))
			}
		})
	}
}

func TestServeDNSAuthoritativeFlag(t *testing.T) {
	data, err := os.ReadFile("testdata/zones/example.org.zone")
	if err != nil {
		t.Fatalf("Failed to read test zone file: %v", err)
	}

	zone, err := parseZoneFile(data, "example.org.", nil)
	if err != nil {
		t.Fatalf("Failed to parse zone file: %v", err)
	}

	o := &OCI{
		zones: []string{"example.org."},
		store: NewZoneStore(),
	}
	o.store.Update("example.org.", zone)

	req := new(dns.Msg)
	req.SetQuestion("www.example.org.", dns.TypeA)

	rec := dnstest.NewRecorder(&test.ResponseWriter{})
	_, err = o.ServeDNS(context.Background(), rec, req)
	if err != nil {
		t.Fatalf("ServeDNS error: %v", err)
	}

	if rec.Msg == nil {
		t.Fatal("Expected response message, got nil")
	}

	if !rec.Msg.Authoritative {
		t.Error("Expected Authoritative flag to be true")
	}
}

func TestServeDNSNXDOMAINWithSOA(t *testing.T) {
	data, err := os.ReadFile("testdata/zones/example.org.zone")
	if err != nil {
		t.Fatalf("Failed to read test zone file: %v", err)
	}

	zone, err := parseZoneFile(data, "example.org.", nil)
	if err != nil {
		t.Fatalf("Failed to parse zone file: %v", err)
	}

	o := &OCI{
		zones: []string{"example.org."},
		store: NewZoneStore(),
	}
	o.store.Update("example.org.", zone)

	req := new(dns.Msg)
	req.SetQuestion("nonexistent.example.org.", dns.TypeA)

	rec := dnstest.NewRecorder(&test.ResponseWriter{})
	_, err = o.ServeDNS(context.Background(), rec, req)
	if err != nil {
		t.Fatalf("ServeDNS error: %v", err)
	}

	if rec.Msg == nil {
		t.Fatal("Expected response message, got nil")
	}

	// Check that SOA is in authority section
	if len(rec.Msg.Ns) == 0 {
		t.Error("Expected SOA in authority section for NXDOMAIN")
	}

	foundSOA := false
	for _, rr := range rec.Msg.Ns {
		if _, ok := rr.(*dns.SOA); ok {
			foundSOA = true
			break
		}
	}

	if !foundSOA {
		t.Error("Expected SOA record in authority section")
	}
}

func TestValidateZone(t *testing.T) {
	tests := []struct {
		name      string
		zone      *Zone
		shouldErr bool
	}{
		{
			name:      "nil zone",
			zone:      nil,
			shouldErr: true,
		},
		{
			name: "empty origin",
			zone: &Zone{
				Origin:  "",
				Records: make(map[string][]dns.RR),
				SOA:     &dns.SOA{},
			},
			shouldErr: true,
		},
		{
			name: "origin not fully qualified",
			zone: &Zone{
				Origin:  "example.org",
				Records: make(map[string][]dns.RR),
				SOA:     &dns.SOA{},
			},
			shouldErr: true,
		},
		{
			name: "missing SOA",
			zone: &Zone{
				Origin:  "example.org.",
				Records: make(map[string][]dns.RR),
				SOA:     nil,
			},
			shouldErr: true,
		},
		{
			name: "no records",
			zone: &Zone{
				Origin:  "example.org.",
				Records: make(map[string][]dns.RR),
				SOA:     &dns.SOA{Hdr: dns.RR_Header{Name: "example.org."}},
			},
			shouldErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateZone(tc.zone)
			if tc.shouldErr && err == nil {
				t.Error("Expected error, got none")
			}
			if !tc.shouldErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestExtractOrigin(t *testing.T) {
	tests := []struct {
		name     string
		metadata *ZoneMetadata
		fallback string
		expected string
	}{
		{
			name:     "nil metadata uses fallback",
			metadata: nil,
			fallback: "example.org",
			expected: "example.org.",
		},
		{
			name: "metadata with origin",
			metadata: &ZoneMetadata{
				Zone: struct {
					Origin       string `json:"origin"`
					TTL          uint32 `json:"ttl,omitempty"`
					Serial       uint32 `json:"serial"`
					Refresh      uint32 `json:"refresh,omitempty"`
					Retry        uint32 `json:"retry,omitempty"`
					Expire       uint32 `json:"expire,omitempty"`
					Minimum      uint32 `json:"minimum,omitempty"`
					RecordCount  int    `json:"recordCount,omitempty"`
					DNSSECSigned bool   `json:"dnssecSigned"`
				}{
					Origin: "test.org",
				},
			},
			fallback: "example.org",
			expected: "test.org.",
		},
		{
			name: "metadata origin already qualified",
			metadata: &ZoneMetadata{
				Zone: struct {
					Origin       string `json:"origin"`
					TTL          uint32 `json:"ttl,omitempty"`
					Serial       uint32 `json:"serial"`
					Refresh      uint32 `json:"refresh,omitempty"`
					Retry        uint32 `json:"retry,omitempty"`
					Expire       uint32 `json:"expire,omitempty"`
					Minimum      uint32 `json:"minimum,omitempty"`
					RecordCount  int    `json:"recordCount,omitempty"`
					DNSSECSigned bool   `json:"dnssecSigned"`
				}{
					Origin: "test.org.",
				},
			},
			fallback: "example.org",
			expected: "test.org.",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := extractOrigin(tc.metadata, tc.fallback)
			if result != tc.expected {
				t.Errorf("Expected %s, got %s", tc.expected, result)
			}
		})
	}
}

func TestOCIReady(t *testing.T) {
	o := &OCI{}

	// Not ready when store is nil
	if o.Ready() {
		t.Error("Expected not ready when store is nil")
	}

	// Not ready when store is empty
	o.store = NewZoneStore()
	if o.Ready() {
		t.Error("Expected not ready when store is empty")
	}

	// Ready when store has zones
	zone := &Zone{Origin: "example.org."}
	o.store.Update("example.org.", zone)
	if !o.Ready() {
		t.Error("Expected ready when store has zones")
	}
}

func TestOCIName(t *testing.T) {
	o := &OCI{}
	if o.Name() != "oci" {
		t.Errorf("Expected name 'oci', got '%s'", o.Name())
	}
}

// ResponseWriter that captures the response
type testResponseWriter struct {
	msg *dns.Msg
}

func (w *testResponseWriter) LocalAddr() net.Addr {
	return &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 53}
}

func (w *testResponseWriter) RemoteAddr() net.Addr {
	return &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345}
}

func (w *testResponseWriter) WriteMsg(m *dns.Msg) error {
	w.msg = m
	return nil
}

func (w *testResponseWriter) Write([]byte) (int, error) {
	return 0, nil
}

func (w *testResponseWriter) Close() error {
	return nil
}

func (w *testResponseWriter) TsigStatus() error {
	return nil
}

func (w *testResponseWriter) TsigTimersOnly(bool) {}

func (w *testResponseWriter) Hijack() {}
