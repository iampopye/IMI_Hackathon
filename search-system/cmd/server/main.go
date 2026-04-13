package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/yourusername/search-system/internal/api"
	"github.com/yourusername/search-system/internal/cache"
	"github.com/yourusername/search-system/internal/config"
	"github.com/yourusername/search-system/internal/dataset"
	"github.com/yourusername/search-system/internal/db"
	"github.com/yourusername/search-system/internal/outbox"
	"github.com/yourusername/search-system/internal/pipeline"
	"github.com/yourusername/search-system/internal/reconciler"
	"github.com/yourusername/search-system/internal/search"
	"github.com/yourusername/search-system/internal/upsert"
)

func main() {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	// ── Config ────────────────────────────────────────────────────────────────
	cfg, err := config.Load("config/config.yaml")
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to load config")
	}
	logger.Info().Str("env", cfg.App.Env).Int("port", cfg.App.Port).Msg("config loaded")

	// ── Database ──────────────────────────────────────────────────────────────
	database, err := db.Connect(cfg.Postgres)
	if err != nil {
		logger.Fatal().Err(err).
			Str("host", cfg.Postgres.Host).
			Msg("failed to connect to PostgreSQL")
	}
	defer database.Close()
	logger.Info().Str("host", cfg.Postgres.Host).Msg("connected to PostgreSQL")

	// ── Migrations ────────────────────────────────────────────────────────────
	if err := db.Migrate(database); err != nil {
		logger.Fatal().Err(err).Msg("migrations failed")
	}
	logger.Info().Msg("all migrations applied")

	// ── Phase 2 components ────────────────────────────────────────────────────
	ob := outbox.New(database)

	engineCfg := upsert.Config{
		BatchSize:   cfg.Search.BatchSize,
		WorkerCount: cfg.Search.WorkerCount,
	}

	// ── Phase 4 components ────────────────────────────────────────────────────
	metaStore := dataset.NewMetaStore(
		database,
		cfg.Search.StabilityThreshold,
		cfg.Search.StabilityTick,
		cfg.Search.StabilityDecay,
	)

	profiler := dataset.NewProfiler(
		database,
		metaStore,
		int64(cfg.Search.InMemoryLimit),
		int64(cfg.Search.BleveFileLimit),
		cfg.Search.TierUpgradeConfirmations,
	)

	monitor := dataset.NewMonitor(database, metaStore, profiler, logger)

	warmup := dataset.NewWarmup(database, cfg.App.WarmupDatasets, logger)

	// ── Phase 5 components ────────────────────────────────────────────────────
	bleveDataDir := cfg.Search.BleveDataDir
	if bleveDataDir == "" {
		bleveDataDir = "./data/bleve"
	}

	bleveMemory := search.NewMemoryEngine(database, logger)
	bleveFile := search.NewFileEngine(bleveDataDir, database, logger)
	defer bleveFile.Close()

	// ── Phase 6 components ────────────────────────────────────────────────────
	// ES is optional — if unavailable the router falls back to Bleve file for large datasets.
	var esEngine *search.ESEngine
	if cfg.Elasticsearch.Host != "" {
		var esErr error
		esEngine, esErr = search.NewESEngine(cfg.Elasticsearch.Host, cfg.Elasticsearch.IndexPrefix, database, logger)
		if esErr != nil {
			logger.Warn().Err(esErr).Msg("elasticsearch unavailable — large-tier search will use bleve_file fallback")
			esEngine = nil
		} else {
			logger.Info().Str("host", cfg.Elasticsearch.Host).Msg("elasticsearch client initialised")
		}
	}

	searchRouter := search.NewSmartSearchRouter(profiler, metaStore, bleveMemory, bleveFile, esEngine, database, logger)

	// ── Phase 7 components ────────────────────────────────────────────────────
	// Redis is optional — if unavailable only the in-process L1 cache is used.
	memCache := cache.NewMemoryCache(cfg.Redis.MemoryCapacity)

	if cfg.Redis.Host != "" {
		redisCache, redisErr := cache.NewRedisCache(cfg.Redis.Host, cfg.Redis.Password, logger)
		if redisErr != nil {
			logger.Warn().Err(redisErr).Msg("redis unavailable — search cache will use in-process L1 only")
		} else {
			defer redisCache.Close()
			searchRouter.SetCache(redisCache, memCache)
			logger.Info().Str("host", cfg.Redis.Host).Msg("redis cache initialised")
		}
	}
	if cfg.Redis.Host == "" {
		searchRouter.SetCache(nil, memCache)
	}

	// Wire the Bleve preloader into warmup so startup preloads real indexes.
	warmup.SetPreloader(searchRouter.PreloadDataset)

	// Run startup warmup synchronously (before accepting traffic).
	warmup.Run(context.Background())
	logger.Info().Msg("startup warmup complete")

	// Start background monitor (ticks stability scores every minute).
	monitorCtx, monitorCancel := context.WithCancel(context.Background())
	defer monitorCancel()
	go monitor.Start(monitorCtx)
	logger.Info().Msg("dataset monitor started")

	// ── Phase 8 components ────────────────────────────────────────────────────
	// Kafka is optional — if broker is empty the pipeline is skipped entirely.
	pipelineCtx, pipelineCancel := context.WithCancel(context.Background())
	defer pipelineCancel()

	if cfg.Kafka.Broker != "" {
		producer := pipeline.NewProducer(cfg.Kafka.Broker, logger)
		defer producer.Close() //nolint:errcheck

		// Outbox poller: reads PENDING rows → publishes to Kafka → marks PUBLISHED.
		pollInterval := cfg.Search.OutboxPollInterval
		if pollInterval == 0 {
			pollInterval = 500 * time.Millisecond
		}
		maxAttempts := cfg.Search.OutboxMaxAttempts
		if maxAttempts == 0 {
			maxAttempts = 5
		}
		poller := outbox.NewPoller(database, producer, pollInterval, maxAttempts, logger)
		go poller.Start(pipelineCtx)

		// ES consumer: indexes upserted records, removes deleted records from ES.
		if esEngine != nil {
			esConsumer := pipeline.NewESConsumer(
				cfg.Kafka.Broker,
				cfg.Kafka.Topics.Upserted,
				cfg.Kafka.Topics.Deleted,
				esEngine,
				database,
				logger,
			)
			defer esConsumer.Close()
			esConsumer.Start(pipelineCtx)
		}

		// Cache consumer: invalidates Redis + Bleve caches on deletion events.
		var redisForConsumer *cache.RedisCache
		if cfg.Redis.Host != "" {
			// Best-effort: reuse an existing connection for the consumer.
			// Errors here are non-fatal — cache invalidation degrades gracefully.
			if rc, rcErr := cache.NewRedisCache(cfg.Redis.Host, cfg.Redis.Password, logger); rcErr == nil {
				defer rc.Close()
				redisForConsumer = rc
			}
		}
		cacheConsumer := pipeline.NewCacheConsumer(
			cfg.Kafka.Broker,
			cfg.Kafka.Topics.Deleted,
			redisForConsumer,
			searchRouter,
			logger,
		)
		defer cacheConsumer.Close()
		cacheConsumer.Start(pipelineCtx)

		logger.Info().
			Str("broker", cfg.Kafka.Broker).
			Dur("poll_interval", pollInterval).
			Msg("phase 8: outbox poller + kafka pipeline started")
	} else {
		logger.Warn().Msg("phase 8: kafka broker not configured — outbox pipeline disabled")
	}

	// ── Phase 9 components ────────────────────────────────────────────────────
	var esForReconciler reconciler.ESEngine
	if esEngine != nil {
		esForReconciler = esEngine
	}
	rec := reconciler.New(database, esForReconciler, reconciler.Config{
		Interval: cfg.Search.ReconcileInterval,
	}, logger)
	reconcilerCtx, reconcilerCancel := context.WithCancel(context.Background())
	defer reconcilerCancel()
	go rec.Start(reconcilerCtx)
	logger.Info().Msg("phase 9: reconciler started")

	// ── Phase 10: Prometheus metrics ─────────────────────────────────────────
	// Metrics are served on /metrics of the main API server (registered in api.NewRouter).
	// A dedicated scrape target can be configured in Prometheus as:
	//   - job_name: search-system
	//     static_configs:
	//       - targets: ['localhost:8080']
	//     metrics_path: /metrics
	logger.Info().Int("port", cfg.App.Port).Msg("phase 10: prometheus metrics available at /metrics")

	// ── HTTP server ───────────────────────────────────────────────────────────
	if cfg.App.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := api.NewRouter(database, ob, engineCfg, *cfg, metaStore, searchRouter)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.App.Port),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// ── Phase 11: TLS ─────────────────────────────────────────────────────────
	go func() {
		logger.Info().Int("port", cfg.App.Port).Msg("server listening")
		var srvErr error
		if cfg.TLS.CertFile != "" && cfg.TLS.KeyFile != "" {
			logger.Info().Str("cert", cfg.TLS.CertFile).Msg("phase 11: TLS enabled")
			srvErr = srv.ListenAndServeTLS(cfg.TLS.CertFile, cfg.TLS.KeyFile)
		} else {
			srvErr = srv.ListenAndServe()
		}
		if srvErr != nil && srvErr != http.ErrServerClosed {
			logger.Fatal().Err(srvErr).Msg("server error")
		}
	}()

	// ── Graceful shutdown ─────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info().Msg("shutdown signal received")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error().Err(err).Msg("server forced to shutdown")
	}

	logger.Info().Msg("server exited cleanly")
}
