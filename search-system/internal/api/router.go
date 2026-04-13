package api

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/yourusername/search-system/internal/config"
	"github.com/yourusername/search-system/internal/dataset"
	"github.com/yourusername/search-system/internal/metrics"
	"github.com/yourusername/search-system/internal/outbox"
	"github.com/yourusername/search-system/internal/search"
	"github.com/yourusername/search-system/internal/upsert"
)

// corsMiddleware allows cross-origin requests from the Next.js dashboard.
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, X-API-Key")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// prometheusMiddleware records request count and latency for every route.
func prometheusMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		path := c.FullPath()
		if path == "" {
			path = "unknown"
		}
		status := strconv.Itoa(c.Writer.Status())
		dur := time.Since(start).Seconds()

		metrics.HTTPRequestsTotal.WithLabelValues(c.Request.Method, path, status).Inc()
		metrics.HTTPRequestDuration.WithLabelValues(c.Request.Method, path).Observe(dur)
	}
}

// NewRouter creates and returns the configured Gin engine with all routes registered.
func NewRouter(
	db *sql.DB,
	ob *outbox.Writer,
	cfg upsert.Config,
	appCfg config.Config,
	metaStore *dataset.MetaStore,
	searchRouter *search.SmartSearchRouter,
) *gin.Engine {
	h := New(db, ob, cfg, appCfg, metaStore, searchRouter)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(corsMiddleware())
	r.Use(prometheusMiddleware())

	// ── Infrastructure ─────────────────────────────────────────────────────
	r.GET("/health", func(c *gin.Context) {
		if err := db.PingContext(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unhealthy", "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})
	r.GET("/ready", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ready", "phase": "15"})
	})

	// ── Prometheus metrics ─────────────────────────────────────────────────
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// ── Dataset management ─────────────────────────────────────────────────
	r.GET("/datasets", h.ListDatasets)
	r.POST("/datasets", h.CreateDataset)

	// ── Record operations ──────────────────────────────────────────────────
	datasets := r.Group("/datasets/:id")
	{
		datasets.POST("/records/bulk", h.BulkUpsert)
		datasets.POST("/records/sync", h.FullSync)
		datasets.DELETE("/records/:record_id", h.DeleteRecord)
		datasets.GET("/search", h.Search)
	}

	// ── Phase 15: Dashboard API endpoints ─────────────────────────────────
	api := r.Group("/api")
	{
		api.GET("/system/stats", h.SystemStats)
		api.GET("/system/health", h.SystemHealth)
		api.GET("/activity", h.ActivityFeed)
		api.GET("/performance", h.Performance)
		api.GET("/datasets/:id/stats", h.DatasetStats)
	}

	return r
}
