package search

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/rs/zerolog"
)

// Query carries search parameters from the HTTP layer to the engine.
type Query struct {
	Term      string
	Limit     int
	Offset    int
	Fuzziness int // 0 → default (1 edit distance); max 2
}

// Hit is a single ranked search result.
type Hit struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Score float64         `json:"score"`
	Value json.RawMessage `json:"value,omitempty"`
}

// Result is the unified response from any search tier.
type Result struct {
	Hits   []Hit         `json:"hits"`
	Total  uint64        `json:"total"`
	Engine string        `json:"engine"`
	TookNs int64         `json:"took_ns"`
}

// indexDoc is the shape stored inside every Bleve document.
// Exported JSON keys must match the field names used in buildIndexMapping.
type indexDoc struct {
	Name  string `json:"name"`
	Value string `json:"value"` // raw JSON text from the JSONB column
}

// buildIndexMapping returns the Bleve index mapping shared by all tiers.
// Both "name" and "value" use the standard analyzer (Unicode tokenisation +
// lower-case), which provides consistent behaviour at every scale.
func buildIndexMapping() *mapping.IndexMappingImpl {
	textField := bleve.NewTextFieldMapping()
	textField.Analyzer = "standard"

	docMapping := bleve.NewDocumentMapping()
	docMapping.AddFieldMappingsAt("name", textField)
	docMapping.AddFieldMappingsAt("value", textField)

	m := bleve.NewIndexMapping()
	m.DefaultMapping = docMapping
	return m
}

// loadRecordsIntoBleveIndex bulk-loads all active (non-deleted) records for
// datasetID from PostgreSQL into idx using 1 000-document Bleve batches.
func loadRecordsIntoBleveIndex(
	ctx context.Context,
	db *sql.DB,
	datasetID string,
	idx bleve.Index,
	log zerolog.Logger,
) error {
	const q = `
		SELECT id,
		       COALESCE(name, ''),
		       COALESCE(value::text, '')
		FROM   records
		WHERE  dataset_id = $1
		  AND  deleted_at IS NULL`

	rows, err := db.QueryContext(ctx, q, datasetID)
	if err != nil {
		return fmt.Errorf("load records for bleve %s: %w", datasetID, err)
	}
	defer rows.Close()

	batch := idx.NewBatch()
	count := 0

	for rows.Next() {
		var id, name, value string
		if err := rows.Scan(&id, &name, &value); err != nil {
			return fmt.Errorf("scan record: %w", err)
		}
		if err := batch.Index(id, indexDoc{Name: name, Value: value}); err != nil {
			return fmt.Errorf("batch index: %w", err)
		}
		count++
		if count%1000 == 0 {
			if err := idx.Batch(batch); err != nil {
				return fmt.Errorf("bleve batch flush at %d: %w", count, err)
			}
			batch = idx.NewBatch()
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	// Flush the remaining partial batch.
	if batch.Size() > 0 {
		if err := idx.Batch(batch); err != nil {
			return fmt.Errorf("bleve final flush: %w", err)
		}
	}

	log.Info().Str("dataset", datasetID).Int("records", count).Msg("bleve index built")
	return nil
}

// searchBleveIndex executes a combined exact + fuzzy + wildcard query on idx.
//
//   - Exact match   boost 3.0 — highest priority
//   - Fuzzy match   fuzziness 1–2 — handles typos (e.g. "iPhne" → "iPhone")
//   - Wildcard      boost 0.5 — substring fallback ("phone" matches "smartphone")
func searchBleveIndex(ctx context.Context, idx bleve.Index, q Query, engine string) (*Result, error) {
	start := time.Now()

	limit := q.Limit
	if limit <= 0 {
		limit = 20
	}

	fuzziness := q.Fuzziness
	if fuzziness <= 0 {
		fuzziness = 1
	}
	if fuzziness > 2 {
		fuzziness = 2
	}

	if q.Term == "" {
		return &Result{Hits: []Hit{}, Total: 0, Engine: engine}, nil
	}

	exact := bleve.NewMatchQuery(q.Term)
	exact.SetBoost(3.0)

	fuzzy := bleve.NewFuzzyQuery(q.Term)
	fuzzy.SetFuzziness(fuzziness)

	// Leading wildcard is intentionally accepted for Phase 5 scale (<5M records).
	// Phase 6 will route large datasets to Elasticsearch which handles this natively.
	wild := bleve.NewWildcardQuery("*" + q.Term + "*")
	wild.SetBoost(0.5)

	combined := bleve.NewDisjunctionQuery(exact, fuzzy, wild)

	req := bleve.NewSearchRequestOptions(combined, limit, q.Offset, false)
	req.Fields = []string{"name", "value"}

	bleveResult, err := idx.SearchInContext(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("bleve search: %w", err)
	}

	hits := make([]Hit, 0, len(bleveResult.Hits))
	for _, h := range bleveResult.Hits {
		name, _ := h.Fields["name"].(string)
		rawVal, _ := h.Fields["value"].(string)

		var val json.RawMessage
		if rawVal != "" {
			val = json.RawMessage(rawVal)
		}

		hits = append(hits, Hit{
			ID:    h.ID,
			Name:  name,
			Score: h.Score,
			Value: val,
		})
	}

	return &Result{
		Hits:   hits,
		Total:  bleveResult.Total,
		Engine: engine,
		TookNs: time.Since(start).Nanoseconds(),
	}, nil
}
