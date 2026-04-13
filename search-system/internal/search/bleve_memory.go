package search

import (
	"context"
	"database/sql"
	"fmt"
	"sync"

	"github.com/blevesearch/bleve/v2"
	"github.com/rs/zerolog"
)

// MemoryEngine manages per-dataset Bleve in-memory indexes.
//
// Each dataset gets its own isolated index. The index is built lazily on first
// access (or eagerly by warmup). Safe for datasets up to ~100K records (~50MB
// RAM); above that the search router directs traffic to FileEngine.
type MemoryEngine struct {
	mu      sync.RWMutex
	indexes map[string]bleve.Index
	db      *sql.DB
	log     zerolog.Logger
}

// NewMemoryEngine creates a MemoryEngine backed by the given database connection.
func NewMemoryEngine(db *sql.DB, log zerolog.Logger) *MemoryEngine {
	return &MemoryEngine{
		indexes: make(map[string]bleve.Index),
		db:      db,
		log:     log,
	}
}

// Ensure returns the in-memory Bleve index for datasetID, building it from
// PostgreSQL if it does not exist yet.  Thread-safe.
func (e *MemoryEngine) Ensure(ctx context.Context, datasetID string) (bleve.Index, error) {
	// Fast path — already built.
	e.mu.RLock()
	if idx, ok := e.indexes[datasetID]; ok {
		e.mu.RUnlock()
		return idx, nil
	}
	e.mu.RUnlock()

	// Slow path — build under write lock.
	e.mu.Lock()
	defer e.mu.Unlock()

	// Re-check: another goroutine may have built it while we waited.
	if idx, ok := e.indexes[datasetID]; ok {
		return idx, nil
	}

	idx, err := bleve.NewMemOnly(buildIndexMapping())
	if err != nil {
		return nil, fmt.Errorf("bleve memory new index %s: %w", datasetID, err)
	}

	if err := loadRecordsIntoBleveIndex(ctx, e.db, datasetID, idx, e.log); err != nil {
		idx.Close() //nolint:errcheck
		return nil, err
	}

	e.indexes[datasetID] = idx
	return idx, nil
}

// Search runs a fuzzy query on the in-memory index for datasetID.
// The index is built on first call if it does not exist yet.
func (e *MemoryEngine) Search(ctx context.Context, datasetID string, q Query) (*Result, error) {
	idx, err := e.Ensure(ctx, datasetID)
	if err != nil {
		return nil, err
	}
	return searchBleveIndex(ctx, idx, q, "bleve_memory")
}

// IndexRecord updates a single document in the in-memory index.
// Silently skips datasets whose index has not been built yet — the next
// search will build a fresh index that includes the new record.
func (e *MemoryEngine) IndexRecord(datasetID string, doc indexDoc) {
	e.mu.RLock()
	idx, ok := e.indexes[datasetID]
	e.mu.RUnlock()
	if !ok {
		return
	}
	idx.Index(doc.Name, doc) //nolint:errcheck
}

// Invalidate drops the in-memory index for a dataset.  The next search will
// trigger a full rebuild from PostgreSQL.  Use after a tier upgrade.
func (e *MemoryEngine) Invalidate(datasetID string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if idx, ok := e.indexes[datasetID]; ok {
		idx.Close() //nolint:errcheck
		delete(e.indexes, datasetID)
	}
}
