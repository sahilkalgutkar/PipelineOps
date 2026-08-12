// Package metrics exposes Prometheus counters/histograms for the ingestion
// hot path, scraped via GET /metrics.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HeartbeatsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "pipelineops_heartbeats_total",
		Help: "Total number of heartbeats ingested, by outcome status.",
	}, []string{"status"})

	HeartbeatsRejected = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "pipelineops_heartbeats_rejected_total",
		Help: "Total number of heartbeat requests rejected, by reason.",
	}, []string{"reason"})

	IngestDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "pipelineops_heartbeat_ingest_duration_seconds",
		Help:    "Time spent handling a single heartbeat ingest request.",
		Buckets: prometheus.DefBuckets,
	})
)
