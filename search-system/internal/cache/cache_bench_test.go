package cache

import (
	"fmt"
	"testing"
	"time"
)

// BenchmarkMemoryCache_Get_Hit measures the Get latency when the key is present
// (hot path — no TTL expiry, entry at front of LRU list).
func BenchmarkMemoryCache_Get_Hit(b *testing.B) {
	c := NewMemoryCache(10_000)
	for i := 0; i < 10_000; i++ {
		c.Set(fmt.Sprintf("k%d", i), []byte(`{"result":"cached"}`), time.Hour)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Get(fmt.Sprintf("k%d", i%10_000))
	}
}

// BenchmarkMemoryCache_Get_Miss measures the Get latency for a key that does
// not exist (miss path — single map lookup + return false).
func BenchmarkMemoryCache_Get_Miss(b *testing.B) {
	c := NewMemoryCache(1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Get(fmt.Sprintf("miss-%d", i))
	}
}

// BenchmarkMemoryCache_Set measures the Set latency including LRU bookkeeping
// when the cache is operating below capacity (no eviction).
func BenchmarkMemoryCache_Set(b *testing.B) {
	c := NewMemoryCache(b.N + 1) // large enough to never evict
	payload := []byte(`{"hits":[{"id":"abc","name":"Widget","score":0.99}]}`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Set(fmt.Sprintf("k%d", i), payload, time.Minute)
	}
}

// BenchmarkMemoryCache_Set_Eviction measures the Set latency under continuous
// LRU eviction pressure (cache is always at capacity).
func BenchmarkMemoryCache_Set_Eviction(b *testing.B) {
	const cap = 500
	c := NewMemoryCache(cap)
	payload := []byte(`{"hits":[]}`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Set(fmt.Sprintf("k%d", i), payload, time.Minute)
	}
}
