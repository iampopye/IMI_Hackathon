-- Migration 002: Transactional outbox table

CREATE TABLE outbox (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type   VARCHAR(100) NOT NULL,            -- records.upserted, records.deleted
    dataset_id   UUID        NOT NULL,
    payload      JSONB       NOT NULL,
    status       VARCHAR(20) DEFAULT 'PENDING',    -- PENDING, PUBLISHED, FAILED
    attempts     INT         DEFAULT 0,
    created_at   TIMESTAMP   DEFAULT NOW(),
    published_at TIMESTAMP
);

CREATE INDEX idx_outbox_status     ON outbox(status) WHERE status = 'PENDING';
CREATE INDEX idx_outbox_created_at ON outbox(created_at);
