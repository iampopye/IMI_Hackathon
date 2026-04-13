package search

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/rs/zerolog"
	"github.com/yourusername/search-system/internal/cache"
	"github.com/yourusername/search-system/internal/dataset"
	"github.com/yourusername/search-system/internal/metrics"
)

// SmartSearchRouter routes every query to the optimal engine based on dataset
// tier (small / medium / large), with a fallback chain if the primary fails.
//
// Routing logic (Phase 6):
//
//	TierSmall  (<100K)   → Bleve in-memory   fallback: Bleve file
//	TierMedium (100K–5M) → Bleve file-backed fallback: Bleve in-memory
//	TierLarge  (5M+)     → Elasticsearch     fallback: Bleve file → Bleve in-memory
//
// Cache layers (Phase 7):
//
//	L1 — in-process MemoryCache  (<1ms)
//	L2 — Redis RedisCache        (<2ms)
//
// Cache is optional: if SetCache is never called the router behaves exactly as
// in Phase 6. es may be nil when Elasticsearch is unavailable; TierLarge falls
// back to Bleve file-backed search automatically.
type SmartSearchRouter struct {
	profiler    *dataset.Profiler
	metaStore   *dataset.MetaStore
	bleveMemory *MemoryEngine
	bleveFile   *FileEngine
	es          *ESEngine // Phase 6 — nil if ES is not configured
	db          *sql.DB
	log         zerolog.Logger

	// Phase 7 cache — both fields are nil until SetCache is called.
	redisCache *cache.RedisCache
	memCache   *cache.MemoryCache
}

// NewSmartSearchRouter constructs a router.
// es may be nil; TierLarge will fall back to Bleve file-backed search.
func NewSmartSearchRouter(
	profiler *dataset.Profiler,
	metaStore *dataset.MetaStore,
	bleveMemory *MemoryEngine,
	bleveFile *FileEngine,
	es *ESEngine,
	db *sql.DB,
	log zerolog.Logger,
) *SmartSearchRouter {
	return &SmartSearchRouter{
		profiler:    profiler,
		metaStore:   metaStore,
		bleveMemory: bleveMemory,
		bleveFile:   bleveFile,
		es:          es,
		db:          db,
		log:         log,
	}
}

// SetCache wires the Phase 7 cache layers into the router.
// Either argument may be nil to disable that layer independently.
func (r *SmartSearchRouter) SetCache(rc *cache.RedisCache, mc *cache.MemoryCache) {
	r.redisCache = rc
	r.memCache = mc
}

