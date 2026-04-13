package cache

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestMemoryCache_SetAndGet verifies that a stored value is retrievable.
func TestMemoryCache_SetAndGet(t *testing.T) {
	c := NewMemoryCache(10)
	c.Set("k1", []byte("hello"), time.Minute)

	got, ok := c.Get("k1")
	if !ok {
		t.Fatal("Get() returned miss for a key that was just Set")
	}
	if string(got) != "hello" {
		t.Errorf("Get() = %q, want %q", got, "hello")
	}
}

// TestMemoryCache_Miss verifies that a non-existent key returns a cache miss.
func TestMemoryCache_Miss(t *testing.T) {
	c := NewMemoryCache(10)
	_, ok := c.Get("nonexistent")
	if ok {
		t.Error("Get() returned hit for a key that was never Set")
	}
}

// TestMemoryCache_TTLExpiry verifies that an expired entry is evicted on Get.
func TestMemoryCache_TTLExpiry(t *testing.T) {
	c := NewMemoryCache(10)
	c.Set("ttl-key", []byte("val"), time.Nanosecond) // expires immediately

	time.Sleep(2 * time.Millisecond)

	_, ok := c.Get("ttl-key")
	if ok {
		t.Error("Get() returned hit for an expired entry")
	}
}

// TestMemoryCache_LRUEviction verifies that the least-recently-used entry is
// evicted when the cache is full.
func TestMemoryCache_LRUEviction(t *testing.T) {
	c := NewMemoryCache(3)
	c.Set("a", []byte("a"), time.Minute)
	c.Set("b", []byte("b"), time.Minute)
	c.Set("c", []byte("c"), time.Minute)

	// Access "a" and "b" so "c" becomes the LRU entry.
	c.Get("a")
	c.Get("b")

	// Inserting "d" must evict "c" (LRU).
	c.Set("d", []byte("d"), time.Minute)

	if _, ok := c.Get("c"); ok {
		t.Error("evicted entry 'c' should not be in cache")
	}
	if _, ok := c.Get("a"); !ok {
		t.Error("recently-used entry 'a' was incorrectly evicted")
	}
	if _, ok := c.Get("d"); !ok {
		t.Error("newly inserted entry 'd' should be in cache")
	}
}

// TestMemoryCache_Update verifies that re-setting an existing key updates its
// value without growing the cache.
func TestMemoryCache_Update(t *testing.T) {
	c := NewMemoryCache(10)
	c.Set("k", []byte("v1"), time.Minute)
	c.Set("k", []byte("v2"), time.Minute)

	got, ok := c.Get("k")
	if !ok || string(got) != "v2" {
		t.Errorf("after update: Get() = %q, ok=%v, want v2,true", got, ok)
	}
	if c.order.Len() != 1 {
		t.Errorf("LRU list len = %d after overwrite, want 1", c.order.Len())
	}
}

// TestMemoryCache_InvalidatePrefix verifies that all keys with the given prefix
// are removed while unrelated keys are left intact.
func TestMemoryCache_InvalidatePrefix(t *testing.T) {
	c := NewMemoryCache(20)
	c.Set("ds:001:search:foo", []byte("1"), time.Minute)
	c.Set("ds:001:search:bar", []byte("2"), time.Minute)
	c.Set("ds:002:search:foo", []byte("3"), time.Minute)

	c.InvalidatePrefix("ds:001:")

	if _, ok := c.Get("ds:001:search:foo"); ok {
		t.Error("ds:001:search:foo should have been invalidated")
	}
	if _, ok := c.Get("ds:001:search:bar"); ok {
		t.Error("ds:001:search:bar should have been invalidated")
	}
	if _, ok := c.Get("ds:002:search:foo"); !ok {
		t.Error("ds:002:search:foo should NOT have been invalidated (different prefix)")
	}
}

// TestMemoryCache_DefaultCapacity verifies that capacity≤0 defaults to 1000.
func TestMemoryCache_DefaultCapacity(t *testing.T) {
	c := NewMemoryCache(0)
	if c.cap != 1000 {
		t.Errorf("cap = %d, want 1000 for zero input", c.cap)
	}
}

// TestMemoryCache_ConcurrentReadWrite verifies there are no data races under
// concurrent read/write load. Run with -race.
func TestMemoryCache_ConcurrentReadWrite(t *testing.T) {
	c := NewMemoryCache(100)
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func(n int) {
			defer wg.Done()
			c.Set(fmt.Sprintf("key-%d", n), []byte("v"), time.Minute)
		}(i)
		go func(n int) {
			defer wg.Done()
			c.Get(fmt.Sprintf("key-%d", n))
		}(i)
	}
	wg.Wait()
}
