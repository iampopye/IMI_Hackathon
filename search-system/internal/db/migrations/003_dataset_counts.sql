-- Migration 003: Trigger-based dataset record counts (replaces COUNT(*))
-- O(1) lookup vs. O(n) COUNT(*) on 100M rows (3–8 second scan eliminated).

CREATE TABLE dataset_counts (
    dataset_id   UUID   PRIMARY KEY REFERENCES datasets(id),
    record_count BIGINT DEFAULT 0,
    updated_at   TIMESTAMP DEFAULT NOW()
);

-- Trigger function: auto-increment/decrement on insert or soft-delete
CREATE OR REPLACE FUNCTION maintain_dataset_count()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' AND NEW.deleted_at IS NULL THEN
        INSERT INTO dataset_counts (dataset_id, record_count)
        VALUES (NEW.dataset_id, 1)
        ON CONFLICT (dataset_id)
        DO UPDATE SET
            record_count = dataset_counts.record_count + 1,
            updated_at   = NOW();

    ELSIF TG_OP = 'UPDATE' THEN
        -- Record soft-deleted
        IF OLD.deleted_at IS NULL AND NEW.deleted_at IS NOT NULL THEN
            UPDATE dataset_counts
            SET record_count = GREATEST(0, record_count - 1),
                updated_at   = NOW()
            WHERE dataset_id = NEW.dataset_id;
        END IF;
        -- Record restored from soft-delete
        IF OLD.deleted_at IS NOT NULL AND NEW.deleted_at IS NULL THEN
            UPDATE dataset_counts
            SET record_count = record_count + 1,
                updated_at   = NOW()
            WHERE dataset_id = NEW.dataset_id;
        END IF;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_dataset_count
AFTER INSERT OR UPDATE ON records
FOR EACH ROW EXECUTE FUNCTION maintain_dataset_count();
