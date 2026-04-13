package upsert

import (
	"fmt"
	"testing"
)

// BenchmarkGenerateID measures the cost of computing a deterministic SHA-256 record ID.
func BenchmarkGenerateID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GenerateID(fmt.Sprintf("external-%d", i), "source-crm")
	}
}

// BenchmarkGenerateChecksum measures the cost of computing a content checksum.
func BenchmarkGenerateChecksum(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GenerateChecksum(fmt.Sprintf("Product Name %d", i), map[string]int{"price": i, "qty": i * 2})
	}
}

// BenchmarkSplitBatches_10k measures how long it takes to split 10 000 records
// into 1 000-record batches (pure slice slicing, no allocations beyond the
// outer slice header).
func BenchmarkSplitBatches_10k(b *testing.B) {
	records := make([]Record, 10_000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		splitBatches(records, 1000)
	}
}

// BenchmarkPrepareRecords_1000 measures the CPU cost of computing IDs and
// checksums for 1 000 records (no DB interaction).
func BenchmarkPrepareRecords_1000(b *testing.B) {
	records := make([]Record, 1000)
	for i := range records {
		records[i] = Record{
			ExternalID: fmt.Sprintf("ext-%d", i),
			Source:     "bench-source",
			Name:       fmt.Sprintf("Product %d", i),
			Value:      fmt.Sprintf(`{"price":%d}`, i),
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Clone slice so prepareRecords can overwrite without contaminating
		// the next iteration.
		clone := make([]Record, len(records))
		copy(clone, records)
		prepareRecords("ds-bench-001", clone)
	}
}
