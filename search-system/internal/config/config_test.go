package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_ValidConfig(t *testing.T) {
	// Write a minimal valid config to a temp file
	yaml := `
app:
  port: 8080
  env: test
  warmup_datasets: 10
postgres:
  host: localhost
  port: 5432
  user: testuser
  password: testpass
  dbname: testdb
  max_connections: 5
  max_idle: 2
search:
  in_memory_limit: 100000
  bleve_file_limit: 5000000
  stability_threshold: 0.70
  stability_tick: 0.05
  stability_decay: 0.80
  batch_size: 1000
  worker_count: 10
`
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.App.Port != 8080 {
		t.Errorf("App.Port = %d, want 8080", cfg.App.Port)
	}
	if cfg.Postgres.Host != "localhost" {
		t.Errorf("Postgres.Host = %s, want localhost", cfg.Postgres.Host)
	}
	if cfg.Search.InMemoryLimit != 100000 {
		t.Errorf("Search.InMemoryLimit = %d, want 100000", cfg.Search.InMemoryLimit)
	}
	if cfg.Search.StabilityThreshold != 0.70 {
		t.Errorf("Search.StabilityThreshold = %v, want 0.70", cfg.Search.StabilityThreshold)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("Load() expected error for missing file, got nil")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(path, []byte(":::invalid yaml:::"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Error("Load() expected error for invalid YAML, got nil")
	}
}

func TestLoad_MissingRequiredFields(t *testing.T) {
	// No app.port
	yaml := `
postgres:
  host: localhost
  dbname: testdb
`
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Error("Load() expected validation error for missing app.port, got nil")
	}
}

func TestPostgresConfig_DSN(t *testing.T) {
	cfg := PostgresConfig{
		Host:     "db",
		Port:     5432,
		User:     "u",
		Password: "p",
		DBName:   "mydb",
	}
	want := "host=db port=5432 user=u password=p dbname=mydb sslmode=disable"
	if got := cfg.DSN(); got != want {
		t.Errorf("DSN() = %q, want %q", got, want)
	}
}
