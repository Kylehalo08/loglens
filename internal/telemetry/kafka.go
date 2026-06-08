package telemetry

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
)

func KafkaBrokers() []string {
	raw := os.Getenv("KAFKA_BROKERS")
	if raw == "" {
		return []string{"localhost:9092"}
	}
	parts := strings.Split(raw, ",")
	brokers := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			brokers = append(brokers, trimmed)
		}
	}
	return brokers
}

func KafkaTopic() string {
	if topic := os.Getenv("KAFKA_TOPIC_LOGS"); topic != "" {
		return topic
	}
	return "loglens.logs"
}

func KafkaConsumerGroup() string {
	if group := os.Getenv("KAFKA_CONSUMER_GROUP"); group != "" {
		return group
	}
	return "loglens-consumer"
}

type Producer struct {
	writer *kafka.Writer
}

func NewProducer() *Producer {
	return &Producer{
		writer: &kafka.Writer{
			Addr:         kafka.TCP(KafkaBrokers()...),
			Topic:        KafkaTopic(),
			RequiredAcks: kafka.RequireAll,
			Balancer:     &kafka.LeastBytes{},
		},
	}
}

func (p *Producer) Publish(ctx context.Context, entry LogEntry) error {
	payload, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	return p.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(entry.ServiceID),
		Value: payload,
		Time:  time.Now().UTC(),
	})
}

func (p *Producer) Close() error {
	if p == nil || p.writer == nil {
		return nil
	}
	return p.writer.Close()
}

type Consumer struct {
	reader *kafka.Reader
}

func NewConsumer() *Consumer {
	return &Consumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:        KafkaBrokers(),
			Topic:          KafkaTopic(),
			GroupID:        KafkaConsumerGroup(),
			MinBytes:       1,
			MaxBytes:       10e6,
			MaxWait:        500 * time.Millisecond,
			CommitInterval: time.Second,
			StartOffset:    kafka.FirstOffset,
		}),
	}
}

func (c *Consumer) Fetch(ctx context.Context) (LogEntry, kafka.Message, error) {
	msg, err := c.reader.FetchMessage(ctx)
	if err != nil {
		return LogEntry{}, kafka.Message{}, err
	}

	var entry LogEntry
	if err := json.Unmarshal(msg.Value, &entry); err != nil {
		return LogEntry{}, msg, err
	}

	if entry.Metadata == nil {
		entry.Metadata = map[string]any{}
	}

	return entry, msg, nil
}

func (c *Consumer) Commit(ctx context.Context, msg kafka.Message) error {
	return c.reader.CommitMessages(ctx, msg)
}

func (c *Consumer) Close() error {
	if c == nil || c.reader == nil {
		return nil
	}
	return c.reader.Close()
}

func IsKafkaUnavailable(err error) bool {
	return err != nil && !errors.Is(err, context.Canceled)
}
