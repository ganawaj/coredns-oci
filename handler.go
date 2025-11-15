package oci

import (
	"context"
	"strings"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"
)

// ServeDNS implements the plugin.Handler interface.
func (o *OCI) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}
	qname := state.Name()
	qtype := state.QType()

	// Find the zone this query belongs to
	zone := plugin.Zones(o.zones).Matches(qname)
	if zone == "" {
		// Not authoritative for this zone, pass to next plugin
		return plugin.NextOrFailure(o.Name(), o.Next, ctx, w, r)
	}

	// Get the zone data from store
	z := o.store.Get(zone)
	if z == nil {
		// Zone not loaded, server failure
		log.Errorf("Zone %s not loaded in store", zone)
		return o.errorResponse(state, zone, dns.RcodeServerFailure, nil)
	}

	// Perform the lookup
	answers, authority, err := o.lookup(state, z, qname, qtype)
	if err != nil {
		log.Errorf("Lookup error for %s %s: %v", qname, dns.TypeToString[qtype], err)
		return o.errorResponse(state, zone, dns.RcodeServerFailure, err)
	}

	// Handle NXDOMAIN
	if len(answers) == 0 && !z.HasName(qname) {
		// Name doesn't exist - check if we should fallthrough
		if o.Fall.Through(qname) {
			return plugin.NextOrFailure(o.Name(), o.Next, ctx, w, r)
		}
		return o.nxdomainResponse(state, z)
	}

	// Handle NODATA (name exists but no records of the requested type)
	if len(answers) == 0 && z.HasName(qname) {
		// Name exists but no records of this type
		return o.nodataResponse(state, z)
	}

	// Build successful response
	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative = true
	m.Answer = answers

	// Add authority section if we have any
	if len(authority) > 0 {
		m.Ns = authority
	}

	// Record metrics
	incrementRequestCount(state.LocalIP(), zone, dns.TypeToString[qtype], dns.RcodeToString[dns.RcodeSuccess])

	// Write response
	if err := w.WriteMsg(m); err != nil {
		log.Errorf("Failed to write DNS response: %v", err)
		return dns.RcodeServerFailure, err
	}

	return dns.RcodeSuccess, nil
}

// lookup finds records matching the query in the zone.
func (o *OCI) lookup(state request.Request, z *Zone, qname string, qtype uint16) ([]dns.RR, []dns.RR, error) {
	var answers []dns.RR
	var authority []dns.RR

	// Ensure qname is fully qualified
	if !strings.HasSuffix(qname, ".") {
		qname = qname + "."
	}

	// Check for CNAME first (unless querying for CNAME specifically)
	if qtype != dns.TypeCNAME {
		cnames := z.Lookup(qname, dns.TypeCNAME)
		if len(cnames) > 0 {
			// Return the CNAME and optionally follow it
			answers = append(answers, cnames...)
			// TODO: Follow CNAME chain if target is within zone
			return answers, authority, nil
		}
	}

	// Look up the requested record type
	records := z.Lookup(qname, qtype)
	answers = append(answers, records...)

	// Add glue records for NS and MX queries if applicable
	if qtype == dns.TypeNS || qtype == dns.TypeMX {
		for _, rr := range answers {
			var target string
			switch rec := rr.(type) {
			case *dns.NS:
				target = rec.Ns
			case *dns.MX:
				target = rec.Mx
			}

			if target != "" && strings.HasSuffix(target, z.Origin) {
				// Add A/AAAA glue records
				aRecords := z.Lookup(target, dns.TypeA)
				answers = append(answers, aRecords...)

				aaaaRecords := z.Lookup(target, dns.TypeAAAA)
				answers = append(answers, aaaaRecords...)
			}
		}
	}

	return answers, authority, nil
}

// errorResponse returns a server failure response.
func (o *OCI) errorResponse(state request.Request, zone string, rcode int, err error) (int, error) {
	m := new(dns.Msg)
	m.SetRcode(state.Req, rcode)
	m.Authoritative = true

	// Record metrics
	incrementRequestCount(state.LocalIP(), zone, dns.TypeToString[state.QType()], dns.RcodeToString[rcode])

	if writeErr := state.W.WriteMsg(m); writeErr != nil {
		log.Errorf("Failed to write error response: %v", writeErr)
		return dns.RcodeServerFailure, writeErr
	}

	return rcode, err
}

// nxdomainResponse returns an NXDOMAIN response with SOA in authority section.
func (o *OCI) nxdomainResponse(state request.Request, z *Zone) (int, error) {
	m := new(dns.Msg)
	m.SetRcode(state.Req, dns.RcodeNameError)
	m.Authoritative = true

	// Add SOA to authority section
	if z.SOA != nil {
		m.Ns = append(m.Ns, z.SOA)
	}

	// Record metrics
	incrementRequestCount(state.LocalIP(), z.Origin, dns.TypeToString[state.QType()], "NXDOMAIN")

	if err := state.W.WriteMsg(m); err != nil {
		log.Errorf("Failed to write NXDOMAIN response: %v", err)
		return dns.RcodeServerFailure, err
	}

	return dns.RcodeNameError, nil
}

// nodataResponse returns a NODATA response (name exists but no records of this type).
func (o *OCI) nodataResponse(state request.Request, z *Zone) (int, error) {
	m := new(dns.Msg)
	m.SetReply(state.Req)
	m.Authoritative = true

	// Add SOA to authority section for NODATA
	if z.SOA != nil {
		m.Ns = append(m.Ns, z.SOA)
	}

	// Record metrics
	incrementRequestCount(state.LocalIP(), z.Origin, dns.TypeToString[state.QType()], "NODATA")

	if err := state.W.WriteMsg(m); err != nil {
		log.Errorf("Failed to write NODATA response: %v", err)
		return dns.RcodeServerFailure, err
	}

	return dns.RcodeSuccess, nil
}
