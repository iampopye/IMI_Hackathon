-- Migration 001: Core tables — datasets, records, dataset_states

-- Dataset registry
CREATE TABLE datasets (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       VARCHAR(255) NOT NULL UNIQUE,
    source     VARCHAR(255),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- All records across all datasets
CREATE TABLE records (
    id          VARCHAR(64) PRIMARY KEY,           -- SHA-256 of natural key
    dataset_id  UUID        NOT NULL REFERENCES datasets(id),
    external_id VARCHAR(255) NOT NULL,
    source      VARCHAR(255) NOT NULL,
    name        TEXT,
    value       JSONB,
    checksum    VARCHAR(64) NOT NULL,              -- content hash for idempotency
    version     BIGINT      DEFAULT 1,
    deleted_at  TIMESTAMP   DEFAULT NULL,          -- soft delete (NULL = active)
    sync_token  VARCHAR(64),                       -- batch sync identifier
    created_at  TIMESTAMP   DEFAULT NOW(),
    updated_at  TIMESTAMP   DEFAULT NOW(),
    UNIQUE(external_id, source)
);

-- Performance indexes
CREATE INDEX idx_records_dataset_id  ON records(dataset_id);
CREATE INDEX idx_records_external_id ON records(external_id);
CREATE INDEX idx_records_deleted_at  ON records(deleted_at) WHERE deleted_at IS NOT NULL;
CREATE INDEX idx_records_updated_at  ON records(updated_at);
CREATE INDEX idx_records_sync_token  ON records(sync_token);

-- Dataset state tracking
CREATE TABLE dataset_states (
    dataset_id      UUID     PRIMARY KEY REFERENCES datasets(id),
    is_sorted       BOOLEAN  DEFAULT FALSE,
    last_modified   TIMESTAMP DEFAULT NOW(),
    last_sorted     TIMESTAMP,
    stability_score FLOAT    DEFAULT 0.0,          -- 0.0 volatile → 1.0 very stable
    current_tier    VARCHAR(20) DEFAULT 'small',   -- small, medium, large
    is_sorting      BOOLEAN  DEFAULT FALSE         -- sort in progress flag
);
