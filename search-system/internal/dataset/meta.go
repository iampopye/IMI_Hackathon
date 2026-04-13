package dataset

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"time"
)

// DatasetMeta is the live state of a dataset as tracked by the state detector.
type DatasetMeta struct {
	DatasetID      string
	StabilityScore float64
	IsSorted       bool
	IsSorting      bool
	LastModified   time.Time
	CurrentTier    SearchTier
}

// IsStable returns true when the stability score meets or exceeds the threshold.
func (m *DatasetMeta) IsStable() bool {
	return m.StabilityScore >= StabilityThreshold
}

// MetaStore reads and writes DatasetMeta to/from the dataset_states table.
type MetaStore struct {
	db                 *sql.DB
	stabilityThreshold float64
	stabilityTick      float64
	stabilityDecay     float64
}

// NewMetaStore creates a MetaStore. Pass config values so the thresholds are
// driven by config.yaml rather than compile-time constants.
func NewMetaStore(db *sql.DB, threshold, tick, decay float64) *MetaStore {
	if threshold == 0 {
		threshold = StabilityThreshold
	}
	if tick == 0 {
		tick = StabilityTick
	}
	if decay == 0 {
		decay = StabilityDecay
	}
	return &MetaStore{db: db, stabilityThreshold: threshold, stabilityTick: tick, stabilityDecay: decay}
}

// Get retrieves the current meta for a dataset. Returns a zero-value meta if no
// row exists yet (dataset_states row is created by the createDataset SQL in the API).
func (s *MetaStore) Get(ctx context.Context, datasetID string) (*DatasetMeta, error) {
	const q = `
		SELECT stability_score, is_sorted, is_sorting, last_modified, current_tier
		FROM   dataset_states
		WHERE  dataset_id = $1`

	var (
		score   float64
		sorted  bool
		sorting bool
		lastMod time.Time
		tier    string
	)

	err := s.db.QueryRowContext(ctx, q, datasetID).Scan(&score, &sorted, &sorting, &lastMod, &tier)
	if err == sql.ErrNoRows {
		return &DatasetMeta{
			DatasetID:      datasetID,
			StabilityScore: 0.0,
			LastModified:   time.Now(),
			CurrentTier:    TierSmall,
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("metastore get %s: %w", datasetID, err)
	}

	return &DatasetMeta{
		DatasetID:      datasetID,
		StabilityScore: score,
		IsSorted:       sorted,
		IsSorting:      sorting,
		LastModified:   lastMod,
		CurrentTier:    tierFromString(tier),
	}, nil
}

// Save persists meta back to dataset_states.
func (s *MetaStore) Save(ctx context.Context, meta *DatasetMeta) error {
	const q = `
		UPDATE dataset_states
		SET    stability_score = $1,
		       is_sorted       = $2,
		       is_sorting      = $3,
		       last_modified   = $4,
		       current_tier    = $5
		WHERE  dataset_id = $6`

	_, err := s.db.ExecContext(ctx, q,
		meta.StabilityScore,
		meta.IsSorted,
		meta.IsSorting,
		meta.LastModified,
		meta.CurrentTier.String(),
		meta.DatasetID,
	)
	if err != nil {
		return fmt.Errorf("metastore save %s: %w", meta.DatasetID, err)
	}
	return nil
}

// OnDatasetChanged decays the stability score when a record is written.
// Must be called after every upsert or delete that touches this dataset.
func (s *MetaStore) OnDatasetChanged(ctx context.Context, datasetID string) error {
	meta, err := s.Get(ctx, datasetID)
	if err != nil {
		return err
	}
	meta.StabilityScore = meta.StabilityScore * s.stabilityDecay
	meta.LastModified = time.Now()
	meta.IsSorted = false
	return s.Save(ctx, meta)
}

// OnDatasetSorted marks the dataset as fully sorted after a successful B-Tree build.
func (s *MetaStore) OnDatasetSorted(ctx context.Context, datasetID string) error {
	meta, err := s.Get(ctx, datasetID)
	if err != nil {
		return err
	}
	meta.IsSorted = true
	meta.IsSorting = false
	return s.Save(ctx, meta)
}

// TickStability nudges the score upward each monitor cycle (no changes observed).
// Capped at 1.0.
func (s *MetaStore) TickStability(ctx context.Context, datasetID string) error {
	meta, err := s.Get(ctx, datasetID)
	if err != nil {
		return err
	}
	if meta.StabilityScore < 1.0 {
		meta.StabilityScore = math.Min(1.0, meta.StabilityScore+s.stabilityTick)
	}
	return s.Save(ctx, meta)
}
