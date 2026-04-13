package search

import (
	"context"
	"fmt"
	"testing"

	"github.com/blevesearch/bleve/v2"
)

// buildTestBleveIndex creates a fresh in-memory Bleve index seeded with docs.
func buildTestBleveIndex(t *testing.T, docs []indexDoc) bleve.Index {
	t.Helper()
	idx, err := bleve.NewMemOnly(buildIndexMapping())
	if err != nil {
		t.Fatalf("bleve.NewMemOnly: %v", err)
	}
	for i, doc := range docs {
		key := fmt.Sprintf("doc-%03d", i)
		if err := idx.Index(key, doc); err != nil {
			t.Fatalf("Index doc %d: %v", i, err)
		}
	}
	t.Cleanup(func() { idx.Close() }) //nolint:errcheck
	return idx
}

// TestSearchBleveIndex_EmptyTerm verifies that an empty query term returns 0 hits.
func TestSearchBleveIndex_EmptyTerm(t *testing.T) {
	idx := buildTestBleveIndex(t, []indexDoc{
		{Name: "iPhone", Value: `{}`},
	})

	result, err := searchBleveIndex(context.Background(), idx, Query{Term: "", Limit: 10}, "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Hits) != 0 {
		t.Errorf("empty term: want 0 hits, got %d", len(result.Hits))
	}
}

// TestSearchBleveIndex_ExactMatch verifies that an exact term returns a relevant hit.
func TestSearchBleveIndex_ExactMatch(t *testing.T) {
	idx := buildTestBleveIndex(t, []indexDoc{
		{Name: "iPhone 14 Pro", Value: `{"brand":"Apple"}`},
		{Name: "Samsung Galaxy S23", Value: `{"brand":"Samsung"}`},
		{Name: "Google Pixel 8", Value: `{"brand":"Google"}`},
	})

	result, err := searchBleveIndex(context.Background(), idx, Query{Term: "iPhone", Limit: 10}, "bleve_test")
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	if result.Total == 0 {
		t.Fatal("expected >= 1 hit for 'iPhone', got 0")
	}
	found := false
	for _, h := range result.Hits {
		if h.Name == "iPhone 14 Pro" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("'iPhone 14 Pro' not found in hits: %+v", result.Hits)
	}
}

// TestSearchBleveIndex_FuzzyMatch verifies that a 1-edit-distance typo still returns a hit.
func TestSearchBleveIndex_FuzzyMatch(t *testing.T) {
	idx := buildTestBleveIndex(t, []indexDoc{
		{Name: "iPhone", Value: `{}`},
	})

	// "iphne" → delete 'o' from "iphone" = edit distance 1 (FuzzyQuery is case-sensitive
	// against the lowercased index terms produced by the standard analyzer).
	result, err := searchBleveIndex(context.Background(), idx, Query{Term: "iphne", Limit: 10, Fuzziness: 1}, "bleve_test")
	if err != nil {
		t.Fatalf("fuzzy search error: %v", err)
	}
	if result.Total == 0 {
		t.Error("fuzzy search: expected hit for 'iPhne' → 'iPhone' (edit distance 1), got 0")
	}
}

// TestSearchBleveIndex_EngineName verifies the engine label is propagated to the result.
func TestSearchBleveIndex_EngineName(t *testing.T) {
	idx := buildTestBleveIndex(t, nil)

	result, err := searchBleveIndex(context.Background(), idx, Query{Term: ""}, "bleve_memory")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Engine != "bleve_memory" {
		t.Errorf("Engine = %q, want bleve_memory", result.Engine)
	}
}

// TestSearchBleveIndex_LimitRespected verifies that the Limit field caps returned hits.
func TestSearchBleveIndex_LimitRespected(t *testing.T) {
	docs := make([]indexDoc, 20)
	for i := range docs {
		docs[i] = indexDoc{Name: fmt.Sprintf("Widget %d", i), Value: `{}`}
	}
	idx := buildTestBleveIndex(t, docs)

	result, err := searchBleveIndex(context.Background(), idx, Query{Term: "Widget", Limit: 5}, "test")
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	if len(result.Hits) > 5 {
		t.Errorf("got %d hits with Limit=5, want ≤5", len(result.Hits))
	}
}
