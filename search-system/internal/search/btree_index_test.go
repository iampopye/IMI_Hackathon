package search

import (
	"fmt"
	"testing"
)

// TestBTreeInsertAndSearch verifies basic insert → search round-trip.
func TestBTreeInsertAndSearch(t *testing.T) {
	idx := NewBTreeIndex()

	idx.Insert("key-001", "id-1", "Alpha Widget", []byte(`{"price":10}`))
	idx.Insert("key-002", "id-2", "Beta Widget", []byte(`{"price":20}`))
	idx.Insert("key-003", "id-3", "Gamma Widget", []byte(`{"price":30}`))

	if idx.Len() != 3 {
		t.Fatalf("Len() = %d, want 3", idx.Len())
	}

	item := idx.Search("key-002")
	if item == nil {
		t.Fatal("Search(key-002) returned nil")
	}
	if item.Name != "Beta Widget" {
		t.Errorf("Search(key-002).Name = %q, want Beta Widget", item.Name)
	}
}

// TestBTreeSearchMiss verifies that searching for a non-existent key returns nil.
func TestBTreeSearchMiss(t *testing.T) {
	idx := NewBTreeIndex()
	idx.Insert("key-001", "id-1", "Alpha", nil)

	if got := idx.Search("key-999"); got != nil {
		t.Errorf("Search(key-999) = %+v, want nil", got)
	}
}

// TestBTreeDelete verifies that a deleted record is no longer findable.
func TestBTreeDelete(t *testing.T) {
	idx := NewBTreeIndex()
	idx.Insert("key-001", "id-1", "Alpha", nil)
	idx.Insert("key-002", "id-2", "Beta", nil)

	idx.Delete("key-001")

	if idx.Len() != 1 {
		t.Errorf("Len() after delete = %d, want 1", idx.Len())
	}
	if got := idx.Search("key-001"); got != nil {
		t.Error("deleted record still findable")
	}
}

// TestBTreeReplaceOrInsert verifies that re-inserting the same key updates the record.
func TestBTreeReplaceOrInsert(t *testing.T) {
	idx := NewBTreeIndex()
	idx.Insert("key-001", "id-1", "Old Name", nil)
	idx.Insert("key-001", "id-1", "New Name", nil) // overwrite

	if idx.Len() != 1 {
		t.Errorf("Len() after overwrite = %d, want 1", idx.Len())
	}
	item := idx.Search("key-001")
	if item == nil || item.Name != "New Name" {
		t.Errorf("after overwrite: Name = %q, want New Name", item.Name)
	}
}

// TestBTreeAscendOrder verifies that AscendAll returns items in sorted key order.
func TestBTreeAscendOrder(t *testing.T) {
	idx := NewBTreeIndex()

	// Insert in reverse order to prove the tree sorts them.
	for i := 9; i >= 0; i-- {
		key := fmt.Sprintf("key-%03d", i)
		idx.Insert(key, key, fmt.Sprintf("Item %d", i), nil)
	}

	var keys []string
	idx.AscendAll(func(item RecordItem) bool {
		keys = append(keys, item.Key)
		return true
	})

	if len(keys) != 10 {
		t.Fatalf("AscendAll returned %d items, want 10", len(keys))
	}
	for i, k := range keys {
		want := fmt.Sprintf("key-%03d", i)
		if k != want {
			t.Errorf("keys[%d] = %q, want %q", i, k, want)
		}
	}
}

// TestBTreeAscendFrom verifies range iteration starting from a given key.
func TestBTreeAscendFrom(t *testing.T) {
	idx := NewBTreeIndex()
	for i := 0; i < 10; i++ {
		idx.Insert(fmt.Sprintf("key-%03d", i), "", "", nil)
	}

	var keys []string
	idx.AscendFrom("key-005", func(item RecordItem) bool {
		keys = append(keys, item.Key)
		return true
	})

	if len(keys) != 5 {
		t.Fatalf("AscendFrom(key-005) returned %d items, want 5", len(keys))
	}
	if keys[0] != "key-005" {
		t.Errorf("first key = %q, want key-005", keys[0])
	}
}

// TestBTreeConcurrentInsert verifies that concurrent inserts do not race.
// Run with -race to detect data races.
func TestBTreeConcurrentInsert(t *testing.T) {
	idx := NewBTreeIndex()

	done := make(chan struct{})
	for i := 0; i < 50; i++ {
		go func(n int) {
			idx.Insert(fmt.Sprintf("key-%04d", n), "", "", nil)
			done <- struct{}{}
		}(i)
	}
	for i := 0; i < 50; i++ {
		<-done
	}

	if idx.Len() != 50 {
		t.Errorf("Len() after concurrent inserts = %d, want 50", idx.Len())
	}
}
