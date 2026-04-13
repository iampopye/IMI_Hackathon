package outbox

import (
	"context"
	"database/sql"
	"encoding/json"
)

const insertSQL = `
INSERT INTO outbox (event_type, dataset_id, payload)
VALUES ($1, $2, $3)`

// Writer inserts events into the transactional outbox table.
// The outbox poller (Phase 8) reads these rows and publishes them to Kafka.
type Writer struct {
	db *sql.DB
}

// New constructs a Writer backed by the given connection pool.
func New(db *sql.DB) *Writer {
	return &Writer{db: db}
}

// Write inserts a single outbox event outside an existing transaction.
// Use this for standalone events (e.g. soft-delete notifications).
func (w *Writer) Write(ctx context.Context, eventType, datasetID string, payload interface{}) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = w.db.ExecContext(ctx, insertSQL, eventType, datasetID, b)
	return err
}

// WriteTx inserts a single outbox event within an existing transaction.
// Use this when you need the outbox write to be atomic with another DB operation.
func (w *Writer) WriteTx(ctx context.Context, tx *sql.Tx, eventType, datasetID string, payload interface{}) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, insertSQL, eventType, datasetID, b)
	return err
}
