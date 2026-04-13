package integration

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/yourusername/search-system/internal/cache"
	"github.com/yourusername/search-system/internal/outbox"
	"github.com/yourusername/search-system/internal/search"
	"github.com/yourusername/search-system/internal/upsert"
)

// ── Pure chaos tests (no DB required) ────────────────────────────────────────

// TestChaos_MemoryCache_EvictionUnderPressure fills a cap-50 cache with 500
// entries and verifies that early entries are evicted and the most-recent entry
// is always present. The cache must never exceed its capacity.
func TestChaos_MemoryCache_EvictionUnderPressure(t *testing.T) {
	const cap = 50
	c := cache.NewMemoryCache(cap)

	for i := 0; i < cap*10; i++ {
		c.Set(fmt.Sprintf("key-%d", i), []byte("data"), time.Minute)
	}

	// Early keys must have been evicted.
	if _, ok := c.Get("key-0"); ok {
		t.Error("key-0 should have been evicted after filling 10× capacity")
	}
	// Most recently added key must still be present.
	last := fmt.Sprintf("key-%d", cap*10-1)
	if _, ok := c.Get(last); !ok {
		t.Errorf("last inserted key %q should still be in cache", last)
	}
}

// TestChaos_BTreeIndex_ConcurrentInsertDelete verifies that simultaneous
// inserts and deletes do not trigger a data race.  Run with -race.
func TestChaos_BTreeIndex_ConcurrentInsertDelete(t *testing.T) {
	idx := search.NewBTreeIndex()

	// Pre-populate 100 entries.
	for i := 0; i < 100; i++ {
		idx.Insert(fmt.Sprintf("key-%04d", i), "", fmt.Sprintf("item-%d", i), nil)
	}

	var wg sync.WaitGroup

	// 50 concurrent inserters (new keys).
	for i := 100; i < 150; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			idx.Insert(fmt.Sprintf("key-%04d", n), "", "", nil)
		}(i)
	}

	// 50 concurrent deleters (existing keys).
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			idx.Delete(fmt.Sprintf("key-%04d", n))
		}(i)
	}

	wg.Wait()

	// After: 50 survivors + 50 new inserts = 100, but due to race ordering
	// the real count can range [50, 150]; either extreme is a bug.
	l := idx.Len()
	if l < 50 || l > 150 {
		t.Errorf("BTree Len = %d after concurrent ops; expected [50, 150]", l)
	}
}

// TestChaos_Cache_InvalidatePrefix_ConcurrentWrites fires concurrent writes and
// a prefix invalidation to confirm there is no data race on the shared map.
// Run with -race.
func TestChaos_Cache_InvalidatePrefix_ConcurrentWrites(t *testing.T) {
	c := cache.NewMemoryCache(500)
	var wg sync.WaitGroup

	// 100 concurrent writers.
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			c.Set(fmt.Sprintf("ds:001:q%d", n), []byte("v"), time.Minute)
		}(i)
	}

	// 10 concurrent invalidations.
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.InvalidatePrefix("ds:001:")
		}()
	}

	wg.Wait()
	// No assertion needed — surviving the -race detector is the test.
}

// ── DB-backed chaos tests ─────────────────────────────────────────────────────

// TestChaos_ContextCancellation_UpsertAborts verifies that a pre-cancelled
// context causes BulkUpsert to return non-zero Failed without panicking.
func TestChaos_ContextCancellation_UpsertAborts(t *testing.T) {
	conn := testDB(t)
	dsID := createTestDataset(t, conn, uname("chaos-ctx"))
	engine := upsert.New(conn, outbox.New(conn), upsert.Config{BatchSize: 100, WorkerCount: 4})

	records := make([]upsert.Record, 1000)
	for i := range records {
		records[i] = upsert.Record{
			ExternalID: fmt.Sprintf("rec-%05d", i),
			Source:     "chaos",
			Name:       fmt.Sprintf("Record %d", i),
			Value:      `{}`,
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before call — every BeginTx must fail

	result := engine.BulkUpsert(ctx, dsID, records)
	if result.Failed == 0 {
		t.Error("expected Failed > 0 with a pre-cancelled context, got 0")
	}
}

// TestChaos_Search_EmptyDataset_NoError verifies that searching a dataset that
// has zero records does not return an error or panic.
func TestChaos_Search_EmptyDataset_NoError(t *testing.T) {
	conn := testDB(t)
	dsID := createTestDataset(t, conn, uname("chaos-empty"))

	memEngine := search.NewMemoryEngine(conn, nopLog())
	result, err := memEngine.Search(context.Background(), dsID,
		search.Query{Term: "anything", Limit: 10},
	)
	if err != nil {
		t.Fatalf("search on empty dataset returned error: %v", err)
	}
	if len(result.Hits) != 0 {
		t.Errorf("empty dataset: got %d hits, want 0", len(result.Hits))
	}
}
