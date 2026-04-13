package dataset

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
)

// Profiler evaluates the search tier for a dataset using its record count plus
// hysteresis to prevent oscillation at tier boundaries.
//
// Hysteresis rule: upgrading to a higher tier requires TierUpgradeConfirmations
// consecutive evaluations above the threshold.  This counter is kept in-memory
// (Phase 7 replaces it with Redis for multi-instance consistency).
type Profiler struct {
	db            *sql.DB
	metaStore     *MetaStore
	inMemoryLimit int64 // from SearchConfig.InMemoryLimit
	fileLimit     int64 // from SearchConfig.BleveFileLimit
	confirmations int   // from SearchConfig.TierUpgradeConfirmations

	mu        sync.Mutex
	hCounters map[string]int64 // datasetID → consecutive boundary-crossing count
}

// NewProfiler creates a Profiler driven by config values.
func NewProfiler(db *sql.DB, metaStore *MetaStore, inMemoryLimit, fileLimit int64, confirmations int) *Profiler {
	if confirmations <= 0 {
		confirmations = 5
	}
	if inMemoryLimit <= 0 {
		inMemoryLimit = InMemoryLimit
	}
	if fileLimit <= 0 {
		fileLimit = BleveFileLimit
	}
	return &Profiler{
		db:            db,
		metaStore:     metaStore,
		inMemoryLimit: inMemoryLimit,
		fileLimit:     fileLimit,
		confirmations: confirmations,
		hCounters:     make(map[string]int64),
	}
}

// GetCount returns the active record count for a dataset from the O(1)
// dataset_counts trigger table (maintained automatically by trg_dataset_count).
func (p *Profiler) GetCount(ctx context.Context, datasetID string) (int64, error) {
	const q = `SELECT record_count FROM dataset_counts WHERE dataset_id = $1`
	var count int64
	err := p.db.QueryRowContext(ctx, q, datasetID).Scan(&count)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("profiler count %s: %w", datasetID, err)
	}
	return count, nil
}

// EvaluateTier returns the search tier for a dataset, applying hysteresis so a
// dataset that sits at exactly 100K records does not oscillate on every write.
func (p *Profiler) EvaluateTier(ctx context.Context, datasetID string) (SearchTier, error) {
	count, err := p.GetCount(ctx, datasetID)
	if err != nil {
		return TierSmall, err
	}

	meta, err := p.metaStore.Get(ctx, datasetID)
	if err != nil {
		return TierSmall, err
	}

	return p.applyHysteresis(datasetID, count, meta.CurrentTier), nil
}

// applyHysteresis is the pure (no-I/O) tier decision with hysteresis logic.
func (p *Profiler) applyHysteresis(datasetID string, count int64, currentTier SearchTier) SearchTier {
	p.mu.Lock()
	defer p.mu.Unlock()

	switch {
	case count < p.inMemoryLimit:
		// Clearly small — reset any pending upgrade counter.
		delete(p.hCounters, datasetID)
		return TierSmall

	case count < p.fileLimit:
		// Medium territory.  Require N consecutive confirmations before upgrading
		// from small (prevents flapping around the 100K boundary).
		if currentTier == TierSmall {
			p.hCounters[datasetID]++
			if p.hCounters[datasetID] < int64(p.confirmations) {
				return TierSmall // not confirmed enough times yet — stay put
			}
		}
		delete(p.hCounters, datasetID)
		return TierMedium

	default:
		// Large territory.  Same hysteresis guard for the 5M boundary.
		if currentTier == TierMedium {
			p.hCounters[datasetID]++
			if p.hCounters[datasetID] < int64(p.confirmations) {
				return TierMedium
			}
		}
		delete(p.hCounters, datasetID)
		return TierLarge
	}
}

// UpdateTier evaluates the tier and persists it to dataset_states if it changed.
func (p *Profiler) UpdateTier(ctx context.Context, datasetID string) error {
	meta, err := p.metaStore.Get(ctx, datasetID)
	if err != nil {
		return err
	}

	newTier, err := p.EvaluateTier(ctx, datasetID)
	if err != nil {
		return err
	}

	if meta.CurrentTier == newTier {
		return nil // nothing to update
	}

	meta.CurrentTier = newTier
	return p.metaStore.Save(ctx, meta)
}
