package cache

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

// TTL constants per the Phase 7 cache key strategy.
const (
	TTLSearchStable = 10 * time.Minute // dataset is STABLE — data rarely changes
	TTLSearchNew    = 30 * time.Second // dataset is NEW / actively written
	TTLProfile      = 5 * time.Minute
	TTLAccessLog    = 24 * time.Hour
	TTLRYOW         = 30 * time.Second // Read-Your-Own-Writes safety window
	TTLTierCheck    = 10 * time.Minute

	maxValueBytes = 1 << 20 // 1 MB — never cache larger values
)

// RedisCache wraps a Redis client and provides domain-level cache operations.
// All values are stored as raw JSON bytes to avoid circular package imports.
type RedisCache struct {
	client *redis.Client
	log    zerolog.Logger
}

// NewRedisCache connects to Redis at addr and returns a ready RedisCache.
// password may be empty for unauthenticated Redis instances.
// Returns an error if Redis is unreachable within 3 seconds.
func NewRedisCache(addr, password string, log zerolog.Logger) (*RedisCache, error) {
	client := redis.NewClient(&redis.Options{Addr: addr, Password: password})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		client.Close() //nolint:errcheck
		return nil, fmt.Errorf("redis ping %s: %w", addr, err)
	}

	return &RedisCache{client: client, log: log}, nil
}

// Close releases the underlying Redis connection pool.
func (r *RedisCache) Close() error {
	return r.client.Close()
}

// HashQuery returns an 8-byte hex fingerprint of (term, limit, offset, fuzziness)
// suitable for use as the query component of a search cache key.
func HashQuery(term string, limit, offset, fuzziness int) string {
	raw := fmt.Sprintf("%s\x00%d\x00%d\x00%d", term, limit, offset, fuzziness)
	sum := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", sum[:8])
}

// SearchKey builds the composite Redis key for a cached search result.
//
//	search:{datasetID}:{queryHash}
func SearchKey(datasetID, queryHash string) string {
	return "search:" + datasetID + ":" + queryHash
}

// GetBytes retrieves raw bytes from Redis. Returns nil, nil on cache miss.
func (r *RedisCache) GetBytes(ctx context.Context, key string) ([]byte, error) {
	data, err := r.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("redis get %s: %w", key, err)
	}
	return data, nil
}

// SetBytes stores raw bytes in Redis with ttl. Skips values larger than 1 MB.
func (r *RedisCache) SetBytes(ctx context.Context, key string, data []byte, ttl time.Duration) {
	if len(data) > maxValueBytes {
		r.log.Warn().Str("key", key).Int("bytes", len(data)).Msg("cache: value >1 MB, skipping")
		return
	}
	if err := r.client.Set(ctx, key, data, ttl).Err(); err != nil {
		r.log.Warn().Err(err).Str("key", key).Msg("cache: set failed")
	}
}

// InvalidateDataset removes all search result entries for datasetID using a
// cursor-based SCAN (safe on large keyspaces), then marks the dataset state as
// NEW in Redis so any state reader sees the freshest value without a DB round-trip.
func (r *RedisCache) InvalidateDataset(ctx context.Context, datasetID string) {
	pattern := "search:" + datasetID + ":*"
	var cursor uint64
	for {
		keys, next, err := r.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			r.log.Warn().Err(err).Str("dataset", datasetID).Msg("cache: scan failed during invalidation")
			break
		}
		if len(keys) > 0 {
			if err := r.client.Del(ctx, keys...).Err(); err != nil {
				r.log.Warn().Err(err).Msg("cache: del failed")
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}

	// Persist "NEW" state without expiry — it is updated explicitly by the monitor.
	if err := r.client.Set(ctx, "state:"+datasetID, "NEW", 0).Err(); err != nil {
		r.log.Warn().Err(err).Str("dataset", datasetID).Msg("cache: state mark NEW failed")
	}
}

// SetRYOW stamps a short-lived flag indicating datasetID has pending writes.
// The search router checks this flag before serving from cache to guarantee
// Read-Your-Own-Writes consistency.
func (r *RedisCache) SetRYOW(ctx context.Context, datasetID string) {
	if err := r.client.Set(ctx, "recent_writes:"+datasetID, "1", TTLRYOW).Err(); err != nil {
		r.log.Warn().Err(err).Str("dataset", datasetID).Msg("cache: RYOW set failed")
	}
}

// HasRYOW returns true if a live recent-write flag exists for datasetID.
func (r *RedisCache) HasRYOW(ctx context.Context, datasetID string) bool {
	return r.client.Exists(ctx, "recent_writes:"+datasetID).Val() == 1
}
