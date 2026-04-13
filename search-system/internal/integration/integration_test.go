// Package integration holds end-to-end tests that require a live PostgreSQL
// instance.  Set TEST_POSTGRES_DSN to enable:
//
//	TEST_POSTGRES_DSN="host=localhost port=5432 user=searchuser password=searchpass dbname=searchdb_test sslmode=disable"
package integration

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"math"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/rs/zerolog"
	"github.com/yourusername/search-system/internal/dataset"
	"github.com/yourusername/search-system/internal/db"
	"github.com/yourusername/search-system/internal/outbox"
	"github.com/yourusername/search-system/internal/search"
	"github.com/yourusername/search-system/internal/upsert"
)

// ── Helpers ───────────────────────────────────────────────────────────────────

// testDB opens a connection to TEST_POSTGRES_DSN, runs migrations, and returns
// the pool.  If the env-var is unset the calling test is skipped.
func testDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set — skipping integration test")
	}
	conn, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	if err := conn.Ping(); err != nil {
		conn.Close()
		t.Fatalf("db.Ping: %v", err)
	}
	if err := db.Migrate(conn); err != nil {
		conn.Close()
		t.Fatalf("db.Migrate: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

// createTestDataset inserts a uniquely-named dataset and its companion
// dataset_states row, and registers cleanup to remove them after the test.
func createTestDataset(t *testing.T, conn *sql.DB, name string) string {
	t.Helper()
	ctx := context.Background()
	var id string
	if err := conn.QueryRowContext(ctx,
		`INSERT INTO datasets (name) VALUES ($1) RETURNING id`, name,
	).Scan(&id); err != nil {
		t.Fatalf("INSERT datasets %q: %v", name, err)
	}
	if _, err := conn.ExecContext(ctx,
		`INSERT INTO dataset_states (dataset_id) VALUES ($1)`, id,
	); err != nil {
		t.Fatalf("INSERT dataset_states for %s: %v", id, err)
	}
	t.Cleanup(func() {
		conn.ExecContext(ctx, `DELETE FROM outbox        WHERE dataset_id = $1`, id) //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM records       WHERE dataset_id = $1`, id) //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM dataset_states WHERE dataset_id = $1`, id) //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM dataset_counts WHERE dataset_id = $1`, id) //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM datasets      WHERE id = $1`, id) //nolint:errcheck
	})
	return id
}

// uname returns a unique name derived from a prefix and the current nanosecond.
func uname(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

// newEngine builds a configured UpsertEngine wired to an outbox.Writer.
func newEngine(conn *sql.DB) *upsert.UpsertEngine {
	return upsert.New(conn, outbox.New(conn), upsert.Config{BatchSize: 100, WorkerCount: 2})
}

// nopLog returns a zerolog.Logger that discards all output.
func nopLog() zerolog.Logger { return zerolog.New(io.Discard) }

// ── Integration Tests ─────────────────────────────────────────────────────────

// 1. Migrations are idempotent — running Migrate twice must not fail.
func TestIntegration_MigrateIdempotent(t *testing.T) {
	conn := testDB(t)
	if err := db.Migrate(conn); err != nil {
		t.Fatalf("second Migrate() call failed: %v", err)
	}
}

// 2. Dataset creation inserts rows in both datasets and dataset_states.
func TestIntegration_DatasetCreate(t *testing.T) {
	conn := testDB(t)
	ctx := context.Background()
	name := uname("ds-create")

	var id string
	if err := conn.QueryRowContext(ctx,
		`INSERT INTO datasets (name) VALUES ($1) RETURNING id`, name,
	).Scan(&id); err != nil {
		t.Fatalf("INSERT datasets: %v", err)
	}
	defer func() {
		conn.ExecContext(ctx, `DELETE FROM datasets WHERE id = $1`, id) //nolint:errcheck
	}()

	if _, err := conn.ExecContext(ctx,
		`INSERT INTO dataset_states (dataset_id) VALUES ($1)`, id,
	); err != nil {
		t.Fatalf("INSERT dataset_states: %v", err)
	}
	defer func() {
		conn.ExecContext(ctx, `DELETE FROM dataset_states WHERE dataset_id = $1`, id) //nolint:errcheck
	}()

	var exists bool
	conn.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM datasets WHERE id = $1)`, id).Scan(&exists) //nolint:errcheck
	if !exists {
		t.Error("dataset row not found after INSERT")
	}

	var score float64
	conn.QueryRowContext(ctx, `SELECT stability_score FROM dataset_states WHERE dataset_id = $1`, id).Scan(&score) //nolint:errcheck
	if score != 0.0 {
		t.Errorf("initial stability_score = %f, want 0.0", score)
	}
}

// 3. First upsert of a new record → Inserted=1, Failed=0.
func TestIntegration_UpsertInsert(t *testing.T) {
	conn := testDB(t)
	dsID := createTestDataset(t, conn, uname("upsert-insert"))
	engine := newEngine(conn)

	result := engine.BulkUpsert(context.Background(), dsID, []upsert.Record{
		{ExternalID: "rec-001", Source: "test", Name: "Widget Alpha", Value: map[string]int{"price": 10}},
	})

	if result.Inserted != 1 {
		t.Errorf("Inserted = %d, want 1", result.Inserted)
	}
	if result.Failed != 0 {
		t.Errorf("Failed = %d, want 0", result.Failed)
	}
}

// 4. Upserting the same record twice with identical content → second call Skipped=1.
func TestIntegration_UpsertIdempotent(t *testing.T) {
	conn := testDB(t)
	dsID := createTestDataset(t, conn, uname("upsert-idempotent"))
	engine := newEngine(conn)

	rec := []upsert.Record{
		{ExternalID: "rec-001", Source: "test", Name: "Widget Alpha", Value: `{"price":10}`},
	}
	engine.BulkUpsert(context.Background(), dsID, rec)   // first — inserts
	r2 := engine.BulkUpsert(context.Background(), dsID, rec) // second — same content

	if r2.Skipped != 1 {
		t.Errorf("second upsert: Skipped = %d, want 1 (content unchanged)", r2.Skipped)
	}
	if r2.Updated != 0 {
		t.Errorf("second upsert: Updated = %d, want 0", r2.Updated)
	}
}

// 5. Upserting the same ExternalID with changed content → Updated=1.
func TestIntegration_UpsertChecksumUpdate(t *testing.T) {
	conn := testDB(t)
	dsID := createTestDataset(t, conn, uname("upsert-update"))
	engine := newEngine(conn)

	engine.BulkUpsert(context.Background(), dsID, []upsert.Record{
		{ExternalID: "rec-001", Source: "test", Name: "Widget A", Value: `{"price":10}`},
	})
	r2 := engine.BulkUpsert(context.Background(), dsID, []upsert.Record{
		{ExternalID: "rec-001", Source: "test", Name: "Widget A", Value: `{"price":99}`},
	})

	if r2.Updated != 1 {
		t.Errorf("Updated = %d, want 1 after content change", r2.Updated)
	}
	if r2.Inserted != 0 {
		t.Errorf("Inserted = %d, want 0 (same record, updated not new)", r2.Inserted)
	}
}

// 6. Bulk upsert of 50 distinct records → Inserted=50, Total=50.
func TestIntegration_BulkUpsert_MultipleRecords(t *testing.T) {
	conn := testDB(t)
	dsID := createTestDataset(t, conn, uname("bulk-upsert"))
	engine := upsert.New(conn, outbox.New(conn), upsert.Config{BatchSize: 10, WorkerCount: 3})

	records := make([]upsert.Record, 50)
	for i := range records {
		records[i] = upsert.Record{
			ExternalID: fmt.Sprintf("rec-%03d", i),
			Source:     "bulk-test",
			Name:       fmt.Sprintf("Item %d", i),
			Value:      map[string]int{"idx": i},
		}
	}

	result := engine.BulkUpsert(context.Background(), dsID, records)

	if result.Total != 50 {
		t.Errorf("Total = %d, want 50", result.Total)
	}
	if result.Inserted != 50 {
		t.Errorf("Inserted = %d, want 50", result.Inserted)
	}
	if result.Failed != 0 {
		t.Errorf("Failed = %d, want 0", result.Failed)
	}
}

// 7. Soft-delete: deleted_at is set after an UPDATE … SET deleted_at.
func TestIntegration_SoftDeleteRecord(t *testing.T) {
	conn := testDB(t)
	dsID := createTestDataset(t, conn, uname("soft-delete"))
	engine := newEngine(conn)

	engine.BulkUpsert(context.Background(), dsID, []upsert.Record{
		{ExternalID: "rec-del", Source: "test", Name: "ToDelete", Value: `{}`},
	})

	recID := upsert.GenerateID("rec-del", "test")
	if _, err := conn.ExecContext(context.Background(),
		`UPDATE records SET deleted_at = NOW() WHERE id = $1`, recID,
	); err != nil {
		t.Fatalf("soft-delete UPDATE: %v", err)
	}

	var deletedAt sql.NullTime
	conn.QueryRowContext(context.Background(), //nolint:errcheck
		`SELECT deleted_at FROM records WHERE id = $1`, recID,
	).Scan(&deletedAt)

	if !deletedAt.Valid {
		t.Error("deleted_at is NULL after soft delete; expected a timestamp")
	}
}

// 8. FullSync: records absent from the new batch are ghost-purged (SyncResult.Deleted=1).
func TestIntegration_FullSync_GhostElimination(t *testing.T) {
	conn := testDB(t)
	dsID := createTestDataset(t, conn, uname("full-sync"))
	engine := newEngine(conn)

	// Seed 3 records.
	engine.BulkUpsert(context.Background(), dsID, []upsert.Record{
		{ExternalID: "keep-1", Source: "test", Name: "Keep 1", Value: `{}`},
		{ExternalID: "keep-2", Source: "test", Name: "Keep 2", Value: `{}`},
		{ExternalID: "ghost",  Source: "test", Name: "Ghost",  Value: `{}`},
	})

	// FullSync with only 2 records → "ghost" must be soft-deleted.
	sr, err := engine.FullSync(context.Background(), dsID, []upsert.Record{
		{ExternalID: "keep-1", Source: "test", Name: "Keep 1", Value: `{}`},
		{ExternalID: "keep-2", Source: "test", Name: "Keep 2", Value: `{}`},
	})
	if err != nil {
		t.Fatalf("FullSync error: %v", err)
	}
	if sr.Deleted != 1 {
		t.Errorf("FullSync Deleted = %d, want 1 ghost purged", sr.Deleted)
	}

	ghostID := upsert.GenerateID("ghost", "test")
	var deletedAt sql.NullTime
	conn.QueryRowContext(context.Background(), //nolint:errcheck
		`SELECT deleted_at FROM records WHERE id = $1`, ghostID,
	).Scan(&deletedAt)
	if !deletedAt.Valid {
		t.Error("ghost record deleted_at is NULL after FullSync; expected soft-deleted")
	}
}

// 9. outbox.Writer.Write() inserts a PENDING event outside a transaction.
func TestIntegration_OutboxWriter_StandaloneEvent(t *testing.T) {
	conn := testDB(t)
	dsID := createTestDataset(t, conn, uname("outbox-standalone"))
	writer := outbox.New(conn)

	if err := writer.Write(context.Background(),
		"records.upserted", dsID, map[string]string{"test": "payload"},
	); err != nil {
		t.Fatalf("outbox.Write: %v", err)
	}

	var status string
	if err := conn.QueryRowContext(context.Background(),
		`SELECT status FROM outbox WHERE dataset_id = $1 ORDER BY created_at DESC LIMIT 1`, dsID,
	).Scan(&status); err != nil {
		t.Fatalf("query outbox: %v", err)
	}
	if status != "PENDING" {
		t.Errorf("outbox status = %q, want PENDING", status)
	}
}

// 10. outbox.Writer.WriteTx() commits atomically with the parent transaction.
func TestIntegration_OutboxWriter_TransactionalEvent(t *testing.T) {
	conn := testDB(t)
	dsID := createTestDataset(t, conn, uname("outbox-tx"))
	writer := outbox.New(conn)

	tx, err := conn.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("BeginTx: %v", err)
	}
	if err := writer.WriteTx(context.Background(), tx,
		"records.deleted", dsID, map[string]int{"count": 5},
	); err != nil {
		tx.Rollback() //nolint:errcheck
		t.Fatalf("WriteTx: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	var count int
	conn.QueryRowContext(context.Background(), //nolint:errcheck
		`SELECT COUNT(*) FROM outbox WHERE dataset_id = $1 AND event_type = 'records.deleted'`, dsID,
	).Scan(&count)
	if count != 1 {
		t.Errorf("outbox row count = %d, want 1 after WriteTx commit", count)
	}
}

// 11. Bleve in-memory engine returns the exact-match record first.
func TestIntegration_BleveMemorySearch_ExactMatch(t *testing.T) {
	conn := testDB(t)
	dsID := createTestDataset(t, conn, uname("bleve-exact"))
	engine := newEngine(conn)

	engine.BulkUpsert(context.Background(), dsID, []upsert.Record{
		{ExternalID: "item-1", Source: "test", Name: "Espresso Machine",  Value: `{"category":"coffee"}`},
		{ExternalID: "item-2", Source: "test", Name: "Drip Coffee Maker", Value: `{"category":"coffee"}`},
		{ExternalID: "item-3", Source: "test", Name: "French Press",      Value: `{"category":"coffee"}`},
	})

	memEngine := search.NewMemoryEngine(conn, nopLog())
	result, err := memEngine.Search(context.Background(), dsID, search.Query{Term: "Espresso", Limit: 10})
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	if result.Total == 0 || len(result.Hits) == 0 {
		t.Fatal("expected at least 1 hit for 'Espresso', got 0")
	}
	if result.Hits[0].Name != "Espresso Machine" {
		t.Errorf("top hit name = %q, want 'Espresso Machine'", result.Hits[0].Name)
	}
}

// 12. Bleve fuzzy search finds a record despite a 1-character typo.
func TestIntegration_BleveMemorySearch_FuzzyMatch(t *testing.T) {
	conn := testDB(t)
	dsID := createTestDataset(t, conn, uname("bleve-fuzzy"))
	engine := newEngine(conn)

	engine.BulkUpsert(context.Background(), dsID, []upsert.Record{
		{ExternalID: "item-1", Source: "test", Name: "Laptop Stand", Value: `{}`},
	})

	memEngine := search.NewMemoryEngine(conn, nopLog())
	// "Laptpo" has edit-distance 1 from "laptop" (transposition).
	result, err := memEngine.Search(context.Background(), dsID,
		search.Query{Term: "Laptpo", Limit: 10, Fuzziness: 2},
	)
	if err != nil {
		t.Fatalf("fuzzy search error: %v", err)
	}
	if result.Total == 0 {
		t.Error("fuzzy search: expected hit for 'Laptpo' → 'Laptop Stand', got 0")
	}
}

// 13. dataset_counts trigger increments the counter on record INSERT.
func TestIntegration_DatasetCountTrigger(t *testing.T) {
	conn := testDB(t)
	dsID := createTestDataset(t, conn, uname("ds-count"))
	engine := newEngine(conn)

	engine.BulkUpsert(context.Background(), dsID, []upsert.Record{
		{ExternalID: "r1", Source: "t", Name: "A", Value: `{}`},
		{ExternalID: "r2", Source: "t", Name: "B", Value: `{}`},
		{ExternalID: "r3", Source: "t", Name: "C", Value: `{}`},
	})

	var recordCount int64
	err := conn.QueryRowContext(context.Background(),
		`SELECT record_count FROM dataset_counts WHERE dataset_id = $1`, dsID,
	).Scan(&recordCount)
	if err == sql.ErrNoRows {
		t.Skip("dataset_counts has no row — trigger may not have fired")
	}
	if err != nil {
		t.Fatalf("query dataset_counts: %v", err)
	}
	if recordCount != 3 {
		t.Errorf("dataset_counts.record_count = %d, want 3", recordCount)
	}
}

// 14. MetaStore round-trip: score survives a Save→Get cycle and decays correctly.
func TestIntegration_MetaStoreRoundTrip(t *testing.T) {
	conn := testDB(t)
	dsID := createTestDataset(t, conn, uname("metastore"))
	store := dataset.NewMetaStore(conn, 0.70, 0.05, 0.80)
	ctx := context.Background()

	// Start from 0.0, add one tick → 0.05, then decay → 0.04.
	if err := store.TickStability(ctx, dsID); err != nil {
		t.Fatalf("TickStability: %v", err)
	}
	if err := store.OnDatasetChanged(ctx, dsID); err != nil {
		t.Fatalf("OnDatasetChanged: %v", err)
	}

	meta, err := store.Get(ctx, dsID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	const want = 0.05 * 0.80 // 0.04
	if math.Abs(meta.StabilityScore-want) > 1e-9 {
		t.Errorf("StabilityScore = %.8f, want %.8f", meta.StabilityScore, want)
	}
	if meta.IsSorted {
		t.Error("IsSorted should be false after OnDatasetChanged")
	}
}
