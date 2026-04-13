-- Migration 005: Performance indexes for soft delete and sync token operations

-- Fast scan for active records within a dataset (most common query pattern)
CREATE INDEX idx_records_active_by_dataset
    ON records(dataset_id, updated_at)
    WHERE deleted_at IS NULL;

-- Fast scan for soft-deleted records (used by reconciler and reporting)
CREATE INDEX idx_records_soft_deleted
    ON records(dataset_id, deleted_at)
    WHERE deleted_at IS NOT NULL;

-- Composite index for PurgeStaleRecords:
--   WHERE dataset_id = $1 AND sync_token != $2 AND deleted_at IS NULL
CREATE INDEX idx_records_purge_lookup
    ON records(dataset_id, sync_token)
    WHERE deleted_at IS NULL;
