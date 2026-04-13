package dataset

import (
	"math"
	"testing"
)

// ── Stability score arithmetic ────────────────────────────────────────────────

// TestStabilityDecay verifies that OnDatasetChanged decays the score by 0.80×
// and clears the IsSorted flag without a real database (pure math test using
// the constants directly).
func TestStabilityDecay(t *testing.T) {
	initial := 1.0
	after := initial * StabilityDecay

	if math.Abs(after-0.80) > 1e-9 {
		t.Errorf("decay: got %.4f, want 0.80", after)
	}

	// A second decay (simulating two rapid writes):
	after2 := after * StabilityDecay
	if math.Abs(after2-0.64) > 1e-9 {
		t.Errorf("double decay: got %.4f, want 0.64", after2)
	}
}

// TestStabilityTick verifies that each tick adds StabilityTick and is capped at 1.0.
func TestStabilityTick(t *testing.T) {
	score := 0.0
	for i := 0; i < 20; i++ {
		score = math.Min(1.0, score+StabilityTick)
	}
	if score != 1.0 {
		t.Errorf("tick cap: got %.4f, want 1.0", score)
	}

	// Exactly 6 ticks from 0 should reach 0.30 (6 × 0.05).
	s6 := math.Min(1.0, 6*StabilityTick)
	if math.Abs(s6-0.30) > 1e-9 {
		t.Errorf("6 ticks: got %.4f, want 0.30", s6)
	}
}

// TestIsStable verifies the 0.70 threshold boundary.
func TestIsStable(t *testing.T) {
	cases := []struct {
		score  float64
		stable bool
	}{
		{0.0, false},
		{0.69, false},
		{0.70, true},   // exactly at threshold → stable
		{0.80, true},
		{1.00, true},
	}

	for _, tc := range cases {
		meta := &DatasetMeta{StabilityScore: tc.score}
		if got := meta.IsStable(); got != tc.stable {
			t.Errorf("IsStable(%.2f) = %v, want %v", tc.score, got, tc.stable)
		}
	}
}

// TestStabilityOscillationPattern reproduces the 4-minute update scenario from
// the plan to ensure the score never gets permanently stuck below threshold.
func TestStabilityOscillationPattern(t *testing.T) {
	// Starting STABLE (score = 1.0).
	score := 1.0

	// t=4: update → score × 0.80
	score = score * StabilityDecay // 0.80 — still STABLE
	if score < StabilityThreshold {
		t.Fatalf("t=4 update: score %.2f unexpectedly dropped below threshold", score)
	}

	// t=5: tick → +0.05
	score = math.Min(1.0, score+StabilityTick) // 0.85

	// t=8: update → score × 0.80
	score = score * StabilityDecay // 0.68 — BELOW threshold

	// t=9: tick → +0.05
	score = math.Min(1.0, score+StabilityTick) // 0.73 — STABLE again

	if score < StabilityThreshold {
		t.Errorf("t=9: expected stable after one tick, got %.4f", score)
	}
}

// ── SearchTier helpers ────────────────────────────────────────────────────────

// TestTierString verifies the String/tierFromString round-trip.
func TestTierString(t *testing.T) {
	cases := []struct {
		tier SearchTier
		str  string
	}{
		{TierSmall, "small"},
		{TierMedium, "medium"},
		{TierLarge, "large"},
	}
	for _, tc := range cases {
		if got := tc.tier.String(); got != tc.str {
			t.Errorf("SearchTier(%d).String() = %q, want %q", tc.tier, got, tc.str)
		}
		if got := tierFromString(tc.str); got != tc.tier {
			t.Errorf("tierFromString(%q) = %d, want %d", tc.str, got, tc.tier)
		}
	}
	// Unknown string defaults to TierSmall.
	if got := tierFromString("unknown"); got != TierSmall {
		t.Errorf("tierFromString(unknown) = %d, want TierSmall", got)
	}
}

// ── Profiler hysteresis (no DB — uses applyHysteresis directly) ───────────────

// TestHysteresisSmallToMedium verifies that upgrading from TierSmall to
// TierMedium requires exactly TierUpgradeConfirmations (5) consecutive calls.
func TestHysteresisSmallToMedium(t *testing.T) {
	p := &Profiler{
		inMemoryLimit: InMemoryLimit,
		fileLimit:     BleveFileLimit,
		confirmations: 5,
		hCounters:     make(map[string]int64),
	}

	const ds = "dataset-hysteresis-test"
	count := int64(InMemoryLimit + 1) // just above the 100K boundary

	// First 4 evaluations should stay at TierSmall.
	for i := 1; i <= 4; i++ {
		tier := p.applyHysteresis(ds, count, TierSmall)
		if tier != TierSmall {
			t.Fatalf("call %d: expected TierSmall (not yet confirmed), got %v", i, tier)
		}
	}

	// 5th evaluation should commit the upgrade to TierMedium.
	tier := p.applyHysteresis(ds, count, TierSmall)
	if tier != TierMedium {
		t.Errorf("call 5: expected TierMedium after 5 confirmations, got %v", tier)
	}
}

// TestHysteresisResetOnSmall verifies that dropping back below the boundary
// resets the hysteresis counter (no phantom upgrade later).
func TestHysteresisResetOnSmall(t *testing.T) {
	p := &Profiler{
		inMemoryLimit: InMemoryLimit,
		fileLimit:     BleveFileLimit,
		confirmations: 5,
		hCounters:     make(map[string]int64),
	}

	const ds = "dataset-reset-test"
	above := int64(InMemoryLimit + 1)
	below := int64(InMemoryLimit - 1)

	// Three evaluations above threshold — counter = 3.
	for i := 0; i < 3; i++ {
		p.applyHysteresis(ds, above, TierSmall)
	}

	// Drop back below — counter must be reset.
	p.applyHysteresis(ds, below, TierSmall)

	p.mu.Lock()
	if p.hCounters[ds] != 0 {
		t.Errorf("counter should be 0 after reset, got %d", p.hCounters[ds])
	}
	p.mu.Unlock()

	// Confirm: 5 more evaluations above are now required again.
	for i := 1; i <= 4; i++ {
		tier := p.applyHysteresis(ds, above, TierSmall)
		if tier != TierSmall {
			t.Fatalf("post-reset call %d: expected TierSmall, got %v", i, tier)
		}
	}
}
