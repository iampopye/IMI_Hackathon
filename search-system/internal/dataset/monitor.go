package dataset

import (
	"context"
	"database/sql"
	"time"

	"github.com/rs/zerolog"
)

const defaultMonitorInterval = time.Minute

// Monitor runs in the background, ticking stability scores, updating tiers, and
// triggering B-Tree builds under PostgreSQL advisory locks (no race conditions
// across multiple server replicas).
type Monitor struct {
	db        *sql.DB
	metaStore *MetaStore
	profiler  *Profiler
	interval  time.Duration
	log       zerolog.Logger
}

// NewMonitor creates a Monitor with a 1-minute poll interval.
func NewMonitor(db *sql.DB, metaStore *MetaStore, profiler *Profiler, log zerolog.Logger) *Monitor {
	return &Monitor{
		db:        db,
		metaStore: metaStore,
		profiler:  profiler,
		interval:  defaultMonitorInterval,
		log:       log,
	}
}

// Start runs the monitor loop until ctx is cancelled.  Call in a goroutine.
func (m *Monitor) Start(ctx context.Context) {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	m.log.Info().Dur("interval", m.interval).Msg("dataset monitor started")

	for {
		select {
		case <-ctx.Done():
			m.log.Info().Msg("dataset monitor stopped")
			return
		case <-ticker.C:
			m.tick(ctx)
		}
	}
}

// tick runs one monitor cycle: stability tick → tier update → maybe sort.
func (m *Monitor) tick(ctx context.Context) {
	ids, err := m.allDatasetIDs(ctx)
	if err != nil {
		m.log.Error().Err(err).Msg("monitor: fetch dataset IDs")
		return
	}

	for _, id := range ids {
		if err := m.metaStore.TickStability(ctx, id); err != nil {
			m.log.Error().Err(err).Str("dataset", id).Msg("monitor: tick stability")
			continue
		}
		if err := m.profiler.UpdateTier(ctx, id); err != nil {
			m.log.Error().Err(err).Str("dataset", id).Msg("monitor: update tier")
		}
		m.maybeSort(ctx, id)
	}
}

// allDatasetIDs returns every dataset_id present in dataset_states.
func (m *Monitor) allDatasetIDs(ctx context.Context) ([]string, error) {
	rows, err := m.db.QueryContext(ctx, `SELECT dataset_id::text FROM dataset_states`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// maybeSort triggers a B-Tree build if the dataset is stable and not yet sorted.
func (m *Monitor) maybeSort(ctx context.Context, datasetID string) {
	meta, err := m.metaStore.Get(ctx, datasetID)
	if err != nil || !meta.IsStable() || meta.IsSorted || meta.IsSorting {
		return
	}
	go m.triggerSort(ctx, datasetID)
}

// triggerSort acquires a PostgreSQL advisory lock and performs the B-Tree build.
// Only one instance across all replicas can hold the lock at a time — the others
// detect the lock is taken and return immediately without duplicating work.
func (m *Monitor) triggerSort(ctx context.Context, datasetID string) {
	var locked bool
	if err := m.db.QueryRowContext(
		ctx, `SELECT pg_try_advisory_lock(hashtext($1))`, datasetID,
	).Scan(&locked); err != nil || !locked {
		return // another replica is already sorting this dataset
	}
	defer m.db.ExecContext(ctx, `SELECT pg_advisory_unlock(hashtext($1))`, datasetID) //nolint:errcheck

	if _, err := m.db.ExecContext(ctx,
		`UPDATE dataset_states SET is_sorting = TRUE WHERE dataset_id = $1`, datasetID,
	); err != nil {
		m.log.Error().Err(err).Str("dataset", datasetID).Msg("monitor: mark is_sorting")
		return
	}

	// performBTreeBuild is the hook for Phase 5.
	// In Phase 4 it succeeds immediately so state transitions are fully exercised.
	err := m.performBTreeBuild(ctx, datasetID)

	if err == nil {
		m.db.ExecContext(ctx, `
			UPDATE dataset_states
			SET    is_sorting = FALSE, is_sorted = TRUE, last_sorted = NOW()
			WHERE  dataset_id = $1`, datasetID) //nolint:errcheck
		m.log.Info().Str("dataset", datasetID).Msg("monitor: sort complete")
	} else {
		m.db.ExecContext(ctx,
			`UPDATE dataset_states SET is_sorting = FALSE WHERE dataset_id = $1`, datasetID) //nolint:errcheck
		m.log.Error().Err(err).Str("dataset", datasetID).Msg("monitor: sort failed, will retry")
	}
}

// performBTreeBuild is the extension point for Phase 5.
// It is intentionally a no-op in Phase 4 so the advisory lock and state
// machine infrastructure can be tested end-to-end without a real index.
func (m *Monitor) performBTreeBuild(_ context.Context, _ string) error {
	return nil
}
