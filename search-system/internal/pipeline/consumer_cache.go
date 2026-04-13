package pipeline

import (
	"context"

	"github.com/rs/zerolog"
	"github.com/segmentio/kafka-go"
	"github.com/yourusername/search-system/internal/cache"
	"github.com/yourusername/search-system/internal/search"
)

// CacheConsumer invalidates search result caches when records are deleted.
// It subscribes to the records.deleted topic and, for each event:
//   - Evicts all Redis search-result keys for the affected dataset
//   - Clears the in-process Bleve MemoryCache entries for the dataset
//
// redisCache may be nil (only in-process cache is used in that case).
type CacheConsumer struct {
	deletedReader *kafka.Reader
	redisCache    *cache.RedisCache   // nil when Redis is unavailable
	searchRouter  *search.SmartSearchRouter
	log           zerolog.Logger
}

// NewCacheConsumer creates a CacheConsumer subscribed to deletedTopic.
func NewCacheConsumer(
	broker, deletedTopic string,
	redisCache *cache.RedisCache,
	searchRouter *search.SmartSearchRouter,
	log zerolog.Logger,
) *CacheConsumer {
	return &CacheConsumer{
		deletedReader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:  []string{broker},
			Topic:    deletedTopic,
			GroupID:  "search-system-cache",
			MinBytes: 1,
			MaxBytes: 1 << 20, // 1 MB
		}),
		redisCache:   redisCache,
		searchRouter: searchRouter,
		log:          log,
	}
}

// Start launches the cache invalidation goroutine. Returns immediately.
func (c *CacheConsumer) Start(ctx context.Context) {
	go c.consume(ctx)
	c.log.Info().Msg("cache-consumer: started (deleted topic)")
}

// Close releases reader resources. Call after ctx is cancelled.
func (c *CacheConsumer) Close() {
	c.deletedReader.Close() //nolint:errcheck
}

func (c *CacheConsumer) consume(ctx context.Context) {
	for {
		msg, err := c.deletedReader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			c.log.Error().Err(err).Msg("cache-consumer: fetch error")
			continue
		}

		datasetID := string(msg.Key)
		if datasetID != "" {
			// InvalidateDataset clears both L1 (MemoryCache) and L2 (Redis) and
			// stamps a RYOW flag so the next search bypasses stale cached results.
			c.searchRouter.InvalidateDataset(ctx, datasetID)
			c.log.Debug().Str("dataset", datasetID).Msg("cache-consumer: cache invalidated")
		}

		if err := c.deletedReader.CommitMessages(ctx, msg); err != nil {
			c.log.Error().Err(err).Int64("offset", msg.Offset).Msg("cache-consumer: commit offset error")
		}
	}
}
