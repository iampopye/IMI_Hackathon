package outbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/rs/zerolog"
	"github.com/yourusername/search-system/internal/metrics"
)

// Publisher is the interface the Poller uses to forward events to the message bus.
// pipeline.Producer satisfies this interface; it can also be mocked in tests.
type Publisher interface {
	Publish(ctx context.Context, topic string, key, value []byte) error
}

const pollBatchSize = 100

const fetchPendingSQL = `
SELECT id, event_type, dataset_id, payload, attempts
FROM   outbox
WHERE  status = 'PENDING'
ORDER  BY created_at
LIMIT  $1
FOR UPDATE SKIP LOCKED`

const markPublishedSQL = `
UPDATE outbox
SET    status       = 'PUBLISHED',
       published_at = NOW()
WHERE  id = $1`

const markFailedSQL = `
UPDATE outbox
SET    attempts = attempts + 1,
       status   = CASE WHEN attempts + 1 >= $2 THEN 'DEAD' ELSE 'PENDING' END
WHERE  id = $1`

// Poller reads PENDING outbox events and publishes them to Kafka.
// Uses SELECT FOR UPDATE SKIP LOCKED so multiple replicas do not double-publish.
// On publish success the row is marked PUBLISHED; on repeated failure it is marked DEAD.
type Poller struct {
	db           *sql.DB
	publisher    Publisher
	pollInterval time.Duration
	maxAttempts  int
	log          zerolog.Logger
}

// NewPoller constructs a Poller.
func NewPoller(db *sql.DB, pub Publisher, pollInterval time.Duration, maxAttempts int, log zerolog.Logger) *Poller {
	return &Poller{
		db:           db,
		publisher:    pub,
		pollInterval: pollInterval,
		maxAttempts:  maxAttempts,
		log:          log,
	}
}

// Start runs the poll loop until ctx is cancelled.
func (p *Poller) Start(ctx context.Context) {
	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	p.log.Info().Dur("interval", p.pollInterval).Msg("outbox: poller started")

	for {
		select {
		case <-ctx.Done():
			p.log.Info().Msg("outbox: poller stopped")
			return
		case <-ticker.C:
			if err := p.poll(ctx); err != nil {
				p.log.Error().Err(err).Msg("outbox: poll cycle error")
			}
		}
	}
}

type outboxRow struct {
	id        string
	eventType string
	datasetID string
	payload   json.RawMessage
	attempts  int
}

func (p *Poller) poll(ctx context.Context) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	rows, err := tx.QueryContext(ctx, fetchPendingSQL, pollBatchSize)
	if err != nil {
		return err
	}

	var events []outboxRow
	for rows.Next() {
		var e outboxRow
		if err := rows.Scan(&e.id, &e.eventType, &e.datasetID, &e.payload, &e.attempts); err != nil {
			rows.Close()
			return err
		}
		events = append(events, e)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	if len(events) == 0 {
		return tx.Commit()
	}

	published := 0
	for _, e := range events {
		if pubErr := p.publisher.Publish(ctx, e.eventType, []byte(e.datasetID), e.payload); pubErr != nil {
			p.log.Error().Err(pubErr).
				Str("id", e.id).
				Str("event_type", e.eventType).
				Msg("outbox: publish failed")
			if _, dbErr := tx.ExecContext(ctx, markFailedSQL, e.id, p.maxAttempts); dbErr != nil {
				p.log.Error().Err(dbErr).Str("id", e.id).Msg("outbox: mark-failed write error")
			}
			// Count as dead when attempts will reach max; otherwise it stays pending for retry.
			if e.attempts+1 >= p.maxAttempts {
				metrics.OutboxEventsTotal.WithLabelValues("dead").Inc()
				metrics.OutboxPendingGauge.Dec()
			}
		} else {
			if _, dbErr := tx.ExecContext(ctx, markPublishedSQL, e.id); dbErr != nil {
				p.log.Error().Err(dbErr).Str("id", e.id).Msg("outbox: mark-published write error")
			}
			metrics.OutboxEventsTotal.WithLabelValues("published").Inc()
			metrics.OutboxPendingGauge.Dec()
			published++
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	if published > 0 {
		p.log.Debug().Int("published", published).Int("total", len(events)).Msg("outbox: poll cycle complete")
	}
	return nil
}
