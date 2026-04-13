package upsert

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/lib/pq"
	"github.com/yourusername/search-system/internal/outbox"
)

// batchUpsertSQL performs an atomic bulk upsert via unnest arrays.
//
// Rules:
//   - INSERT on new records (no conflict)
//   - UPDATE only when checksum differs OR record was previously soft-deleted
//   - No-op (skip) when checksum matches and record is active
//
// RETURNING + xmax lets us distinguish inserts from updates in the aggregate.
const batchUpsertSQL = `
WITH upsert AS (
    INSERT INTO records (
        id, dataset_id, external_id, source,
        name, value, checksum, sync_token,
        version, created_at, updated_at
    )
    SELECT
        r.id,
        $2::uuid,
        r.external_id,
        r.source,
        r.name,
        r.val::jsonb,
        r.checksum,
        NULLIF(r.sync_token, ''),
        1, NOW(), NOW()
    FROM unnest(
        $1::text[], $3::text[], $4::text[], $5::text[], $6::text[], $7::text[], $8::text[]
    ) AS r(id, external_id, source, name, val, checksum, sync_token)
    ON CONFLICT (id) DO UPDATE SET
        name       = EXCLUDED.name,
        value      = EXCLUDED.value,
        checksum   = EXCLUDED.checksum,
        sync_token = EXCLUDED.sync_token,
        version    = records.version + 1,
        deleted_at = NULL,
        updated_at = NOW()
    WHERE
        records.checksum != EXCLUDED.checksum
        OR records.deleted_at IS NOT NULL
    RETURNING id, (xmax::text::bigint = 0) AS is_insert
)
SELECT
    count(*)::int                                  AS affected,
    count(*) FILTER (WHERE is_insert)::int         AS inserted,
    count(*) FILTER (WHERE NOT is_insert)::int     AS updated
FROM upsert`

// batchOutboxSQL inserts one outbox event per record, all in the same transaction.
const batchOutboxSQL = `
INSERT INTO outbox (event_type, dataset_id, payload)
SELECT 'records.upserted', $1::uuid, unnest($2::text[])::jsonb`

// Record is the canonical input unit for the upsert engine.
type Record struct {
	ExternalID string      `json:"external_id"`
	Source     string      `json:"source"`
	Name       string      `json:"name"`
	Value      interface{} `json:"value"`
	SyncToken  string      `json:"sync_token,omitempty"`

	// Computed fields — set by the engine, not provided by API callers.
	ID        string `json:"-"`
	Checksum  string `json:"-"`
	DatasetID string `json:"-"`
}

// UpsertResult summarises a completed bulk upsert operation.
type UpsertResult struct {
	DatasetID  string `json:"dataset_id"`
	Total      int    `json:"total"`
	Inserted   int    `json:"inserted"`
	Updated    int    `json:"updated"`
	Skipped    int    `json:"skipped"`
	Failed     int    `json:"failed"`
	DurationMs int64  `json:"duration_ms"`
}

// batchResult is an internal per-batch summary.
type batchResult struct {
	inserted int
	updated  int
	skipped  int
	failed   int
}

// Config holds engine tuning parameters sourced from config.yaml.
type Config struct {
	BatchSize   int
	WorkerCount int
}

// UpsertEngine processes bulk record upserts idempotently.
type UpsertEngine struct {
	db     *sql.DB
	outbox *outbox.Writer
	cfg    Config
}

// New constructs an UpsertEngine.
func New(db *sql.DB, ob *outbox.Writer, cfg Config) *UpsertEngine {
	return &UpsertEngine{db: db, outbox: ob, cfg: cfg}
}

// prepareRecords computes deterministic ID and content checksum for every record.
// This is CPU-only work — no DB calls.
func prepareRecords(datasetID string, records []Record) {
	for i := range records {
		records[i].DatasetID = datasetID
		records[i].ID = GenerateID(records[i].ExternalID, records[i].Source)
		records[i].Checksum = GenerateChecksum(records[i].Name, records[i].Value)
	}
}

// processBatch upserts one batch atomically:
//  1. Bulk-upsert all records via unnest CTE.
//  2. Write one outbox event per record in the same transaction.
//
// Either both steps commit or both roll back — outbox always mirrors records.
func (e *UpsertEngine) processBatch(ctx context.Context, datasetID string, batch []Record) batchResult {
	result := batchResult{}

	tx, err := e.db.BeginTx(ctx, nil)
	if err != nil {
		result.failed = len(batch)
		return result
	}
	defer tx.Rollback() //nolint:errcheck

	// Build parallel arrays for the unnest call.
	ids := make([]string, len(batch))
	extIDs := make([]string, len(batch))
	sources := make([]string, len(batch))
	names := make([]string, len(batch))
	values := make([]string, len(batch))
	checksums := make([]string, len(batch))
	syncTokens := make([]string, len(batch))
	payloads := make([]string, len(batch))

	for i, r := range batch {
		v, _ := json.Marshal(r.Value)
		p, _ := json.Marshal(map[string]interface{}{
			"id":          r.ID,
			"dataset_id":  datasetID,
			"external_id": r.ExternalID,
			"source":      r.Source,
		})
		ids[i] = r.ID
		extIDs[i] = r.ExternalID
		sources[i] = r.Source
		names[i] = r.Name
		values[i] = string(v)
		checksums[i] = r.Checksum
		syncTokens[i] = r.SyncToken
		payloads[i] = string(p)
	}

	// ── Step 1: bulk upsert ──────────────────────────────────────────────────
	var affected, inserted, updated int
	err = tx.QueryRowContext(ctx, batchUpsertSQL,
		pq.Array(ids),
		datasetID,
		pq.Array(extIDs),
		pq.Array(sources),
		pq.Array(names),
		pq.Array(values),
		pq.Array(checksums),
		pq.Array(syncTokens),
	).Scan(&affected, &inserted, &updated)
	if err != nil {
		result.failed = len(batch)
		return result
	}

	// ── Step 2: write outbox events in the same transaction ──────────────────
	if _, err = tx.ExecContext(ctx, batchOutboxSQL, datasetID, pq.Array(payloads)); err != nil {
		result.failed = len(batch)
		return result
	}

	if err = tx.Commit(); err != nil {
		result.failed = len(batch)
		return result
	}

	result.inserted = inserted
	result.updated = updated
	result.skipped = len(batch) - affected
	return result
}

// splitBatches divides records into chunks of at most n items.
func splitBatches(records []Record, n int) [][]Record {
	if n <= 0 {
		n = 1000
	}
	batches := make([][]Record, 0, (len(records)+n-1)/n)
	for len(records) > 0 {
		size := n
		if size > len(records) {
			size = len(records)
		}
		batches = append(batches, records[:size])
		records = records[size:]
	}
	return batches
}

// now is a replaceable time source for testing.
var now = time.Now
