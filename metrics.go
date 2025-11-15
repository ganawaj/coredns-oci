package oci

import (
	"sync"

	"github.com/coredns/coredns/plugin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Request metrics
	requestCount *prometheus.CounterVec

	// Zone health metrics
	zoneSerial            *prometheus.GaugeVec
	zoneLoadedTimestamp   *prometheus.GaugeVec
	zoneRecordCount       *prometheus.GaugeVec

	// Pull operation metrics
	pullDuration *prometheus.HistogramVec
	pullTotal    *prometheus.CounterVec

	once sync.Once
)

// initMetrics initializes all Prometheus metrics for the OCI plugin.
// This is called once per process to avoid duplicate registration.
func initMetrics() {
	once.Do(func() {
		requestCount = promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: plugin.Namespace,
			Subsystem: "oci",
			Name:      "request_count_total",
			Help:      "Total number of DNS requests handled by the OCI plugin.",
		}, []string{"server", "zone", "type", "rcode"})

		zoneSerial = promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: plugin.Namespace,
			Subsystem: "oci",
			Name:      "zone_serial",
			Help:      "Current serial number of the zone.",
		}, []string{"zone"})

		zoneLoadedTimestamp = promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: plugin.Namespace,
			Subsystem: "oci",
			Name:      "zone_loaded_timestamp_seconds",
			Help:      "Unix timestamp when the zone was last loaded.",
		}, []string{"zone"})

		zoneRecordCount = promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: plugin.Namespace,
			Subsystem: "oci",
			Name:      "zone_record_count",
			Help:      "Number of DNS records in the zone.",
		}, []string{"zone"})

		pullDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: plugin.Namespace,
			Subsystem: "oci",
			Name:      "pull_duration_seconds",
			Help:      "Duration of OCI artifact pull operations.",
			Buckets:   []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60},
		}, []string{"status"})

		pullTotal = promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: plugin.Namespace,
			Subsystem: "oci",
			Name:      "pull_total",
			Help:      "Total number of OCI artifact pull operations.",
		}, []string{"status"})
	})
}

// updateZoneMetrics updates the metrics for a loaded zone.
func updateZoneMetrics(zone *Zone) {
	if zone == nil {
		return
	}

	if zoneSerial != nil {
		zoneSerial.WithLabelValues(zone.Origin).Set(float64(zone.Serial))
	}
	if zoneLoadedTimestamp != nil {
		zoneLoadedTimestamp.WithLabelValues(zone.Origin).Set(float64(zone.LoadedAt.Unix()))
	}

	// Count total records
	var count int
	for _, records := range zone.Records {
		count += len(records)
	}
	if zoneRecordCount != nil {
		zoneRecordCount.WithLabelValues(zone.Origin).Set(float64(count))
	}
}

// recordPullDuration records the duration of a pull operation.
func recordPullDuration(duration float64, status string) {
	if pullDuration != nil {
		pullDuration.WithLabelValues(status).Observe(duration)
	}
	if pullTotal != nil {
		pullTotal.WithLabelValues(status).Inc()
	}
}

// incrementRequestCount increments the request counter for the given parameters.
func incrementRequestCount(server, zone, qtype, rcode string) {
	if requestCount != nil {
		requestCount.WithLabelValues(server, zone, qtype, rcode).Inc()
	}
}
