package db

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisStore struct {
	client    *redis.Client
	available bool
}

func ConnectRedis(ctx context.Context) *RedisStore {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		log.Println("warning: REDIS_ADDR not set, redis unavailable")
		return &RedisStore{available: false}
	}

	client := redis.NewClient(&redis.Options{
		Addr: addr,
	})

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := client.Ping(pingCtx).Err(); err != nil {
		log.Printf("warning: redis unavailable, falling back to database: %v", err)
		return &RedisStore{client: client, available: false}
	}

	return &RedisStore{client: client, available: true}
}

func (r *RedisStore) Client() *redis.Client {
	return r.client
}

func (r *RedisStore) IsAvailable() bool {
	return r.available && r.client != nil
}
