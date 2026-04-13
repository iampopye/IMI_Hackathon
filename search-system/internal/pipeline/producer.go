package pipeline

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"github.com/segmentio/kafka-go"
)

// Producer publishes messages to Kafka topics using a single persistent writer.
// Topic is specified per message (AllowAutoTopicCreation is on for dev convenience).
type Producer struct {
	writer *kafka.Writer
	log    zerolog.Logger
}

// NewProducer constructs a Producer connected to broker.
func NewProducer(broker string, log zerolog.Logger) *Producer {
	w := &kafka.Writer{
		Addr:                   kafka.TCP(broker),
		Balancer:               &kafka.LeastBytes{},
		AllowAutoTopicCreation: true,
		WriteTimeout:           5 * time.Second,
		ReadTimeout:            5 * time.Second,
		MaxAttempts:            3,
	}
	return &Producer{writer: w, log: log}
}

// Publish sends a message to topic with the given key and value.
// key is used for partition routing — pass dataset_id for ordered per-dataset delivery.
func (p *Producer) Publish(ctx context.Context, topic string, key, value []byte) error {
	return p.writer.WriteMessages(ctx, kafka.Message{
		Topic: topic,
		Key:   key,
		Value: value,
	})
}

// Close flushes pending messages and shuts down the underlying writer.
func (p *Producer) Close() error {
	return p.writer.Close()
}
