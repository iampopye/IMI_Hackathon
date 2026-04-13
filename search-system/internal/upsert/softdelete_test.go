package upsert

import (
	"strings"
	"testing"
)

// TestGenerateSyncToken verifies that generateSyncToken returns unique, non-empty tokens.
func TestGenerateSyncToken(t *testing.T) {
	t1 := generateSyncToken()
	t2 := generateSyncToken()

	if t1 == "" {
		t.Error("generateSyncToken() returned empty string")
	}
	if t1 == t2 {
		t.Errorf("generateSyncToken() returned duplicate tokens: %q", t1)
	}
	// 16 random bytes → 32 hex chars
	if len(t1) != 32 {
		t.Errorf("generateSyncToken() len = %d, want 32", len(t1))
	}
	// Must be lowercase hex
	if strings.ToLower(t1) != t1 {
		t.Errorf("generateSyncToken() not lowercase hex: %q", t1)
	}
}

// TestGenerateSyncToken_Uniqueness generates 1000 tokens and asserts no collisions.
func TestGenerateSyncToken_Uniqueness(t *testing.T) {
	seen := make(map[string]struct{}, 1000)
	for i := 0; i < 1000; i++ {
		tok := generateSyncToken()
		if _, exists := seen[tok]; exists {
			t.Fatalf("collision at iteration %d: %q", i, tok)
		}
		seen[tok] = struct{}{}
	}
}

// TestSyncResult_Fields ensures SyncResult carries all required fields.
func TestSyncResult_Fields(t *testing.T) {
	sr := SyncResult{
		DatasetID:  "ds-001",
		SyncToken:  "abc123",
		Upserted:   UpsertResult{Total: 10, Inserted: 8, Updated: 2},
		Deleted:    3,
		DurationMs: 42,
	}

	if sr.DatasetID != "ds-001" {
		t.Errorf("DatasetID = %q, want ds-001", sr.DatasetID)
	}
	if sr.Deleted != 3 {
		t.Errorf("Deleted = %d, want 3", sr.Deleted)
	}
	if sr.Upserted.Total != 10 {
		t.Errorf("Upserted.Total = %d, want 10", sr.Upserted.Total)
	}
}
