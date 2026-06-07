package org

import (
	"context"
	"encoding/json"
	"time"

	"loglens/internal/db"
)

const inviteTokenKeyPref = "invite:"

type InviteTokenCache interface {
	IsAvailable() bool
	Set(ctx context.Context, token string, payload InviteTokenPayload, ttl time.Duration) error
	Get(ctx context.Context, token string) (*InviteTokenPayload, error)
	Delete(ctx context.Context, token string) error
}

type RedisInviteCache struct {
	store *db.RedisStore
}

func NewRedisInviteCache(store *db.RedisStore) *RedisInviteCache {
	return &RedisInviteCache{store: store}
}

func (c *RedisInviteCache) IsAvailable() bool {
	return c.store != nil && c.store.IsAvailable()
}

func (c *RedisInviteCache) Set(ctx context.Context, token string, payload InviteTokenPayload, ttl time.Duration) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return c.store.Client().Set(ctx, inviteTokenKeyPref+token, data, ttl).Err()
}

func (c *RedisInviteCache) Get(ctx context.Context, token string) (*InviteTokenPayload, error) {
	data, err := c.store.Client().Get(ctx, inviteTokenKeyPref+token).Result()
	if err != nil {
		return nil, err
	}

	var payload InviteTokenPayload
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		return nil, err
	}

	return &payload, nil
}

func (c *RedisInviteCache) Delete(ctx context.Context, token string) error {
	return c.store.Client().Del(ctx, inviteTokenKeyPref+token).Err()
}
