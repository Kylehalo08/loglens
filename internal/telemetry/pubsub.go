package telemetry

import (
	"context"
	"encoding/json"

	"loglens/internal/db"
)

type Publisher struct {
	redis *db.RedisStore
}

func NewPublisher(redis *db.RedisStore) *Publisher {
	return &Publisher{redis: redis}
}

func (p *Publisher) PublishLog(ctx context.Context, entry LogEntry) error {
	if p.redis == nil || !p.redis.IsAvailable() {
		return nil
	}

	payload, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	return p.redis.Client().Publish(ctx, ServiceChannel(entry.ServiceID), payload).Err()
}
