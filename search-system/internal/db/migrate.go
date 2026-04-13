package db

import (
	"database/sql"
	"embed"
	"fmt"
	"sort"
	"strings"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Migrate applies all pending SQL migrations in ascending order.
// Applied versions are tracked in the schema_migrations table so
// each migration runs exactly once.
func Migrate(db *sql.DB) error {
	if err := ensureMigrationsTable(db); err != nil {
		return err
	}

	applied, err := appliedMigrations(db)
	if err != nil {
		return err
	}

	pending, err := pendingMigrations(applied)
	if err != nil {
		return err
	}

	for _, file := range pending {
		if err := applyMigration(db, file); err != nil {
			return err
		}
	}

	return nil
}

func ensureMigrationsTable(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMP   DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	return nil
}

func appliedMigrations(db *sql.DB) (map[string]bool, error) {
	rows, err := db.Query("SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		return nil, fmt.Errorf("query schema_migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		applied[v] = true
	}
	return applied, rows.Err()
}

func pendingMigrations(applied map[string]bool) ([]string, error) {
	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}

	var files []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	var pending []string
	for _, f := range files {
		version := strings.TrimSuffix(f, ".sql")
		if !applied[version] {
			pending = append(pending, f)
		}
	}
	return pending, nil
}

func applyMigration(db *sql.DB, filename string) error {
	version := strings.TrimSuffix(filename, ".sql")

	content, err := migrationFiles.ReadFile("migrations/" + filename)
	if err != nil {
		return fmt.Errorf("read %s: %w", filename, err)
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx for %s: %w", filename, err)
	}

	if _, err := tx.Exec(string(content)); err != nil {
		tx.Rollback()
		return fmt.Errorf("execute %s: %w", filename, err)
	}

	if _, err := tx.Exec(
		"INSERT INTO schema_migrations (version) VALUES ($1)", version,
	); err != nil {
		tx.Rollback()
		return fmt.Errorf("record %s in schema_migrations: %w", filename, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit %s: %w", filename, err)
	}

	fmt.Printf("[migrate] applied %s\n", filename)
	return nil
}
