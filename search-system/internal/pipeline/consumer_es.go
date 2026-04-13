package pipeline

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/rs/zerolog"
	"github.com/segmentio/kafka-go"
	"github.com/yourusername/search-system/internal/metrics"
	"github.com/yourusername/search-system/internal/search"
)

// ESConsumer keeps Elasticsearch in sync by processing two Kafka topics:
//   - records.upserted → index the record in ES (fetches name+value from PostgreSQL)
//   - records.deleted  → remove soft-deleted records from ES
//
// Both consumers share the consumer-group "search-system-es" so offsets are
// committed once per message and replayed on restart.
type ESConsumer struct {
	upsertedReader *kafka.Reader
	deletedReader  *kafka.Reader
	esEngine       *search.ESEngine
	db             *sql.DB
	log            zerolog.Logger
}

// NewESConsumer creates an ESConsumer subscribed to upsertedTopic and deletedTopic.
func NewESConsumer(
	broker, upsertedTopic, deletedTopic string,
	esEngine *search.ESEngine,
	db *sql.DB,
	log zerolog.Logger,
) *ESConsumer {
	return &ESConsumer{
		upsertedReader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:  []string{broker},
			Topic:    upsertedTopic,
			GroupID:  "search-system-es",
			MinBytes: 1,
			MaxBytes: 10 << 20, // 10 MB
		}),
		deletedReader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:  []string{broker},
			Topic:    deletedTopic,
			GroupID:  "search-system-es",
			MinBytes: 1,
			MaxBytes: 1 << 20, // 1 MB
		}),
		esEngine: esEngine,
		db:       db,
		log:      log,
	}
}

// Start launches both consumer goroutines. Returns immediately.
// Goroutines stop when ctx is cancelled.
func (c *ESConsumer) Start(ctx context.Context) {
	go c.consumeUpserted(ctx)
	go c.consumeDeleted(ctx)
	c.log.Info().Msg("es-consumer: started (upserted + deleted topics)")
}

// Close releases reader resources. Call after ctx is cancelled.
func (c *ESConsumer) Close() {
	c.upsertedReader.Close() //nolint:errcheck
	c.deletedReader.Close()  //nolint:errcheck
}

// upsertedPayload matches the outbox payload written by UpsertEngine.processBatch.
type upsertedPayload struct {
	ID         string `json:"id"`
	DatasetID  string `json:"dataset_id"`
	ExternalID string `json:"external_id"`
	Source     string `json:"source"`
}

const fetchRecordSQL = `
SELECT COALESCE(name, ''), COALESCE(value::text, '{}')
FROM   records
WHERE  id = $1 AND deleted_at IS NULL`

func (c *ESConsumer) consumeUpserted(ctx context.Context) {
	for {
		msg, err := c.upsertedReader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			c.log.Error().Err(err).Msg("es-consumer: fetch upserted error")
			continue
		}

		if err := c.handleUpserted(ctx, msg); err != nil {
			c.log.Error().Err(err).
				Int64("offset", msg.Offset).
				Msg("es-consumer: handle upserted error — skipping message")
		}

		if err := c.upsertedReader.CommitMessages(ctx, msg); err != nil {
			c.log.Error().Err(err).Int64("offset", msg.Offset).Msg("es-consumer: commit upserted offset error")
		}
	}
}

func (c *ESConsumer) handleUpserted(ctx context.Context, msg kafka.Message) error {
	var p upsertedPayload
	if err := json.Unmarshal(msg.Value, &p); err != nil {
		return fmt.Errorf("unmarshal upserted payload: %w", err)
	}

	var name, valueStr string
	err := c.db.QueryRowContext(ctx, fetchRecordSQL, p.ID).Scan(&name, &valueStr)
	if err == sql.ErrNoRows {
		// Record deleted before we processed this event — nothing to index.
		return nil
	}
	if err != nil {
		return fmt.Errorf("fetch record %s from pg: %w", p.ID, err)
	}

	if err := c.esEngine.IndexRecord(ctx, p.DatasetID, p.ID, name, json.RawMessage(valueStr)); err != nil {
		metrics.ESIndexOperationsTotal.WithLabelValues("index", "error").Inc()
		return fmt.Errorf("es index record %s: %w", p.ID, err)
	}
	metrics.ESIndexOperationsTotal.WithLabelValues("index", "success").Inc()
	return nil
}

// fetchRecentDeletedSQL returns records soft-deleted in the last 10 minutes.
// The generous window ensures we catch every deletion even under high Kafka lag.
const fetchRecentDeletedSQL = `
SELECT id
FROM   records
WHERE  dataset_id = $1
  AND  deleted_at IS NOT NULL
  AND  updated_at > NOW() - INTERVAL '10 minutes'`

func (c *ESConsumer) consumeDeleted(ctx context.Context) {
	for {
		msg, err := c.deletedReader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			c.log.Error().Err(err).Msg("es-consumer: fetch deleted error")
			continue
		}

		if err := c.handleDeleted(ctx, msg); err != nil {
			c.log.Error().Err(err).
				Int64("offset", msg.Offset).
				Msg("es-consumer: handle deleted error — skipping message")
		}

		if err := c.deletedReader.CommitMessages(ctx, msg); err != nil {
			c.log.Error().Err(err).Int64("offset", msg.Offset).Msg("es-consumer: commit deleted offset error")
		}
	}
}

func (c *ESConsumer) handleDeleted(ctx context.Context, msg kafka.Message) error {
	datasetID := string(msg.Key)
	if datasetID == "" {
		return nil
	}

	rows, err := c.db.QueryContext(ctx, fetchRecentDeletedSQL, datasetID)
	if err != nil {
		return fmt.Errorf("fetch recently deleted records for %s: %w", datasetID, err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("scan deleted record id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, id := range ids {
		if err := c.esEngine.DeleteRecord(ctx, datasetID, id); err != nil {
			// Log but don't abort — DeleteRecord is idempotent (404 is fine).
			c.log.Error().Err(err).Str("record", id).Msg("es-consumer: delete record error")
			metrics.ESIndexOperationsTotal.WithLabelValues("delete", "error").Inc()
		} else {
			metrics.ESIndexOperationsTotal.WithLabelValues("delete", "success").Inc()
		}
	}

	c.log.Debug().
		Str("dataset", datasetID).
		Int("deleted", len(ids)).
		Msg("es-consumer: removed deleted records from ES")
	return nil
}
