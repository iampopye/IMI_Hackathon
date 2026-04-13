package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// ── HTTP layer ────────────────────────────────────────────────────────────

	// HTTPRequestsTotal counts every HTTP request by method, route pattern, and status code.
	HTTPRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total HTTP requests by method, path, and status code.",
	}, []string{"method", "path", "status"})

	// HTTPRequestDuration tracks HTTP handler latency per route.
	HTTPRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request duration in seconds by method and path.",
		Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
	}, []string{"method", "path"})

	// ── Search ────────────────────────────────────────────────────────────────

	// SearchRequestsTotal counts search requests by tier, cache layer hit, and status.
	SearchRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "search_requests_total",
		Help: "Total search requests by tier, cache_layer (none/L1/L2), and status (ok/error).",
	}, []string{"tier", "cache_layer", "status"})

	// SearchDuration tracks search engine latency broken out by tier.
	SearchDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "search_duration_seconds",
		Help:    "Search duration in seconds by tier.",
		Buckets: []float64{.001, .002, .005, .01, .015, .025, .05, .1, .25, .5, 1},
	}, []string{"tier"})

	// ── Cache ─────────────────────────────────────────────────────────────────

	// CacheHitsTotal counts cache hits by layer (L1 = in-process, L2 = redis).
	CacheHitsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cache_hits_total",
		Help: "Total cache hits by layer (L1 or L2).",
	}, []string{"layer"})

	// CacheMissesTotal counts cache misses by layer.
	CacheMissesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cache_misses_total",
		Help: "Total cache misses by layer (L1 or L2).",
	}, []string{"layer"})

	// ── Upsert ────────────────────────────────────────────────────────────────

	// UpsertRecordsTotal counts individual records processed by status.
	UpsertRecordsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "upsert_records_total",
		Help: "Total records processed by upsert engine by status (inserted/updated/skipped/failed).",
	}, []string{"status"})

	// UpsertDuration tracks the duration of bulk upsert operations.
	UpsertDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "upsert_duration_seconds",
		Help:    "Bulk upsert operation duration in seconds.",
		Buckets: []float64{.1, .5, 1, 2.5, 5, 10, 30, 60},
	})

	// ── Outbox ────────────────────────────────────────────────────────────────

	// OutboxEventsTotal counts outbox events by final status.
	OutboxEventsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "outbox_events_total",
		Help: "Total outbox events processed by status (published/dead/retried).",
	}, []string{"status"})

	// OutboxPendingGauge tracks the current number of pending outbox events.
	OutboxPendingGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "outbox_pending_total",
		Help: "Current number of PENDING outbox events awaiting Kafka publish.",
	})

	// ── Elasticsearch pipeline ────────────────────────────────────────────────

	// ESIndexOperationsTotal counts ES index/delete operations by outcome.
	ESIndexOperationsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "es_index_operations_total",
		Help: "Total Elasticsearch index operations by operation (index/delete) and status (success/error).",
	}, []string{"operation", "status"})

	// ── Reconciler ────────────────────────────────────────────────────────────

	// ReconcilerDriftTotal counts times the reconciler detected PG ↔ ES drift.
	ReconcilerDriftTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "reconciler_drift_detected_total",
		Help: "Total times the reconciler detected drift between PostgreSQL and Elasticsearch.",
	})

	// ReconcilerReindexTotal counts zero-downtime reindex operations triggered.
	ReconcilerReindexTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "reconciler_reindex_total",
		Help: "Total zero-downtime reindex operations triggered by the reconciler.",
	})

	// ── Dataset ───────────────────────────────────────────────────────────────

	// DatasetTierTransitionsTotal counts tier upgrades/downgrades.
	DatasetTierTransitionsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "dataset_tier_transitions_total",
		Help: "Total dataset tier transitions by from_tier and to_tier.",
	}, []string{"from_tier", "to_tier"})
)
