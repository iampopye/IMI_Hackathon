package dataset

// Default stability constants — overridden by config at runtime.
const (
	StabilityThreshold = 0.70 // score >= 0.70 → STABLE
	StabilityTick      = 0.05 // score increase per monitor cycle (no changes observed)
	StabilityDecay     = 0.80 // multiply score by this factor on each change event

	// Tier thresholds (record counts).
	InMemoryLimit  = 100_000   // < 100K  → TierSmall  (Bleve in-memory)
	BleveFileLimit = 5_000_000 // < 5M    → TierMedium (Bleve file-backed); ≥ 5M → TierLarge (ES)
)

// SearchTier represents which search engine to use for a dataset.
type SearchTier int

const (
	TierSmall  SearchTier = iota // Bleve in-memory  (< 100K records)
	TierMedium                   // Bleve file-backed (100K–5M records)
	TierLarge                    // Elasticsearch    (5M+ records)
)

// String returns the lowercase string form stored in dataset_states.current_tier.
func (t SearchTier) String() string {
	switch t {
	case TierMedium:
		return "medium"
	case TierLarge:
		return "large"
	default:
		return "small"
	}
}

// tierFromString maps the database string back to SearchTier.
func tierFromString(s string) SearchTier {
	switch s {
	case "medium":
		return TierMedium
	case "large":
		return TierLarge
	default:
		return TierSmall
	}
}
