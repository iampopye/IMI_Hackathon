package upsert

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// GenerateID produces the same deterministic ID for the same natural key every time.
// SHA-256 of "externalID:source" guarantees global uniqueness without a central sequence.
func GenerateID(externalID, source string) string {
	hash := sha256.Sum256([]byte(externalID + ":" + source))
	return hex.EncodeToString(hash[:])
}

// GenerateChecksum hashes the content fields of a record.
// If the checksum matches what is already in the DB, the content has not changed —
// the PostgreSQL upsert WHERE guard will skip the write entirely.
func GenerateChecksum(name string, value interface{}) string {
	content := fmt.Sprintf("%v:%v", name, value)
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}
