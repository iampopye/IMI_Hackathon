package search

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/rs/zerolog"
)

// ESEngine implements full-text and fuzzy search using Elasticsearch.
// Used for the large tier (5M+ records). Each dataset gets its own versioned
// index behind a stable alias: {indexPrefix}{datasetID}.
//
// Index alias strategy (zero-downtime reindex):
//
//	alias:  search_{datasetID}          → live; all queries hit this
//	index:  search_{datasetID}_v{unix}  → concrete versioned index
//
// Soft-deleted records are excluded via must_not exists(deleted_at) on every
// query, providing a safety net even before Kafka deletion events are processed.
type ESEngine struct {
	client      *elasticsearch.Client
	indexPrefix string
	db          *sql.DB
	log         zerolog.Logger
}

// NewESEngine constructs an ESEngine.
// host is the full ES address (e.g. "http://localhost:9200").
// indexPrefix is prepended to every dataset alias/index name.
func NewESEngine(host, indexPrefix string, db *sql.DB, log zerolog.Logger) (*ESEngine, error) {
	cfg := elasticsearch.Config{
		Addresses: []string{host},
	}
	client, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("es: new client: %w", err)
	}
	return &ESEngine{
		client:      client,
		indexPrefix: indexPrefix,
		db:          db,
		log:         log,
	}, nil
}

// aliasName returns the stable alias for a dataset's ES index.
func (e *ESEngine) aliasName(datasetID string) string {
	return e.indexPrefix + datasetID
}

// EnsureIndex creates the versioned index + alias for datasetID if not present.
// Idempotent — safe to call on every Search.
func (e *ESEngine) EnsureIndex(ctx context.Context, datasetID string) error {
	alias := e.aliasName(datasetID)

	existsReq := esapi.IndicesExistsAliasRequest{Name: []string{alias}}
	res, err := existsReq.Do(ctx, e.client)
	if err != nil {
		return fmt.Errorf("es: check alias %s: %w", alias, err)
	}
	res.Body.Close()

	if res.StatusCode == 200 {
		return nil // already exists
	}

	// First-time creation: versioned index + alias in one call.
	versionedName := fmt.Sprintf("%s_v1", alias)
	return e.createESIndex(ctx, versionedName, alias)
}

// createESIndex creates indexName with the canonical mapping.
// If alias is non-empty the alias is created atomically with the index.
func (e *ESEngine) createESIndex(ctx context.Context, indexName, alias string) error {
	props := map[string]interface{}{
		"id":         map[string]interface{}{"type": "keyword"},
		"dataset_id": map[string]interface{}{"type": "keyword"},
		"name": map[string]interface{}{
			"type":     "text",
			"analyzer": "standard",
			"fields": map[string]interface{}{
				"keyword": map[string]interface{}{"type": "keyword"},
				"suggest": map[string]interface{}{"type": "search_as_you_type"},
			},
		},
		"value":      map[string]interface{}{"type": "object", "dynamic": true},
		"deleted_at": map[string]interface{}{"type": "date"},
		"updated_at": map[string]interface{}{"type": "date"},
	}

	m := map[string]interface{}{
		"settings": map[string]interface{}{
			"refresh_interval":   "30s",
			"number_of_shards":   1,
			"number_of_replicas": 0,
		},
		"mappings": map[string]interface{}{"properties": props},
	}
	if alias != "" {
		m["aliases"] = map[string]interface{}{alias: map[string]interface{}{}}
	}

	body, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("es: marshal mapping: %w", err)
	}

	req := esapi.IndicesCreateRequest{Index: indexName, Body: bytes.NewReader(body)}
	res, err := req.Do(ctx, e.client)
	if err != nil {
		return fmt.Errorf("es: create index %s: %w", indexName, err)
	}
	defer res.Body.Close()

	if res.IsError() {
		var errBody map[string]interface{}
		json.NewDecoder(res.Body).Decode(&errBody) //nolint:errcheck
		return fmt.Errorf("es: create index %s: %v", indexName, errBody)
	}

	e.log.Info().Str("index", indexName).Str("alias", alias).Msg("es: index created")
	return nil
}

