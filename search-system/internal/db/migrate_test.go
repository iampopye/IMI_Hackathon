package db

import (
	"database/sql"
	"os"
	"testing"

	_ "github.com/lib/pq"
)

// TestMigrate_Integration requires a running PostgreSQL instance.
// Set TEST_POSTGRES_DSN to enable:
//
//	TEST_POSTGRES_DSN="host=localhost port=5432 user=searchuser password=searchpass dbname=searchdb_test sslmode=disable"
func TestMigrate_Integration(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set — skipping integration test")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Fatalf("ping db: %v", err)
	}

	// Run migrations
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	// Running again must be idempotent (no error, no duplicate applies)
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate() second run error = %v (must be idempotent)", err)
	}

	// Verify core tables exist
	tables := []string{"datasets", "records", "dataset_states", "outbox", "dataset_counts", "dataset_access_log", "schema_migrations"}
	for _, table := range tables {
		var exists bool
		err := db.QueryRow(
			"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)",
			table,
		).Scan(&exists)
		if err != nil {
			t.Fatalf("check table %s: %v", table, err)
		}
		if !exists {
			t.Errorf("table %q was not created by migrations", table)
		}
	}
}

// TestMigrationFiles verifies that all 5 migration SQL files are embedded.
func TestMigrationFiles(t *testing.T) {
	expected := []string{
		"migrations/001_records.sql",
		"migrations/002_outbox.sql",
		"migrations/003_dataset_counts.sql",
		"migrations/004_access_log.sql",
		"migrations/005_soft_delete.sql",
	}

	for _, path := range expected {
		content, err := migrationFiles.ReadFile(path)
		if err != nil {
			t.Errorf("embedded file %s not found: %v", path, err)
			continue
		}
		if len(content) == 0 {
			t.Errorf("embedded file %s is empty", path)
		}
	}
}
