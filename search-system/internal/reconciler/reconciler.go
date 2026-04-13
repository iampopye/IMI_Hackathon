package reconciler

import (
	"context"
	"database/sql"
	"time"

	"github.com/rs/zerolog"
	"github.com/yourusername/search-system/internal/metrics"
)

const (
	defaultInterval    = 5 * time.Minute
	accessLogRetention = "30 days"
)

// ESEngine is the interface Reconciler uses for ES operations.
// search.ESEngine satisfies this interface.
type ESEngine interface {
	DocCount(ctx context.Context, datasetID string) (int64, error)
	ReindexZeroDowntime(ctx context.Context, datasetID string) error
}

// Reconciler detects and repairs drift between PostgreSQL (source of truth),
// Elasticsearch (large-tier search index), and housekeeping tables.
//
// Responsibilities:
//  1. Detect record count drift between PostgreSQL and Elasticsearch per dataset.
//  2. Trigger zero-downtime reindex when drift is detected.
//  3. Purge dataset_access_log entries older than 30 days.
//  4. Warn on DEAD outbox events and reset them to PENDING for retry.
type Reconciler struct {
	db       *sql.DB
	es       ESEngine // nil when Elasticsearch is not configured
	interval time.Duration
	log      zerolog.Logger
}

// Config holds tunable Reconciler parameters.
type Config struct {
	Interval time.Duration // default: 5 minutes
}

// New creates a Reconciler. es may be nil when Elasticsearch is not configured.
func New(db *sql.DB, es ESEngine, cfg Config, log zerolog.Logger) *Reconciler {
	interval := cfg.Interval
	if interval == 0 {
		interval = defaultInterval
	}
	return &Reconciler{
		db:       db,
		es:       es,
		interval: interval,
		log:      log,
	}
}

// Start runs the reconciliation loop until ctx is cancelled. Call in a goroutine.
func (r *Reconciler) Start(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	r.log.Info().Dur("interval", r.interval).Msg("reconciler: started")

	// Run one cycle immediately on startup so drift is caught without waiting.
	r.tick(ctx)

	for {
		select {
		case <-ctx.Done():
			r.log.Info().Msg("reconciler: stopped")
			return
		case <-ticker.C:
			r.tick(ctx)
		}
	}
}

// tick runs one full reconciliation cycle.
func (r *Reconciler) tick(ctx context.Context) {
	r.log.Debug().Msg("reconciler: cycle start")

	// 1. Drift detection + corrective reindex (large-tier datasets only).
	if r.es != nil {
		r.reconcileAllDatasets(ctx)
	}

	// 2. Purge stale access log entries (> 30 days).
	r.purgeAccessLog(ctx)

	// 3. Detect and requeue DEAD outbox events.
	r.checkDeadOutbox(ctx)

	r.log.Debug().Msg("reconciler: cycle complete")
}

// ─── Drift detection ──────────────────────────────────────────────────────────

type datasetRow struct {
	id      string
	pgCount int64
}

// reconcileAllDatasets checks count drift for every dataset in the large tier.
func (r *Reconciler) reconcileAllDatasets(ctx context.Context) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT ds.dataset_id::text, COALESCE(dc.record_count, 0)
		FROM   dataset_states  ds
		LEFT  JOIN dataset_counts dc ON dc.dataset_id = ds.dataset_id
		WHERE  ds.current_tier = 'large'`)
	if err != nil {
		r.log.Error().Err(err).Msg("reconciler: fetch large-tier datasets")
		return
	}
	defer rows.Close()

	var datasets []datasetRow
	for rows.Next() {
		var d datasetRow
		if err := rows.Scan(&d.id, &d.pgCount); err != nil {
			r.log.Error().Err(err).Msg("reconciler: scan dataset row")
			continue
		}
		datasets = append(datasets, d)
	}
	if err := rows.Err(); err != nil {
		r.log.Error().Err(err).Msg("reconciler: iterate dataset rows")
		return
	}

	for _, d := range datasets {
		r.reconcileDataset(ctx, d)
	}
}

// reconcileDataset compares PG and ES counts for one dataset.
// Logs a warning when drift is detected and triggers a zero-downtime reindex.
func (r *Reconciler) reconcileDataset(ctx context.Context, d datasetRow) {
	esCount, err := r.es.DocCount(ctx, d.id)
	if err != nil {
		r.log.Error().Err(err).Str("dataset", d.id).Msg("reconciler: ES doc count failed")
		return
	}

	drift := d.pgCount - esCount
	if drift < 0 {
		drift = -drift
	}

	if drift == 0 {
		r.log.Debug().
			Str("dataset", d.id).
			Int64("count", d.pgCount).
			Msg("reconciler: no drift")
		return
	}

	var driftPct float64
	if d.pgCount > 0 {
		driftPct = float64(drift) / float64(d.pgCount) * 100
	}

	r.log.Warn().
		Str("dataset", d.id).
		Int64("pg_count", d.pgCount).
		Int64("es_count", esCount).
		Int64("drift", drift).
		Float64("drift_pct", driftPct).
		Msg("reconciler: drift detected — triggering zero-downtime reindex")

	metrics.ReconcilerDriftTotal.Inc()

	if err := r.es.ReindexZeroDowntime(ctx, d.id); err != nil {
		r.log.Error().Err(err).Str("dataset", d.id).Msg("reconciler: reindex failed")
		return
	}

	metrics.ReconcilerReindexTotal.Inc()

	r.log.Info().
		Str("dataset", d.id).
		Int64("drift_corrected", drift).
		Msg("reconciler: reindex complete, drift corrected")
}

// ─── Access log cleanup ───────────────────────────────────────────────────────

// purgeAccessLog deletes dataset_access_log entries older than 30 days.
// Migration 004 documents this as the reconciler's responsibility.
func (r *Reconciler) purgeAccessLog(ctx context.Context) {
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM dataset_access_log WHERE accessed_at < NOW() - INTERVAL '`+accessLogRetention+`'`)
	if err != nil {
		r.log.Error().Err(err).Msg("reconciler: purge access log failed")
		return
	}
	n, _ := res.RowsAffected()
	if n > 0 {
		r.log.Info().Int64("rows_purged", n).Msg("reconciler: access log entries purged")
	}
}

// ─── Dead outbox recovery ─────────────────────────────────────────────────────

// checkDeadOutbox warns when outbox events hit max retry attempts and could
// not be delivered to Kafka. Automatically requeues them to PENDING so the
// poller retries — this recovers from transient Kafka unavailability.
func (r *Reconciler) checkDeadOutbox(ctx context.Context) {
	var count int64
	if err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM outbox WHERE status = 'DEAD'`,
	).Scan(&count); err != nil {
		r.log.Error().Err(err).Msg("reconciler: count dead outbox events")
		return
	}
	if count == 0 {
		return
	}

	r.log.Warn().
		Int64("dead_events", count).
		Msg("reconciler: DEAD outbox events detected — resetting to PENDING for retry")

	r.requeueDead(ctx)
}

// requeueDead resets all DEAD outbox events back to PENDING so the poller
// retries them. Attempts counter is reset to give each event a fresh window.
func (r *Reconciler) requeueDead(ctx context.Context) {
	res, err := r.db.ExecContext(ctx,
		`UPDATE outbox SET status = 'PENDING', attempts = 0 WHERE status = 'DEAD'`)
	if err != nil {
		r.log.Error().Err(err).Msg("reconciler: requeue dead outbox events failed")
		return
	}
	n, _ := res.RowsAffected()
	r.log.Info().Int64("requeued", n).Msg("reconciler: dead outbox events requeued to PENDING")
}
