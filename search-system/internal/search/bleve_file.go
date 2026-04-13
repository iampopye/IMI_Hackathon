package search

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/blevesearch/bleve/v2"
	"github.com/rs/zerolog"
)

// FileEngine manages per-dataset file-backed Bleve indexes.
//
// Each dataset's index is stored under dataDir/<datasetID>.  The OS pages the
// mmap'd files to disk automatically, keeping RAM usage low regardless of index
// size.  Suitable for datasets up to ~5M records.
type FileEngine struct {
	mu      sync.RWMutex
	indexes map[string]bleve.Index
	dataDir string // root directory, e.g. "./data/bleve"
	db      *sql.DB
	log     zerolog.Logger
}

// NewFileEngine creates a FileEngine that stores indexes under dataDir.
func NewFileEngine(dataDir string, db *sql.DB, log zerolog.Logger) *FileEngine {
	return &FileEngine{
		indexes: make(map[string]bleve.Index),
		dataDir: dataDir,
		db:      db,
		log:     log,
	}
}

// Ensure opens (or creates) the file-backed Bleve index for datasetID.
//
//   - If the on-disk index already exists, it is opened and reused across
//     restarts — no rebuild required.
//   - If it does not exist yet, a new index is created and loaded from PG.
func (e *FileEngine) Ensure(ctx context.Context, datasetID string) (bleve.Index, error) {
	// Fast path.
	e.mu.RLock()
	if idx, ok := e.indexes[datasetID]; ok {
		e.mu.RUnlock()
		return idx, nil
	}
	e.mu.RUnlock()

	// Slow path — open or build under write lock.
	e.mu.Lock()
	defer e.mu.Unlock()

	if idx, ok := e.indexes[datasetID]; ok {
		return idx, nil
	}

	path := filepath.Join(e.dataDir, datasetID)
	if err := os.MkdirAll(e.dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("bleve file mkdir %s: %w", e.dataDir, err)
	}

	// Try opening an existing on-disk index first.
	idx, err := bleve.Open(path)
	if err != nil {
		// Not found or corrupt — create fresh.
		idx, err = bleve.New(path, buildIndexMapping())
		if err != nil {
			return nil, fmt.Errorf("bleve file new index %s: %w", datasetID, err)
		}
		if err := loadRecordsIntoBleveIndex(ctx, e.db, datasetID, idx, e.log); err != nil {
			idx.Close() //nolint:errcheck
			return nil, err
		}
	}

	e.indexes[datasetID] = idx
	return idx, nil
}

// Search runs a fuzzy query on the file-backed index for datasetID.
func (e *FileEngine) Search(ctx context.Context, datasetID string, q Query) (*Result, error) {
	idx, err := e.Ensure(ctx, datasetID)
	if err != nil {
		return nil, err
	}
	return searchBleveIndex(ctx, idx, q, "bleve_file")
}

// IndexRecord updates a single document in the file-backed index.
// Silently skips datasets whose index has not been opened yet.
func (e *FileEngine) IndexRecord(datasetID string, doc indexDoc) {
	e.mu.RLock()
	idx, ok := e.indexes[datasetID]
	e.mu.RUnlock()
	if !ok {
		return
	}
	idx.Index(doc.Name, doc) //nolint:errcheck
}

// Close flushes and closes all open file indexes.  Call during server shutdown.
func (e *FileEngine) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()
	for id, idx := range e.indexes {
		if err := idx.Close(); err != nil {
			e.log.Error().Err(err).Str("dataset", id).Msg("bleve file close")
		}
	}
}
