package oci

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/retry"
)

// Layers holds the parsed layers from an OCI artifact.
type Layers struct {
	Config   []byte
	ZoneFile []byte
}

// pullAndParse pulls an OCI artifact and parses it into a Zone.
func (o *OCI) pullAndParse(ctx context.Context) error {
	start := time.Now()
	status := "success"

	defer func() {
		duration := time.Since(start).Seconds()
		recordPullDuration(duration, status)
		log.Debugf("OCI pull completed in %.2fs with status %s for %s", duration, status, o.repo)
	}()

	// Create remote repository
	repo, err := remote.NewRepository(o.repo)
	if err != nil {
		status = "error"
		return fmt.Errorf("failed to create repository: %w", err)
	}

	// Configure authentication if provided
	if o.username != "" && o.password != "" {
		repo.Client = &auth.Client{
			Client: retry.DefaultClient,
			Cache:  auth.NewCache(),
			Credential: auth.StaticCredential(repo.Reference.Registry, auth.Credential{
				Username: o.username,
				Password: o.password,
			}),
		}
	} else {
		repo.Client = retry.DefaultClient
	}

	// Resolve the artifact descriptor
	desc, err := repo.Resolve(ctx, repo.Reference.Reference)
	if err != nil {
		status = "error"
		return fmt.Errorf("failed to resolve artifact: %w", err)
	}

	// Check if digest has changed
	if o.currentDigest != "" && o.currentDigest == desc.Digest.String() {
		log.Debugf("Artifact digest unchanged for %s, skipping pull", o.repo)
		return nil
	}

	log.Infof("Pulling new artifact version for %s (digest: %s)", o.repo, desc.Digest.String())

	// Pull the manifest
	manifest, err := o.pullManifest(ctx, repo, desc)
	if err != nil {
		status = "error"
		return fmt.Errorf("failed to pull manifest: %w", err)
	}

	// Pull layers
	layers, err := o.pullLayers(ctx, repo, manifest)
	if err != nil {
		status = "error"
		return fmt.Errorf("failed to pull layers: %w", err)
	}

	// Parse metadata from config
	var metadata *ZoneMetadata
	if len(layers.Config) > 0 {
		metadata, err = parseZoneMetadata(layers.Config)
		if err != nil {
			log.Warningf("Failed to parse zone metadata: %v, continuing without metadata", err)
		}
	}

	// Determine origin
	origin := extractOrigin(metadata, o.zones[0])

	// Parse zone file
	zone, err := parseZoneFile(layers.ZoneFile, origin, metadata)
	if err != nil {
		status = "error"
		return fmt.Errorf("failed to parse zone file: %w", err)
	}

	// Validate zone
	if err := validateZone(zone); err != nil {
		status = "error"
		return fmt.Errorf("zone validation failed: %w", err)
	}

	// Update digest tracking
	zone.Digest = desc.Digest.String()

	// Atomic update of zone store
	o.store.Update(origin, zone)
	o.currentDigest = desc.Digest.String()

	// Update metrics
	updateZoneMetrics(zone)

	log.Infof("Successfully loaded zone %s with serial %d (%d records)", origin, zone.Serial, len(zone.Records))

	return nil
}

// pullManifest pulls and decodes the OCI manifest.
func (o *OCI) pullManifest(ctx context.Context, repo *remote.Repository, desc ocispec.Descriptor) (*ocispec.Manifest, error) {
	rc, err := repo.Fetch(ctx, desc)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch manifest: %w", err)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	var manifest ocispec.Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to decode manifest: %w", err)
	}

	return &manifest, nil
}

// pullLayers pulls the config and zone file layers from the artifact.
func (o *OCI) pullLayers(ctx context.Context, repo *remote.Repository, manifest *ocispec.Manifest) (*Layers, error) {
	layers := &Layers{}

	// Pull config layer
	if manifest.Config.Size > 0 {
		configData, err := o.fetchBlob(ctx, repo, manifest.Config)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch config: %w", err)
		}
		layers.Config = configData
		log.Debugf("Fetched config layer: %s (%d bytes)", manifest.Config.MediaType, len(configData))
	}

	// Find and pull zone file layer
	var zoneLayerFound bool
	for _, layerDesc := range manifest.Layers {
		switch layerDesc.MediaType {
		case MediaTypeZoneRFC1035:
			data, err := o.fetchBlob(ctx, repo, layerDesc)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch zone file: %w", err)
			}
			layers.ZoneFile = data
			zoneLayerFound = true
			log.Debugf("Fetched zone file layer: %s (%d bytes)", layerDesc.MediaType, len(data))

		case MediaTypeZoneRFC1035Gzip:
			data, err := o.fetchBlob(ctx, repo, layerDesc)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch compressed zone file: %w", err)
			}
			// Decompress
			decompressed, err := decompressGzip(data)
			if err != nil {
				return nil, fmt.Errorf("failed to decompress zone file: %w", err)
			}
			layers.ZoneFile = decompressed
			zoneLayerFound = true
			log.Debugf("Fetched and decompressed zone file layer: %s (%d -> %d bytes)", layerDesc.MediaType, len(data), len(decompressed))

		default:
			// Check if it's a generic layer that might be a zone file
			if !zoneLayerFound {
				data, err := o.fetchBlob(ctx, repo, layerDesc)
				if err != nil {
					log.Warningf("Failed to fetch layer %s: %v", layerDesc.MediaType, err)
					continue
				}
				// Assume it's a zone file if no specific media type found
				layers.ZoneFile = data
				zoneLayerFound = true
				log.Debugf("Fetched generic layer as zone file: %s (%d bytes)", layerDesc.MediaType, len(data))
			}
		}

		if zoneLayerFound {
			break
		}
	}

	if !zoneLayerFound || len(layers.ZoneFile) == 0 {
		return nil, fmt.Errorf("no zone file layer found in artifact")
	}

	return layers, nil
}

// fetchBlob fetches a blob from the repository.
func (o *OCI) fetchBlob(ctx context.Context, repo *remote.Repository, desc ocispec.Descriptor) ([]byte, error) {
	rc, err := repo.Fetch(ctx, desc)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// decompressGzip decompresses gzip-compressed data.
func decompressGzip(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	return io.ReadAll(reader)
}

// verifySignature verifies the cosign signature of an artifact.
// This is a placeholder for future implementation.
func (o *OCI) verifySignature(ctx context.Context, desc ocispec.Descriptor) error {
	if !o.requireSigned {
		return nil
	}

	// TODO: Implement cosign signature verification
	// For now, return an error if signature verification is required but not implemented
	return fmt.Errorf("signature verification not yet implemented")
}
