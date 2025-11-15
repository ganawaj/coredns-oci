# oci

## Name

*oci* - serve DNS zones directly from OCI artifacts

## Description

*oci* is a CoreDNS plugin that serves DNS records directly from OCI (Open Container Initiative) artifacts. Unlike traditional zone file management, this plugin pulls zone files packaged as OCI artifacts from container registries and serves them in-memory, eliminating the need for disk storage.

**Key Features:**
- **Direct serving** - Serves DNS queries directly from memory (no delegation to file/auto plugins)
- **Cloud-native** - Zone files packaged as OCI artifacts for GitOps workflows
- **Atomic updates** - Zero-downtime zone reloads when new versions are pulled
- **Automatic reloading** - Periodically checks for updated artifacts
- **Prometheus metrics** - Built-in observability for monitoring

## Syntax

```txt
zone {
    oci REPO {
        username USERNAME
        password PASSWORD
        interval SECONDS
        require_signed
        fallthrough [ZONES...]
    }
}
```

- **REPO** (required) is the OCI artifact reference (registry/repository:tag)
- **USERNAME** (optional) username for private registry authentication
- **PASSWORD** (optional) password for private registry authentication
- **INTERVAL** (optional) seconds between update checks; default is 3600 (1 hour), minimum 5
- **require_signed** (optional) flag to require cosign signature verification
- **fallthrough** (optional) pass to next plugin if no record found

## OCI Artifact Structure

Zone files must be packaged as OCI artifacts with the following structure:

### Media Types

```
# Config descriptor (optional)
application/vnd.dns.zonefile.config.v1+json

# Zone file layer
application/vnd.dns.zonefile.rfc1035.v1
application/vnd.dns.zonefile.rfc1035.v1+gzip  # Compressed variant
```

### Config Schema (Optional)

The config descriptor contains zone metadata:

```json
{
  "schemaVersion": "1.0.0",
  "kind": "DNSZoneFile",
  "metadata": {
    "name": "example.org",
    "version": "1.2.3",
    "created": "2025-11-15T12:00:00Z"
  },
  "zone": {
    "origin": "example.org.",
    "serial": 2025111501,
    "dnssecSigned": false
  },
  "validation": {
    "syntax": "RFC1035",
    "validated": true,
    "validator": "named-checkzone"
  }
}
```

## Examples

### Public Registry

Simple configuration for a public registry:

```corefile
example.org {
    oci registry-1.docker.io/ganawaj/zones/example.org:latest
    cache 30
    log
}
```

### Private Registry with Authentication

```corefile
example.org {
    oci ghcr.io/myorg/zones/example.org:v1.2.3 {
        username myuser
        password ghp_xxxxxxxxxxxxxxxxxxxx
        interval 600
    }
    cache 300
    prometheus
    errors
    log
}
```

### With Signature Verification

```corefile
example.org {
    oci registry.example.com/zones/example.org:production {
        username admin
        password secrettoken
        interval 1800
        require_signed
    }
    cache 300
}
```

### With Fallthrough

If a record is not found in the zone, pass to the next plugin:

```corefile
example.org {
    oci ghcr.io/myorg/zones/example.org:latest {
        fallthrough
    }
    forward . 8.8.8.8
}
```

### Multiple Zones

```corefile
.:53 {
    example.org {
        oci registry.example.com/zones/example.org:latest {
            interval 600
        }
    }

    example.net {
        oci registry.example.com/zones/example.net:latest {
            interval 600
        }
    }

    forward . 8.8.8.8
    cache 30
}
```

## Building a Zone Artifact

Example script to package and push a zone file:

```bash
#!/bin/bash

ZONE_FILE="db.example.org"
ZONE_NAME="example.org"
REGISTRY="registry.example.com"
VERSION="1.2.3"

# Validate zone
named-checkzone "$ZONE_NAME" "$ZONE_FILE" || exit 1

# Extract serial from zone file
SERIAL=$(grep -oP '(?<=\s)\d{10}(?=\s+;\s*serial)' "$ZONE_FILE" | head -1)

# Create metadata config
cat > config.json <<EOF
{
  "schemaVersion": "1.0.0",
  "kind": "DNSZoneFile",
  "metadata": {
    "name": "$ZONE_NAME",
    "version": "$VERSION",
    "created": "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  },
  "zone": {
    "origin": "$ZONE_NAME.",
    "serial": $SERIAL,
    "dnssecSigned": false
  },
  "validation": {
    "syntax": "RFC1035",
    "validated": true,
    "validatedAt": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
    "validator": "named-checkzone"
  }
}
EOF

# Push to registry using ORAS
oras push "$REGISTRY/zones/$ZONE_NAME:$VERSION" \
  --config config.json:application/vnd.dns.zonefile.config.v1+json \
  "$ZONE_FILE:application/vnd.dns.zonefile.rfc1035.v1"

# Tag as latest
oras tag "$REGISTRY/zones/$ZONE_NAME:$VERSION" latest

echo "Published zone $ZONE_NAME:$VERSION (serial: $SERIAL)"
```

## Metrics

The plugin exposes the following Prometheus metrics:

| Metric | Type | Description |
|--------|------|-------------|
| `coredns_oci_request_count_total` | Counter | Total DNS requests by server, zone, type, rcode |
| `coredns_oci_zone_serial` | Gauge | Current serial number of each zone |
| `coredns_oci_zone_loaded_timestamp_seconds` | Gauge | Unix timestamp when zone was last loaded |
| `coredns_oci_zone_record_count` | Gauge | Number of DNS records in each zone |
| `coredns_oci_pull_duration_seconds` | Histogram | Duration of OCI artifact pull operations |
| `coredns_oci_pull_total` | Counter | Total number of pull operations by status |

## How It Works

1. **Startup**: Plugin pulls OCI artifact from registry
2. **Parsing**: Zone file is parsed into in-memory data structures using miekg/dns
3. **Serving**: DNS queries are answered directly from memory (O(1) lookups)
4. **Reloading**: Background goroutine periodically checks for new artifact versions
5. **Atomic Updates**: Zone data is swapped atomically with no query interruption

## DNS Response Behavior

- Sets `Authoritative` flag to `true` for all responses
- Returns `NXDOMAIN` with SOA in authority section for non-existent names
- Returns `NODATA` (empty answer) with SOA for existing names without requested type
- Adds glue records (A/AAAA) for NS and MX queries when targets are in-zone
- Supports all standard DNS record types (A, AAAA, CNAME, MX, NS, SOA, TXT, SRV, CAA, etc.)

## Requirements

- CoreDNS 1.10.0+
- Go 1.21+ (for building)
- ORAS CLI (for pushing artifacts)
- Valid RFC 1035 zone files with SOA records

## Limitations

- Zone files must include a valid SOA record
- No disk persistence (in-memory only)
- Signature verification requires cosign (not yet implemented)
- DNSSEC signing must be done before packaging (pre-signed zones)

## Also See

- [ORAS (OCI Registry As Storage)](https://oras.land/)
- [CoreDNS Plugin Development](https://coredns.io/manual/plugins/)
- [RFC 1035 - DNS Zone Files](https://datatracker.ietf.org/doc/html/rfc1035)
- [OCI Image Specification](https://github.com/opencontainers/image-spec)

## License

MIT License
