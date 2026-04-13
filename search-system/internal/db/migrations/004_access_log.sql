-- Migration 004: Dataset access log for startup warmup
-- Top-N most recently accessed datasets are preloaded into Bleve on startup
-- to eliminate cold-start latency on first query.

CREATE TABLE dataset_access_log (
    id          BIGSERIAL PRIMARY KEY,
    dataset_id  UUID      NOT NULL,
    accessed_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_access_log_dataset ON dataset_access_log(dataset_id, accessed_at);

-- Note: entries older than 30 days are purged by the background reconciler job.
