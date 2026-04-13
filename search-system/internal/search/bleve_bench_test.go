package search

import (
	"context"
	"fmt"
	"testing"

	"github.com/blevesearch/bleve/v2"
)

// BenchmarkBleveSearch_100docs measures search latency on a 100-document
// in-memory Bleve index — representative of the small/medium tier hot path.
func BenchmarkBleveSearch_100docs(b *testing.B) {
	idx, err := bleve.NewMemOnly(buildIndexMapping())
	if err != nil {
		b.Fatalf("bleve.NewMemOnly: %v", err)
	}
	defer idx.Close() //nolint:errcheck

	for i := 0; i < 100; i++ {
		_ = idx.Index(fmt.Sprintf("id-%d", i), indexDoc{
			Name:  fmt.Sprintf("Product %d Widget", i),
			Value: fmt.Sprintf(`{"price":%d,"category":"electronics"}`, i*10),
		})
	}

	q := Query{Term: "Widget", Limit: 10}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = searchBleveIndex(context.Background(), idx, q, "bench")
	}
}

// BenchmarkBleveSearch_1000docs measures search latency on a 1 000-document
// index — still within the in-memory tier but with more competition.
func BenchmarkBleveSearch_1000docs(b *testing.B) {
	idx, err := bleve.NewMemOnly(buildIndexMapping())
	if err != nil {
		b.Fatalf("bleve.NewMemOnly: %v", err)
	}
	defer idx.Close() //nolint:errcheck

	for i := 0; i < 1000; i++ {
		_ = idx.Index(fmt.Sprintf("id-%d", i), indexDoc{
			Name:  fmt.Sprintf("Item %d", i),
			Value: fmt.Sprintf(`{"sku":"SKU-%05d"}`, i),
		})
	}

	q := Query{Term: "Item", Limit: 20}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = searchBleveIndex(context.Background(), idx, q, "bench")
	}
}

// BenchmarkBleveSearch_FuzzyQuery measures the overhead of fuzzy matching
// (fuzziness=2) versus exact matching on the same 100-document index.
func BenchmarkBleveSearch_FuzzyQuery(b *testing.B) {
	idx, err := bleve.NewMemOnly(buildIndexMapping())
	if err != nil {
		b.Fatalf("bleve.NewMemOnly: %v", err)
	}
	defer idx.Close() //nolint:errcheck

	for i := 0; i < 100; i++ {
		_ = idx.Index(fmt.Sprintf("id-%d", i), indexDoc{
			Name:  fmt.Sprintf("Product %d", i),
			Value: `{}`,
		})
	}

	q := Query{Term: "Prodct", Limit: 10, Fuzziness: 2} // intentional typo
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = searchBleveIndex(context.Background(), idx, q, "bench")
	}
}
