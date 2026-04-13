package search

import (
	"sync"

	"github.com/google/btree"
)

// RecordItem wraps a record for storage in the BTreeIndex.
// The Key field drives ordering (typically the record ID or a sort field).
type RecordItem struct {
	Key   string
	ID    string
	Name  string
	Value []byte
}

// Less satisfies btree.Item.  Items are ordered lexicographically by Key.
func (r RecordItem) Less(other btree.Item) bool {
	return r.Key < other.(RecordItem).Key
}

// BTreeIndex is a concurrency-safe, always-sorted in-memory index backed by a
// B-Tree (degree 32).  Every insert maintains sorted order at O(log n) — there
// is never a need to sort the entire dataset, eliminating the CPU spike from v1.
//
// Why B-Tree eliminates the Thundering Herd (v1 vs v2 comparison):
//
//	v1: data arrives unsorted → dataset stabilises → sort ALL records → CPU spike
//	    100 datasets stable simultaneously → 100 spikes → API freezes
//
//	v2: data arrives → inserted into B-Tree → O(log n) cost per insert
//	    dataset stabilises → tree already sorted → zero extra work
type BTreeIndex struct {
	tree *btree.BTree
	mu   sync.RWMutex
}

// NewBTreeIndex creates a BTreeIndex with btree degree 32 (optimal for most
// record sizes — balances node fan-out against memory allocation overhead).
func NewBTreeIndex() *BTreeIndex {
	return &BTreeIndex{tree: btree.New(32)}
}

// Insert adds or replaces a record.  O(log n).
func (b *BTreeIndex) Insert(key, id, name string, value []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.tree.ReplaceOrInsert(RecordItem{Key: key, ID: id, Name: name, Value: value})
}

// Delete removes a record by key.  O(log n).
func (b *BTreeIndex) Delete(key string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.tree.Delete(RecordItem{Key: key})
}

// Search finds a record by exact key.  Returns nil if not found.  O(log n).
func (b *BTreeIndex) Search(key string) *RecordItem {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var found *RecordItem
	b.tree.AscendGreaterOrEqual(RecordItem{Key: key}, func(i btree.Item) bool {
		item := i.(RecordItem)
		if item.Key == key {
			found = &item
		}
		return false // stop after first candidate
	})
	return found
}

// Len returns the number of records in the index.
func (b *BTreeIndex) Len() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.tree.Len()
}

// AscendAll calls fn for each item in ascending key order.
// fn returning false stops the iteration early.
func (b *BTreeIndex) AscendAll(fn func(RecordItem) bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	b.tree.Ascend(func(i btree.Item) bool {
		return fn(i.(RecordItem))
	})
}

// AscendFrom calls fn for each item with Key >= start, in ascending order.
func (b *BTreeIndex) AscendFrom(start string, fn func(RecordItem) bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	b.tree.AscendGreaterOrEqual(RecordItem{Key: start}, func(i btree.Item) bool {
		return fn(i.(RecordItem))
	})
}
