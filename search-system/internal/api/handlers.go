package api

import (
	"database/sql"
	"encoding/json"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/search-system/internal/config"
	"github.com/yourusername/search-system/internal/dataset"
	"github.com/yourusername/search-system/internal/metrics"
	"github.com/yourusername/search-system/internal/outbox"
	"github.com/yourusername/search-system/internal/search"
	"github.com/yourusername/search-system/internal/upsert"
)

// createDatasetSQL inserts a dataset and initialises its state and count rows atomically.
const createDatasetSQL = `
WITH ins AS (
    INSERT INTO datasets (name, source) VALUES ($1, $2) RETURNING id
),
_state AS (
    INSERT INTO dataset_states (dataset_id)
    SELECT id FROM ins
),
_counts AS (
    INSERT INTO dataset_counts (dataset_id, record_count)
    SELECT id, 0 FROM ins
)
SELECT id FROM ins`

// QueryLogEntry records a single search request for the /api/performance endpoint.
type QueryLogEntry struct {
	Time      time.Time `json:"time"`
	DatasetID string    `json:"dataset_id"`
	Term      string    `json:"term"`
	Engine    string    `json:"engine"`
	LatencyMS float64   `json:"latency_ms"`
	Hits      int       `json:"hits"`
	CacheHit  bool      `json:"cache_hit"`
}

const maxQueryLog = 1000

// Handler holds all HTTP route handlers.
type Handler struct {
	db           *sql.DB
	engine       *upsert.UpsertEngine
	metaStore    *dataset.MetaStore
	searchRouter *search.SmartSearchRouter
	cfg          config.Config

	mu       sync.Mutex
	queryLog []QueryLogEntry // ring buffer capped at maxQueryLog
}

// New constructs a Handler.
func New(
	db *sql.DB,
	ob *outbox.Writer,
	cfg upsert.Config,
	appCfg config.Config,
	metaStore *dataset.MetaStore,
	searchRouter *search.SmartSearchRouter,
) *Handler {
	return &Handler{
		db:           db,
		engine:       upsert.New(db, ob, cfg),
		metaStore:    metaStore,
		searchRouter: searchRouter,
		cfg:          appCfg,
		queryLog:     make([]QueryLogEntry, 0, maxQueryLog),
	}
}

// appendQuery records a query to the in-memory log (thread-safe, capped at maxQueryLog).
func (h *Handler) appendQuery(e QueryLogEntry) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.queryLog) >= maxQueryLog {
		h.queryLog = h.queryLog[1:]
	}
	h.queryLog = append(h.queryLog, e)
}

// ─────────────────────────────────────────────────────────────
// Dataset management
// ─────────────────────────────────────────────────────────────

const listDatasetsSQL = `
SELECT d.id, d.name, COALESCE(d.source,''),
       COALESCE(dc.record_count, 0),
       COALESCE(ds.stability_score, 0.0),
       COALESCE(ds.current_tier, 'NEW')
FROM datasets d
LEFT JOIN dataset_counts dc ON dc.dataset_id = d.id
LEFT JOIN dataset_states ds ON ds.dataset_id = d.id
ORDER BY d.name`

