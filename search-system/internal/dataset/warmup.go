package dataset

import (
	"context"
	"database/sql"
	"sync"

	"github.com/rs/zerolog"
)

// Warmup preloads the most-accessed datasets at startup to eliminate the
// cold-start latency that would otherwise occur on the first query.
//
// It reads dataset_access_log for the past 7 days, ranks by access frequency,
// and triggers a preload for the top-N datasets (max 5 concurrently).
// SetPreloader wires in the actual Bleve index loader from the search package.
type Warmup struct {
	db             *sql.DB
	warmupDatasets int
	log            zerolog.Logger
	preloadFn      func(ctx context.Context, datasetID string) // set by SetPreloader
}

// NewWarmup creates a Warmup runner. warmupDatasets is the maximum number of
// datasets to preload (from AppConfig.WarmupDatasets, default 50).
func NewWarmup(db *sql.DB, warmupDatasets int, log zerolog.Logger) *Warmup {
	if warmupDatasets <= 0 {
		warmupDatasets = 50
	}
	return &Warmup{db: db, warmupDatasets: warmupDatasets, log: log}
}

// Run queries the access log and preloads the top-N datasets.
// Call once during server startup (before the HTTP listener starts).
func (w *Warmup) Run(ctx context.Context) {
	topIDs, err := w.topAccessedDatasets(ctx)
	if err != nil {
		w.log.Error().Err(err).Msg("warmup: query access log")
		return
	}
	if len(topIDs) == 0 {
		w.log.Info().Msg("warmup: no datasets in access log — skipping")
		return
	}

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 5) // max 5 parallel preloads

	for _, dsID := range topIDs {
		wg.Add(1)
		go func(datasetID string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			w.preloadDataset(ctx, datasetID)
		}(dsID)
	}

	wg.Wait()
	w.log.Info().Int("datasets_loaded", len(topIDs)).Msg("warmup complete")
}

// topAccessedDatasets returns the IDs of the most-accessed datasets in the
// last 7 days, ordered by hit count descending.
func (w *Warmup) topAccessedDatasets(ctx context.Context) ([]string, error) {
	const q = `
		SELECT dataset_id::text, COUNT(*) AS hits
		FROM   dataset_access_log
		WHERE  accessed_at > NOW() - INTERVAL '7 days'
		GROUP  BY dataset_id
		ORDER  BY hits DESC
		LIMIT  $1`

	rows, err := w.db.QueryContext(ctx, q, w.warmupDatasets)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		var hits int64
		if err := rows.Scan(&id, &hits); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// SetPreloader sets the function called for each dataset during warmup.
// The search router provides this via SmartSearchRouter.PreloadDataset.
// If not set, warmup logs intent only (safe for unit tests without Bleve).
func (w *Warmup) SetPreloader(fn func(ctx context.Context, datasetID string)) {
	w.preloadFn = fn
}

// preloadDataset calls the registered preloader or logs a placeholder message.
func (w *Warmup) preloadDataset(ctx context.Context, datasetID string) {
	if w.preloadFn != nil {
		w.preloadFn(ctx, datasetID)
		return
	}
	w.log.Info().Str("dataset", datasetID).Msg("warmup: no preloader set — skipping index build")
}

// LogAccess records one access hit for a dataset in dataset_access_log.
// Called by the search router (Phase 5) on every query so warmup stays current.
func LogAccess(ctx context.Context, db *sql.DB, datasetID string) {
	db.ExecContext(ctx, //nolint:errcheck
		`INSERT INTO dataset_access_log (dataset_id) VALUES ($1)`, datasetID)
}