// Search executes a boosted exact + match + fuzzy query against the dataset alias.
//
// Boost tiers:
//
//	name.keyword exact match → boost 3.0 (highest)
//	name text match          → boost 2.0
//	name fuzzy (AUTO)        → boost 1.0
//
// Soft-deleted documents are always excluded via must_not exists(deleted_at).
func (e *ESEngine) Search(ctx context.Context, datasetID string, q Query) (*Result, error) {
	start := time.Now()

	if q.Term == "" {
		return &Result{Hits: []Hit{}, Total: 0, Engine: "elasticsearch"}, nil
	}

	if err := e.EnsureIndex(ctx, datasetID); err != nil {
		return nil, err
	}

	limit := q.Limit
	if limit <= 0 {
		limit = 20
	}

	queryBody := map[string]interface{}{
		"from": q.Offset,
		"size": limit,
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"filter": []map[string]interface{}{
					{"term": map[string]interface{}{"dataset_id": datasetID}},
				},
				"must_not": []map[string]interface{}{
					{"exists": map[string]interface{}{"field": "deleted_at"}},
				},
				"should": []map[string]interface{}{
					{"term": map[string]interface{}{
						"name.keyword": map[string]interface{}{"value": q.Term, "boost": 3.0},
					}},
					{"match": map[string]interface{}{
						"name": map[string]interface{}{"query": q.Term, "boost": 2.0},
					}},
					{"match": map[string]interface{}{
						"name": map[string]interface{}{
							"query":     q.Term,
							"fuzziness": "AUTO",
							"boost":     1.0,
						},
					}},
				},
				"minimum_should_match": 1,
			},
		},
	}

	body, err := json.Marshal(queryBody)
	if err != nil {
		return nil, fmt.Errorf("es: marshal query: %w", err)
	}

	alias := e.aliasName(datasetID)
	req := esapi.SearchRequest{Index: []string{alias}, Body: bytes.NewReader(body)}
	res, err := req.Do(ctx, e.client)
	if err != nil {
		return nil, fmt.Errorf("es: search %s: %w", alias, err)
	}
	defer res.Body.Close()

	if res.IsError() {
		var errBody map[string]interface{}
		json.NewDecoder(res.Body).Decode(&errBody) //nolint:errcheck
		return nil, fmt.Errorf("es: search error: %v", errBody)
	}

	var esResult struct {
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			Hits []struct {
				ID     string          `json:"_id"`
				Score  float64         `json:"_score"`
				Source json.RawMessage `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(res.Body).Decode(&esResult); err != nil {
		return nil, fmt.Errorf("es: decode response: %w", err)
	}

	hits := make([]Hit, 0, len(esResult.Hits.Hits))
	for _, h := range esResult.Hits.Hits {
		var doc struct {
			Name  string          `json:"name"`
			Value json.RawMessage `json:"value"`
		}
		if err := json.Unmarshal(h.Source, &doc); err != nil {
			continue
		}
		hits = append(hits, Hit{
			ID:    h.ID,
			Name:  doc.Name,
			Score: h.Score,
			Value: doc.Value,
		})
	}

	return &Result{
		Hits:   hits,
		Total:  uint64(esResult.Hits.Total.Value),
		Engine: "elasticsearch",
		TookNs: time.Since(start).Nanoseconds(),
	}, nil
}

// IndexRecord upserts a single record into the dataset's ES index.
// Called by the Kafka/outbox consumer (Phase 8) on records.upserted events.
func (e *ESEngine) IndexRecord(ctx context.Context, datasetID, recordID, name string, value json.RawMessage) error {
	if err := e.EnsureIndex(ctx, datasetID); err != nil {
		return err
	}

	doc := map[string]interface{}{
		"id":         recordID,
		"dataset_id": datasetID,
		"name":       name,
		"value":      value,
		"updated_at": time.Now().UTC().Format(time.RFC3339),
	}
	body, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("es: marshal doc: %w", err)
	}

	alias := e.aliasName(datasetID)
	req := esapi.IndexRequest{
		Index:      alias,
		DocumentID: recordID,
		Body:       bytes.NewReader(body),
	}
	res, err := req.Do(ctx, e.client)
	if err != nil {
		return fmt.Errorf("es: index record %s: %w", recordID, err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("es: index record %s: %s", recordID, res.Status())
	}
	return nil
}

// DeleteRecord removes a record from the dataset's ES index.
// Called by the Kafka/outbox consumer (Phase 8) on records.deleted events.
func (e *ESEngine) DeleteRecord(ctx context.Context, datasetID, recordID string) error {
	alias := e.aliasName(datasetID)
	req := esapi.DeleteRequest{Index: alias, DocumentID: recordID}
	res, err := req.Do(ctx, e.client)
	if err != nil {
		return fmt.Errorf("es: delete record %s: %w", recordID, err)
	}
	defer res.Body.Close()

	// 404 is fine — idempotent delete.
	if res.IsError() && res.StatusCode != 404 {
		return fmt.Errorf("es: delete record %s: %s", recordID, res.Status())
	}
	return nil
}

// BulkIndexFromPostgres loads all active (non-deleted) records for datasetID
// from PostgreSQL into the dataset's ES alias using 500-document bulk batches.
// Used for initial index population and forced full reindex.
func (e *ESEngine) BulkIndexFromPostgres(ctx context.Context, datasetID string) error {
	if err := e.EnsureIndex(ctx, datasetID); err != nil {
		return err
	}
	alias := e.aliasName(datasetID)
	return e.bulkIndexInto(ctx, datasetID, alias)
}

// ReindexZeroDowntime rebuilds the ES index for datasetID without query downtime.
//
// Steps:
//  1. Create a new versioned index (no alias yet — reads still hit old index).
//  2. Bulk-load all active records from PostgreSQL into new index.
//  3. Atomic alias swap: alias now points to new index.
//  4. Delete the old index.
func (e *ESEngine) ReindexZeroDowntime(ctx context.Context, datasetID string) error {
	alias := e.aliasName(datasetID)
	newIndex := fmt.Sprintf("%s_v%d", alias, time.Now().Unix())

	// Step 1: Create bare new index (no alias — live traffic unaffected).
	if err := e.createESIndex(ctx, newIndex, ""); err != nil {
		return fmt.Errorf("es: reindex create %s: %w", newIndex, err)
	}

	// Step 2: Bulk load into the new index directly.
	if err := e.bulkIndexInto(ctx, datasetID, newIndex); err != nil {
		return fmt.Errorf("es: reindex bulk load: %w", err)
	}

	// Step 3: Resolve which index the alias currently points to.
	oldIndex, err := e.resolveAlias(ctx, alias)
	if err != nil {
		return fmt.Errorf("es: reindex resolve alias: %w", err)
	}

	// Atomic alias swap.
	actions := map[string]interface{}{
		"actions": []map[string]interface{}{
			{"remove": map[string]interface{}{"index": "*", "alias": alias}},
			{"add": map[string]interface{}{"index": newIndex, "alias": alias}},
		},
	}
	swapBody, _ := json.Marshal(actions)
	swapReq := esapi.IndicesUpdateAliasesRequest{Body: bytes.NewReader(swapBody)}
	swapRes, err := swapReq.Do(ctx, e.client)
	if err != nil {
		return fmt.Errorf("es: alias swap: %w", err)
	}
	swapRes.Body.Close()
	if swapRes.IsError() {
		return fmt.Errorf("es: alias swap error: %s", swapRes.Status())
	}

	// Step 4: Delete old index (best-effort).
	if oldIndex != "" && oldIndex != newIndex {
		delReq := esapi.IndicesDeleteRequest{Index: []string{oldIndex}}
		delRes, err := delReq.Do(ctx, e.client)
		if err != nil {
			e.log.Warn().Err(err).Str("index", oldIndex).Msg("es: delete old index failed")
		} else {
			delRes.Body.Close()
		}
	}

	e.log.Info().
		Str("dataset", datasetID).
		Str("new_index", newIndex).
		Str("old_index", oldIndex).
		Msg("es: zero-downtime reindex complete")
	return nil
}

// bulkIndexInto bulk-loads all active records for datasetID into targetIndex.
// Uses 500-document batches to bound memory usage.
func (e *ESEngine) bulkIndexInto(ctx context.Context, datasetID, targetIndex string) error {
	const q = `
		SELECT id,
		       COALESCE(name, ''),
		       COALESCE(value::text, '{}')
		FROM   records
		WHERE  dataset_id = $1
		  AND  deleted_at IS NULL`

	rows, err := e.db.QueryContext(ctx, q, datasetID)
	if err != nil {
		return fmt.Errorf("es: query records for %s: %w", datasetID, err)
	}
	defer rows.Close()

	const chunkSize = 500
	var buf bytes.Buffer
	count := 0
	now := time.Now().UTC().Format(time.RFC3339)

	flush := func() error {
		if buf.Len() == 0 {
			return nil
		}
		req := esapi.BulkRequest{Index: targetIndex, Body: bytes.NewReader(buf.Bytes())}
		res, err := req.Do(ctx, e.client)
		if err != nil {
			return fmt.Errorf("es: bulk flush: %w", err)
		}
		defer res.Body.Close()
		if res.IsError() {
			return fmt.Errorf("es: bulk error: %s", res.Status())
		}
		buf.Reset()
		return nil
	}

	for rows.Next() {
		var id, name, valueStr string
		if err := rows.Scan(&id, &name, &valueStr); err != nil {
			return fmt.Errorf("es: scan record: %w", err)
		}

		// Bulk action line.
		action := map[string]interface{}{"index": map[string]interface{}{"_id": id}}
		actionBytes, _ := json.Marshal(action)
		buf.Write(actionBytes)
		buf.WriteByte('\n')

		// Document line.
		doc := map[string]interface{}{
			"id":         id,
			"dataset_id": datasetID,
			"name":       name,
			"value":      json.RawMessage(valueStr),
			"updated_at": now,
		}
		docBytes, err := json.Marshal(doc)
		if err != nil {
			return fmt.Errorf("es: marshal bulk doc: %w", err)
		}
		buf.Write(docBytes)
		buf.WriteByte('\n')
		count++

		if count%chunkSize == 0 {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := flush(); err != nil {
		return err
	}

	e.log.Info().
		Str("dataset", datasetID).
		Str("target", targetIndex).
		Int("records", count).
		Msg("es: bulk index complete")
	return nil
}

// DocCount returns the number of active (non-soft-deleted) documents in the
// dataset's ES index. Used by the reconciler to detect drift vs PostgreSQL.
// Returns 0 if the index does not yet exist (nothing indexed yet).
func (e *ESEngine) DocCount(ctx context.Context, datasetID string) (int64, error) {
	alias := e.aliasName(datasetID)

	existsReq := esapi.IndicesExistsAliasRequest{Name: []string{alias}}
	chk, err := existsReq.Do(ctx, e.client)
	if err != nil {
		return 0, fmt.Errorf("es: check alias %s: %w", alias, err)
	}
	chk.Body.Close()
	if chk.StatusCode == 404 {
		return 0, nil // index not yet created — no drift
	}

	body := []byte(`{"query":{"bool":{"must_not":[{"exists":{"field":"deleted_at"}}]}}}`)
	req := esapi.CountRequest{Index: []string{alias}, Body: bytes.NewReader(body)}
	res, err := req.Do(ctx, e.client)
	if err != nil {
		return 0, fmt.Errorf("es: count %s: %w", alias, err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return 0, fmt.Errorf("es: count %s: %s", alias, res.Status())
	}

	var r struct {
		Count int64 `json:"count"`
	}
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		return 0, fmt.Errorf("es: decode count response: %w", err)
	}
	return r.Count, nil
}

// resolveAlias returns the concrete index name currently pointed to by alias.
// Returns "" if the alias does not exist.
func (e *ESEngine) resolveAlias(ctx context.Context, alias string) (string, error) {
	req := esapi.IndicesGetAliasRequest{Name: []string{alias}}
	res, err := req.Do(ctx, e.client)
	if err != nil {
		return "", fmt.Errorf("es: get alias %s: %w", alias, err)
	}
	defer res.Body.Close()

	if res.StatusCode == 404 {
		return "", nil
	}

	// Response shape: {"<index_name>": {"aliases": {"<alias>": {}}}}
	var body map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("es: decode alias response: %w", err)
	}
	for indexName := range body {
		return indexName, nil
	}
	return "", nil
}