// ListDatasets handles GET /datasets
func (h *Handler) ListDatasets(c *gin.Context) {
	rows, err := h.db.QueryContext(c.Request.Context(), listDatasetsSQL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type DatasetRow struct {
		ID             string  `json:"id"`
		Name           string  `json:"name"`
		Source         string  `json:"source"`
		RecordCount    int64   `json:"record_count"`
		StabilityScore float64 `json:"stability_score"`
		State          string  `json:"state"`
	}

	datasets := []DatasetRow{}
	for rows.Next() {
		var r DatasetRow
		if err := rows.Scan(&r.ID, &r.Name, &r.Source, &r.RecordCount, &r.StabilityScore, &r.State); err == nil {
			datasets = append(datasets, r)
		}
	}
	c.JSON(http.StatusOK, gin.H{"datasets": datasets})
}

// CreateDataset handles POST /datasets
//
// Request:  {"name": "products_catalog", "source": "erp_system"}
// Response: {"id": "<uuid>", "name": "...", "source": "..."}
func (h *Handler) CreateDataset(c *gin.Context) {
	var req struct {
		Name   string `json:"name"   binding:"required"`
		Source string `json:"source"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var id string
	err := h.db.QueryRowContext(c.Request.Context(), createDatasetSQL, req.Name, req.Source).Scan(&id)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "dataset already exists or db error: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"id": id, "name": req.Name, "source": req.Source})
}

// ─────────────────────────────────────────────────────────────
// Record operations
// ─────────────────────────────────────────────────────────────

// bulkUpsertRequest is the JSON body for POST /datasets/:id/records/bulk
type bulkUpsertRequest struct {
	SyncToken string          `json:"sync_token"`
	Records   []upsert.Record `json:"records" binding:"required,min=1"`
}

// fullSyncRequest is the JSON body for POST /datasets/:id/records/sync
type fullSyncRequest struct {
	Records []upsert.Record `json:"records" binding:"required,min=1"`
}

// BulkUpsert handles POST /datasets/:id/records/bulk
func (h *Handler) BulkUpsert(c *gin.Context) {
	datasetID := c.Param("id")

	var req bulkUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.SyncToken != "" {
		for i := range req.Records {
			req.Records[i].SyncToken = req.SyncToken
		}
	}

	t0 := time.Now()
	result := h.engine.BulkUpsert(c.Request.Context(), datasetID, req.Records)
	metrics.UpsertDuration.Observe(time.Since(t0).Seconds())

	metrics.UpsertRecordsTotal.WithLabelValues("inserted").Add(float64(result.Inserted))
	metrics.UpsertRecordsTotal.WithLabelValues("updated").Add(float64(result.Updated))
	metrics.UpsertRecordsTotal.WithLabelValues("skipped").Add(float64(result.Skipped))
	metrics.UpsertRecordsTotal.WithLabelValues("failed").Add(float64(result.Failed))

	if result.Total > 0 {
		h.metaStore.OnDatasetChanged(c.Request.Context(), datasetID)           //nolint:errcheck
		h.searchRouter.InvalidateDataset(c.Request.Context(), datasetID)
	}

	status := http.StatusOK
	if result.Failed > 0 && result.Failed == result.Total {
		status = http.StatusInternalServerError
	}
	c.JSON(status, result)
}

// FullSync handles POST /datasets/:id/records/sync
func (h *Handler) FullSync(c *gin.Context) {
	datasetID := c.Param("id")

	var req fullSyncRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	t0 := time.Now()
	result, err := h.engine.FullSync(c.Request.Context(), datasetID, req.Records)
	metrics.UpsertDuration.Observe(time.Since(t0).Seconds())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	metrics.UpsertRecordsTotal.WithLabelValues("inserted").Add(float64(result.Upserted.Inserted))
	metrics.UpsertRecordsTotal.WithLabelValues("updated").Add(float64(result.Upserted.Updated))
	metrics.UpsertRecordsTotal.WithLabelValues("skipped").Add(float64(result.Upserted.Skipped))
	metrics.UpsertRecordsTotal.WithLabelValues("failed").Add(float64(result.Upserted.Failed))

	h.metaStore.OnDatasetChanged(c.Request.Context(), datasetID) //nolint:errcheck
	h.searchRouter.InvalidateDataset(c.Request.Context(), datasetID)

	c.JSON(http.StatusOK, result)
}

// deleteRecordSQL soft-deletes a single record by its deterministic ID.
const deleteRecordSQL = `
UPDATE records
SET deleted_at = NOW(),
    updated_at = NOW()
WHERE id         = $1
  AND dataset_id = $2
  AND deleted_at IS NULL`

// DeleteRecord handles DELETE /datasets/:id/records/:record_id
func (h *Handler) DeleteRecord(c *gin.Context) {
	datasetID := c.Param("id")
	recordID := c.Param("record_id")

	res, err := h.db.ExecContext(c.Request.Context(), deleteRecordSQL, recordID, datasetID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	affected, _ := res.RowsAffected()
	if affected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "record not found or already deleted"})
		return
	}

	h.searchRouter.InvalidateDataset(c.Request.Context(), datasetID)
	c.JSON(http.StatusOK, gin.H{"deleted": recordID})
}

// Search handles GET /datasets/:id/search
func (h *Handler) Search(c *gin.Context) {
	datasetID := c.Param("id")

	term := c.Query("q")
	if term == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "q parameter is required"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	fuzziness, _ := strconv.Atoi(c.DefaultQuery("fuzziness", "1"))

	if limit <= 0 || limit > 1000 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	t0 := time.Now()
	result, err := h.searchRouter.Search(c.Request.Context(), datasetID, search.Query{
		Term:      term,
		Limit:     limit,
		Offset:    offset,
		Fuzziness: fuzziness,
	})
	latencyMS := float64(time.Since(t0).Milliseconds())

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	cacheHit := result != nil && strings.Contains(result.Engine, "cache")
	h.appendQuery(QueryLogEntry{
		Time:      time.Now(),
		DatasetID: datasetID,
		Term:      term,
		Engine:    func() string { if result != nil { return result.Engine }; return "unknown" }(),
		LatencyMS: latencyMS,
		Hits:      func() int { if result != nil { return int(result.Total) }; return 0 }(),
		CacheHit:  cacheHit,
	})

	c.JSON(http.StatusOK, result)
}

// ─────────────────────────────────────────────────────────────
// Phase 15: Dashboard API endpoints
// ─────────────────────────────────────────────────────────────

// SystemStats handles GET /api/system/stats
func (h *Handler) SystemStats(c *gin.Context) {
	ctx := c.Request.Context()

	var totalRecords int64
	_ = h.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(record_count), 0) FROM dataset_counts`).Scan(&totalRecords)

	var outboxPending int64
	_ = h.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox WHERE status = 'PENDING'`).Scan(&outboxPending)

	h.mu.Lock()
	total := len(h.queryLog)
	var cacheHits int
	var totalLatency float64
	today := 0
	midnight := time.Now().Truncate(24 * time.Hour)
	for _, q := range h.queryLog {
		if q.Time.After(midnight) {
			today++
		}
		if q.CacheHit {
			cacheHits++
		}
		totalLatency += q.LatencyMS
	}
	h.mu.Unlock()

	cacheHitRate := 0.0
	avgLatencyMS := 0.0
	if total > 0 {
		cacheHitRate = float64(cacheHits) / float64(total) * 100
		avgLatencyMS = totalLatency / float64(total)
	}

	c.JSON(http.StatusOK, gin.H{
		"total_records":  totalRecords,
		"searches_today": today,
		"cache_hit_rate": cacheHitRate,
		"avg_latency_ms": avgLatencyMS,
		"outbox_pending": outboxPending,
	})
}

// SystemHealth handles GET /api/system/health
func (h *Handler) SystemHealth(c *gin.Context) {
	type ServiceStatus struct {
		Name      string `json:"name"`
		OK        bool   `json:"ok"`
		LatencyMS int64  `json:"latency_ms"`
	}

	services := make([]ServiceStatus, 0, 4)
	allOK := true
	httpClient := &http.Client{Timeout: 2 * time.Second}

	// PostgreSQL
	t := time.Now()
	pgOK := h.db.PingContext(c.Request.Context()) == nil
	services = append(services, ServiceStatus{"postgresql", pgOK, time.Since(t).Milliseconds()})
	if !pgOK {
		allOK = false
	}

	// Elasticsearch — HTTP health check
	esHost := h.cfg.Elasticsearch.Host
	if esHost == "" {
		esHost = "http://localhost:9200"
	}
	t = time.Now()
	esResp, esErr := httpClient.Get(esHost + "/_cluster/health")
	esOK := esErr == nil && esResp != nil && esResp.StatusCode < 500
	if esResp != nil {
		esResp.Body.Close()
	}
	services = append(services, ServiceStatus{"elasticsearch", esOK, time.Since(t).Milliseconds()})
	if !esOK {
		allOK = false
	}

	// Redis — TCP dial
	redisHost := h.cfg.Redis.Host
	if redisHost == "" {
		redisHost = "localhost:6379"
	}
	t = time.Now()
	rConn, rErr := net.DialTimeout("tcp", redisHost, 2*time.Second)
	redisOK := rErr == nil
	if rConn != nil {
		rConn.Close()
	}
	services = append(services, ServiceStatus{"redis", redisOK, time.Since(t).Milliseconds()})
	if !redisOK {
		allOK = false
	}

	// Kafka — TCP dial
	kafkaAddr := h.cfg.Kafka.Broker
	if kafkaAddr == "" {
		kafkaAddr = "localhost:9092"
	}
	t = time.Now()
	kConn, kErr := net.DialTimeout("tcp", kafkaAddr, 2*time.Second)
	kafkaOK := kErr == nil
	if kConn != nil {
		kConn.Close()
	}
	services = append(services, ServiceStatus{"kafka", kafkaOK, time.Since(t).Milliseconds()})
	if !kafkaOK {
		allOK = false
	}

	c.JSON(http.StatusOK, gin.H{"services": services, "all_ok": allOK})
}

// DatasetStats handles GET /api/datasets/:id/stats
func (h *Handler) DatasetStats(c *gin.Context) {
	ctx := c.Request.Context()
	datasetID := c.Param("id")

	type SyncEntry struct {
		Time     time.Time `json:"time"`
		Type     string    `json:"type"`
		Inserted int64     `json:"inserted"`
		Skipped  int64     `json:"skipped"`
		Failed   int64     `json:"failed"`
	}

	rows, err := h.db.QueryContext(ctx, `
		SELECT created_at, event_type,
			COALESCE((payload->>'inserted')::bigint, 0),
			COALESCE((payload->>'skipped')::bigint, 0),
			COALESCE((payload->>'failed')::bigint, 0)
		FROM outbox
		WHERE dataset_id = $1
		ORDER BY created_at DESC
		LIMIT 10`, datasetID)

	history := []SyncEntry{}
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var e SyncEntry
			if scanErr := rows.Scan(&e.Time, &e.Type, &e.Inserted, &e.Skipped, &e.Failed); scanErr == nil {
				history = append(history, e)
			}
		}
	}

	// Discover value fields from a sample record
	var sampleJSON []byte
	_ = h.db.QueryRowContext(ctx, `
		SELECT value FROM records WHERE dataset_id = $1 AND deleted_at IS NULL LIMIT 1`,
		datasetID).Scan(&sampleJSON)

	var valueFields []string
	if len(sampleJSON) > 0 {
		var obj map[string]interface{}
		if json.Unmarshal(sampleJSON, &obj) == nil {
			for k := range obj {
				valueFields = append(valueFields, k)
			}
			sort.Strings(valueFields)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"sync_history": history,
		"value_fields": valueFields,
	})
}

// ActivityFeed handles GET /api/activity
func (h *Handler) ActivityFeed(c *gin.Context) {
	ctx := c.Request.Context()
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	type ActivityEvent struct {
		Time      time.Time `json:"time"`
		Type      string    `json:"type"`
		Dataset   string    `json:"dataset"`
		Message   string    `json:"message"`
		Engine    string    `json:"engine"`
		LatencyMS float64   `json:"latency_ms"`
	}

	rows, err := h.db.QueryContext(ctx, `
		SELECT o.created_at, o.event_type, d.name, o.payload
		FROM outbox o
		JOIN datasets d ON d.id = o.dataset_id
		ORDER BY o.created_at DESC
		LIMIT $1`, limit)

	events := []ActivityEvent{}
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var ev ActivityEvent
			var payload []byte
			if scanErr := rows.Scan(&ev.Time, &ev.Type, &ev.Dataset, &payload); scanErr == nil {
				var p map[string]interface{}
				if json.Unmarshal(payload, &p) == nil {
					if msg, ok := p["message"].(string); ok {
						ev.Message = msg
					}
					if eng, ok := p["engine"].(string); ok {
						ev.Engine = eng
					}
					if lat, ok := p["latency_ms"].(float64); ok {
						ev.LatencyMS = lat
					}
				}
				events = append(events, ev)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"events": events})
}

// Performance handles GET /api/performance
func (h *Handler) Performance(c *gin.Context) {
	n, _ := strconv.Atoi(c.DefaultQuery("n", "1000"))
	if n <= 0 || n > maxQueryLog {
		n = maxQueryLog
	}

	h.mu.Lock()
	log := make([]QueryLogEntry, len(h.queryLog))
	copy(log, h.queryLog)
	h.mu.Unlock()

	if len(log) > n {
		log = log[len(log)-n:]
	}

	latencies := make([]float64, len(log))
	for i, q := range log {
		latencies[i] = q.LatencyMS
	}
	sort.Float64s(latencies)

	p50, p95, p99 := 0.0, 0.0, 0.0
	if len(latencies) > 0 {
		p50 = percentile(latencies, 50)
		p95 = percentile(latencies, 95)
		p99 = percentile(latencies, 99)
	}

	var cacheHits int
	for _, q := range log {
		if q.CacheHit {
			cacheHits++
		}
	}
	cacheHitRate := 0.0
	if len(log) > 0 {
		cacheHitRate = float64(cacheHits) / float64(len(log)) * 100
	}

	c.JSON(http.StatusOK, gin.H{
		"p50":            p50,
		"p95":            p95,
		"p99":            p99,
		"cache_hit_rate": cacheHitRate,
		"queries":        log,
	})
}

func percentile(sorted []float64, p int) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(p)/100.0*float64(len(sorted)-1) + 0.5)
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}