// Search executes the full routing pipeline:
//
//  1. Log access (async, for warmup ranking)
//  2. Check RYOW flag — bypass cache if dataset has recent un-propagated writes
//  3. Check L1 in-process memory cache
//  4. Check L2 Redis cache; on hit backfill L1 and return
//  5. Evaluate tier via Profiler (with hysteresis)
//  6. Route to primary engine for that tier; fallback on error
//  7. Store result in L1 and L2 with stability-aware TTL
func (r *SmartSearchRouter) Search(ctx context.Context, datasetID string, q Query) (*Result, error) {
	start := time.Now()

	// Fire-and-forget access log — does not block the query path.
	go dataset.LogAccess(context.Background(), r.db, datasetID)

	// ── Phase 7: cache lookup ─────────────────────────────────────────────────
	var (
		cacheKey  string
		skipCache bool
	)

	if r.memCache != nil || r.redisCache != nil {
		queryHash := cache.HashQuery(q.Term, q.Limit, q.Offset, q.Fuzziness)
		cacheKey = cache.SearchKey(datasetID, queryHash)

		// Skip cache if there are un-propagated recent writes for this dataset.
		if r.redisCache != nil && r.redisCache.HasRYOW(ctx, datasetID) {
			skipCache = true
		}

		if !skipCache {
			// L1 — in-process memory cache (sub-millisecond).
			if r.memCache != nil {
				if data, ok := r.memCache.Get(cacheKey); ok {
					metrics.CacheHitsTotal.WithLabelValues("L1").Inc()
					var result Result
					if err := json.Unmarshal(data, &result); err == nil {
						result.Engine += "+mem_cache"
						metrics.SearchRequestsTotal.WithLabelValues(result.Engine, "L1", "ok").Inc()
						return &result, nil
					}
				} else {
					metrics.CacheMissesTotal.WithLabelValues("L1").Inc()
				}
			}

			// L2 — Redis cache (<2ms).
			if r.redisCache != nil {
				if data, err := r.redisCache.GetBytes(ctx, cacheKey); data != nil && err == nil {
					metrics.CacheHitsTotal.WithLabelValues("L2").Inc()
					var result Result
					if err := json.Unmarshal(data, &result); err == nil {
						// Backfill L1 so the next request never hits Redis.
						if r.memCache != nil {
							r.memCache.Set(cacheKey, data, cache.TTLSearchStable)
						}
						result.Engine += "+redis_cache"
						metrics.SearchRequestsTotal.WithLabelValues(result.Engine, "L2", "ok").Inc()
						return &result, nil
					}
				} else {
					metrics.CacheMissesTotal.WithLabelValues("L2").Inc()
				}
			}
		}
	}

	// ── Engine routing ────────────────────────────────────────────────────────
	tier, err := r.profiler.EvaluateTier(ctx, datasetID)
	if err != nil {
		r.log.Warn().Err(err).Str("dataset", datasetID).Msg("search: tier eval failed, defaulting to small")
		tier = dataset.TierSmall
	}
	tierLabel := tier.String()

	var result *Result

	switch tier {

	case dataset.TierSmall:
		result, err = r.bleveMemory.Search(ctx, datasetID, q)
		if err != nil {
			r.log.Warn().Err(err).Str("dataset", datasetID).Msg("search: bleve_memory failed, falling back to bleve_file")
			result, err = r.bleveFile.Search(ctx, datasetID, q)
		}

	case dataset.TierMedium:
		result, err = r.bleveFile.Search(ctx, datasetID, q)
		if err != nil {
			r.log.Warn().Err(err).Str("dataset", datasetID).Msg("search: bleve_file failed, falling back to bleve_memory")
			result, err = r.bleveMemory.Search(ctx, datasetID, q)
		}

	default: // TierLarge (5M+) — Elasticsearch primary, Bleve file fallback.
		if r.es != nil {
			result, err = r.es.Search(ctx, datasetID, q)
			if err != nil {
				r.log.Warn().Err(err).Str("dataset", datasetID).Msg("search: elasticsearch failed, falling back to bleve_file")
			}
		}
		if result == nil {
			result, err = r.bleveFile.Search(ctx, datasetID, q)
			if err != nil {
				r.log.Warn().Err(err).Str("dataset", datasetID).Msg("search: bleve_file (large fallback) failed, falling back to bleve_memory")
				result, err = r.bleveMemory.Search(ctx, datasetID, q)
			}
		}
	}

	// ── Phase 10: record search outcome ──────────────────────────────────────
	dur := time.Since(start).Seconds()
	metrics.SearchDuration.WithLabelValues(tierLabel).Observe(dur)

	if err != nil {
		metrics.SearchRequestsTotal.WithLabelValues(tierLabel, "none", "error").Inc()
		return nil, err
	}
	metrics.SearchRequestsTotal.WithLabelValues(tierLabel, "none", "ok").Inc()

	// ── Phase 7: cache store ──────────────────────────────────────────────────
	if cacheKey != "" && result != nil {
		// Choose TTL based on dataset stability; default to the shorter window.
		ttl := cache.TTLSearchNew
		if meta, metaErr := r.metaStore.Get(ctx, datasetID); metaErr == nil && meta.IsStable() {
			ttl = cache.TTLSearchStable
		}

		if data, marshalErr := json.Marshal(result); marshalErr == nil {
			if r.memCache != nil {
				r.memCache.Set(cacheKey, data, ttl)
			}
			if r.redisCache != nil {
				r.redisCache.SetBytes(ctx, cacheKey, data, ttl)
			}
		}
	}

	return result, nil
}

// InvalidateDataset clears all cached search results for datasetID from both
// cache layers and stamps a RYOW flag to prevent stale reads.
// Call this after any write (upsert / delete) that touches the dataset.
func (r *SmartSearchRouter) InvalidateDataset(ctx context.Context, datasetID string) {
	prefix := "search:" + datasetID + ":"
	if r.memCache != nil {
		r.memCache.InvalidatePrefix(prefix)
	}
	if r.redisCache != nil {
		r.redisCache.InvalidateDataset(ctx, datasetID)
		r.redisCache.SetRYOW(ctx, datasetID)
	}
}

// PreloadDataset builds or opens the Bleve index for datasetID based on its
// current tier.  Called by warmup at startup to eliminate cold-start latency.
func (r *SmartSearchRouter) PreloadDataset(ctx context.Context, datasetID string) {
	meta, err := r.metaStore.Get(ctx, datasetID)
	if err != nil {
		r.log.Error().Err(err).Str("dataset", datasetID).Msg("warmup: get meta")
		return
	}

	var buildErr error
	switch meta.CurrentTier {
	case dataset.TierSmall:
		_, buildErr = r.bleveMemory.Ensure(ctx, datasetID)
	case dataset.TierMedium:
		_, buildErr = r.bleveFile.Ensure(ctx, datasetID)
	default: // TierLarge — ensure ES index exists; also keep a Bleve file index as fallback.
		if r.es != nil {
			buildErr = r.es.EnsureIndex(ctx, datasetID)
		} else {
			_, buildErr = r.bleveFile.Ensure(ctx, datasetID)
		}
	}

	if buildErr != nil {
		r.log.Error().Err(buildErr).Str("dataset", datasetID).
			Str("tier", meta.CurrentTier.String()).Msg("warmup: index build failed")
	}
}
