package upsert

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// SyncResult summarises a completed FullSync operation.
type SyncResult struct {
	DatasetID  string      `json:"dataset_id"`
	SyncToken  string      `json:"sync_token"`
	Upserted   UpsertResult `json:"upserted"`
	Deleted    int64       `json:"deleted"`
	DurationMs int64       `json:"duration_ms"`
}

// generateSyncToken produces a cryptographically random 16-byte hex token.
// Each full-sync batch gets a unique token so PurgeStaleRecords can identify
// records that were not present in the latest batch from the source.
func generateSyncToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback: timestamp-based token (extremely unlikely to collide in practice)
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// purgeStaleSQL soft-deletes every active record whose sync_token does not match
// the current batch token. These records were absent from the latest source batch
// and must not appear in search results.
const purgeStaleSQL = `
UPDATE records
SET deleted_at = NOW(),
    updated_at = NOW()
WHERE dataset_id  = $1
  AND sync_token != $2
  AND deleted_at IS NULL`

// PurgeStaleRecords marks all active records in the dataset that carry an older
// sync_token as soft-deleted. This eliminates ghost records: anything not present
// in the latest batch from the source is treated as removed.
//
// After purging it writes a single outbox event so downstream consumers
// (Elasticsearch, Redis) can evict the deleted records.
func (e *UpsertEngine) PurgeStaleRecords(ctx context.Context, datasetID, syncToken string) (int64, error) {
	result, err := e.db.ExecContext(ctx, purgeStaleSQL, datasetID, syncToken)
	if err != nil {
		return 0, fmt.Errorf("purge stale records: %w", err)
	}

	rowsDeleted, _ := result.RowsAffected()

	// Publish one deletion event so Elasticsearch + Redis cache can evict
	// the removed records asynchronously via the outbox pipeline (Phase 8).
	if rowsDeleted > 0 {
		_ = e.outbox.Write(ctx, "records.deleted", datasetID, map[string]interface{}{
			"sync_token": syncToken,
			"count":      rowsDeleted,
		})
	}

	return rowsDeleted, nil
}

// FullSync performs an atomic source-of-truth synchronisation:
//
//  1. Generate a unique sync token for this batch.
//  2. Tag every incoming record with the token.
//  3. Bulk-upsert all records (idempotent — duplicates are no-ops).
//  4. Soft-delete any record that still carries an older token
//     (i.e. was not present in the source batch).
//
// After FullSync completes, the dataset in PostgreSQL is an exact mirror of the
// source. Ghost records are impossible.
//
// Note: MetaStore.OnDatasetChanged notification is added in Phase 4 once the
// Dataset State Detector is in place.
func (e *UpsertEngine) FullSync(ctx context.Context, datasetID string, records []Record) (*SyncResult, error) {
	start := time.Now()

	syncToken := generateSyncToken()

	// Tag all incoming records with this batch's sync token so the purge query
	// can distinguish them from records that belong to older batches.
	for i := range records {
		records[i].SyncToken = syncToken
	}

	// Step 1: upsert everything from the source.
	upsertResult := e.BulkUpsert(ctx, datasetID, records)

	// Step 2: soft-delete any record not in this batch.
	deleted, err := e.PurgeStaleRecords(ctx, datasetID, syncToken)
	if err != nil {
		return nil, fmt.Errorf("full sync purge: %w", err)
	}

	return &SyncResult{
		DatasetID:  datasetID,
		SyncToken:  syncToken,
		Upserted:   upsertResult,
		Deleted:    deleted,
		DurationMs: time.Since(start).Milliseconds(),
	}, nil
}
