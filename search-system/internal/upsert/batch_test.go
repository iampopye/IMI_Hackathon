package upsert

import (
	"testing"
)

// TestSplitBatches_Even verifies equal-sized batches when records divide evenly.
func TestSplitBatches_Even(t *testing.T) {
	records := make([]Record, 10)
	batches := splitBatches(records, 5)
	if len(batches) != 2 {
		t.Fatalf("want 2 batches, got %d", len(batches))
	}
	if len(batches[0]) != 5 || len(batches[1]) != 5 {
		t.Errorf("unequal batch sizes: %d, %d", len(batches[0]), len(batches[1]))
	}
}

// TestSplitBatches_Uneven verifies the last batch carries the remainder.
func TestSplitBatches_Uneven(t *testing.T) {
	records := make([]Record, 11)
	batches := splitBatches(records, 5)
	if len(batches) != 3 {
		t.Fatalf("want 3 batches, got %d", len(batches))
	}
	if len(batches[2]) != 1 {
		t.Errorf("last batch = %d records, want 1", len(batches[2]))
	}
}

// TestSplitBatches_SmallerThanBatch verifies all records land in one batch when
// the total count is below batchSize.
func TestSplitBatches_SmallerThanBatch(t *testing.T) {
	records := make([]Record, 3)
	batches := splitBatches(records, 10)
	if len(batches) != 1 {
		t.Fatalf("want 1 batch, got %d", len(batches))
	}
	if len(batches[0]) != 3 {
		t.Errorf("batch size = %d, want 3", len(batches[0]))
	}
}

// TestSplitBatches_Empty verifies that an empty input produces no batches.
func TestSplitBatches_Empty(t *testing.T) {
	batches := splitBatches(nil, 10)
	if len(batches) != 0 {
		t.Errorf("want 0 batches for empty input, got %d", len(batches))
	}
}

// TestSplitBatches_ZeroSizeDefaults verifies that batchSize=0 falls back to 1000.
func TestSplitBatches_ZeroSizeDefaults(t *testing.T) {
	records := make([]Record, 5)
	batches := splitBatches(records, 0) // 0 → default 1000
	if len(batches) != 1 {
		t.Errorf("want 1 batch (5 records fit in default 1000), got %d", len(batches))
	}
}

// TestPrepareRecords_SetsIDAndChecksum verifies that prepareRecords fills in
// deterministic IDs, checksums, and DatasetID for every record.
func TestPrepareRecords_SetsIDAndChecksum(t *testing.T) {
	records := []Record{
		{ExternalID: "ext-1", Source: "crm", Name: "Widget", Value: "v1"},
		{ExternalID: "ext-2", Source: "crm", Name: "Gadget", Value: "v2"},
	}
	prepareRecords("ds-123", records)

	for i, r := range records {
		if r.DatasetID != "ds-123" {
			t.Errorf("[%d] DatasetID = %q, want ds-123", i, r.DatasetID)
		}
		if r.ID == "" {
			t.Errorf("[%d] ID is empty after prepareRecords", i)
		}
		if r.Checksum == "" {
			t.Errorf("[%d] Checksum is empty after prepareRecords", i)
		}
	}

	// Different external IDs → different IDs.
	if records[0].ID == records[1].ID {
		t.Errorf("records with different ExternalID produced same ID: %q", records[0].ID)
	}

	// Verify ID matches what GenerateID would produce independently.
	expected := GenerateID("ext-1", "crm")
	if records[0].ID != expected {
		t.Errorf("records[0].ID = %q, want %q", records[0].ID, expected)
	}
}
