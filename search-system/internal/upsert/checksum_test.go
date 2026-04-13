package upsert

import (
	"strings"
	"testing"
)

// TestGenerateID_Deterministic verifies that identical inputs always produce the same ID.
func TestGenerateID_Deterministic(t *testing.T) {
	id1 := GenerateID("record-001", "crm")
	id2 := GenerateID("record-001", "crm")
	if id1 != id2 {
		t.Errorf("GenerateID not deterministic: %q != %q", id1, id2)
	}
}

// TestGenerateID_DifferentSources verifies that different sources produce different IDs.
func TestGenerateID_DifferentSources(t *testing.T) {
	idA := GenerateID("record-001", "crm")
	idB := GenerateID("record-001", "erp")
	if idA == idB {
		t.Errorf("same ExternalID with different Source produced duplicate ID: %q", idA)
	}
}

// TestGenerateID_DifferentExternalIDs verifies that different external IDs produce different IDs.
func TestGenerateID_DifferentExternalIDs(t *testing.T) {
	idA := GenerateID("rec-001", "src")
	idB := GenerateID("rec-002", "src")
	if idA == idB {
		t.Errorf("different ExternalIDs produced same ID: %q", idA)
	}
}

// TestGenerateID_Format verifies the output is 64 lowercase hex characters (SHA-256).
func TestGenerateID_Format(t *testing.T) {
	id := GenerateID("abc", "xyz")
	if len(id) != 64 {
		t.Errorf("ID length = %d, want 64", len(id))
	}
	if strings.ToLower(id) != id {
		t.Errorf("ID is not lowercase hex: %q", id)
	}
}

// TestGenerateChecksum_Deterministic verifies that identical inputs always produce the same checksum.
func TestGenerateChecksum_Deterministic(t *testing.T) {
	c1 := GenerateChecksum("Widget A", `{"price":100}`)
	c2 := GenerateChecksum("Widget A", `{"price":100}`)
	if c1 != c2 {
		t.Errorf("GenerateChecksum not deterministic: %q != %q", c1, c2)
	}
}

// TestGenerateChecksum_ChangeDetection verifies that a value change produces a different checksum.
func TestGenerateChecksum_ChangeDetection(t *testing.T) {
	c1 := GenerateChecksum("Widget A", `{"price":100}`)
	c2 := GenerateChecksum("Widget A", `{"price":200}`)
	if c1 == c2 {
		t.Errorf("different values produced same checksum: %q", c1)
	}
}

// TestGenerateChecksum_NameChangeDetection verifies that a name change produces a different checksum.
func TestGenerateChecksum_NameChangeDetection(t *testing.T) {
	c1 := GenerateChecksum("Widget A", `{"price":100}`)
	c2 := GenerateChecksum("Widget B", `{"price":100}`)
	if c1 == c2 {
		t.Errorf("different names produced same checksum: %q", c1)
	}
}

// TestGenerateChecksum_Format verifies the output is 64 lowercase hex characters.
func TestGenerateChecksum_Format(t *testing.T) {
	c := GenerateChecksum("Test Product", nil)
	if len(c) != 64 {
		t.Errorf("checksum length = %d, want 64", len(c))
	}
	if strings.ToLower(c) != c {
		t.Errorf("checksum is not lowercase hex: %q", c)
	}
}
