package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"loglens/internal/db"

	"github.com/redis/go-redis/v9"
)

var ErrAPIKeyRevoked = errors.New("api key revoked")

type APIKeyRecord struct {
	ID        string `json:"id"`
	ServiceID string `json:"service_id"`
	OrgID     string `json:"org_id"`
	Prefix    string `json:"prefix"`
	KeyHash   string `json:"key_hash"`
	Revoked   bool   `json:"revoked"`
}

type APIKeyLookup interface {
	GetActiveAPIKeyByPrefix(ctx context.Context, prefix string) (*APIKeyRecord, error)
}

type APIKeyCache struct {
	redis            *db.RedisStore
	lookup           APIKeyLookup
	prefixTTL        time.Duration
	validationTTL    time.Duration
}

func NewAPIKeyCache(redis *db.RedisStore, lookup APIKeyLookup) *APIKeyCache {
	return &APIKeyCache{
		redis:         redis,
		lookup:        lookup,
		prefixTTL:     durationFromEnv("API_KEY_CACHE_TTL_SECONDS", 300),
		validationTTL: durationFromEnv("API_KEY_VALIDATION_CACHE_TTL_SECONDS", 120),
	}
}

type ResolvedAPIKey struct {
	ID        string
	ServiceID string
	OrgID     string
}

func (c *APIKeyCache) Resolve(ctx context.Context, rawKey string) (*ResolvedAPIKey, error) {
	prefix, err := ExtractAPIKeyPrefix(rawKey)
	if err != nil {
		return nil, err
	}

	if c.redis != nil && c.redis.IsAvailable() {
		if revoked, err := c.redis.Client().Exists(ctx, revokedKey(prefix)).Result(); err == nil && revoked > 0 {
			return nil, ErrAPIKeyRevoked
		}

		validatedKey := validationKey(rawKey)
		if cached, err := c.redis.Client().Get(ctx, validatedKey).Result(); err == nil {
			var resolved ResolvedAPIKey
			if json.Unmarshal([]byte(cached), &resolved) == nil {
				return &resolved, nil
			}
		}
	}

	record, err := c.loadRecord(ctx, prefix)
	if err != nil {
		return nil, err
	}
	if record.Revoked {
		return nil, ErrAPIKeyRevoked
	}

	if err := ValidateAPIKey(rawKey, record.KeyHash); err != nil {
		return nil, err
	}

	resolved := &ResolvedAPIKey{
		ID:        record.ID,
		ServiceID: record.ServiceID,
		OrgID:     record.OrgID,
	}

	if c.redis != nil && c.redis.IsAvailable() {
		payload, _ := json.Marshal(resolved)
		_ = c.redis.Client().Set(ctx, validationKey(rawKey), payload, c.validationTTL).Err()
	}

	return resolved, nil
}

func (c *APIKeyCache) Invalidate(ctx context.Context, prefix string) error {
	if c.redis == nil || !c.redis.IsAvailable() {
		return nil
	}

	pipe := c.redis.Client().Pipeline()
	pipe.Del(ctx, prefixKey(prefix))
	pipe.Set(ctx, revokedKey(prefix), "1", time.Hour)
	_, err := pipe.Exec(ctx)
	return err
}

func (c *APIKeyCache) loadRecord(ctx context.Context, prefix string) (*APIKeyRecord, error) {
	if c.redis != nil && c.redis.IsAvailable() {
		cached, err := c.redis.Client().Get(ctx, prefixKey(prefix)).Result()
		if err == nil {
			var record APIKeyRecord
			if json.Unmarshal([]byte(cached), &record) == nil {
				return &record, nil
			}
		} else if !errors.Is(err, redis.Nil) {
			return nil, err
		}
	}

	record, err := c.lookup.GetActiveAPIKeyByPrefix(ctx, prefix)
	if err != nil {
		return nil, err
	}

	if c.redis != nil && c.redis.IsAvailable() {
		payload, _ := json.Marshal(record)
		_ = c.redis.Client().Set(ctx, prefixKey(prefix), payload, c.prefixTTL).Err()
	}

	return record, nil
}

func prefixKey(prefix string) string {
	return fmt.Sprintf("apikey:prefix:%s", prefix)
}

func revokedKey(prefix string) string {
	return fmt.Sprintf("apikey:revoked:%s", prefix)
}

func validationKey(rawKey string) string {
	sum := sha256.Sum256([]byte(rawKey))
	return fmt.Sprintf("apikey:validated:%s", hex.EncodeToString(sum[:]))
}

func durationFromEnv(key string, defaultSeconds int) time.Duration {
	if raw := os.Getenv(key); raw != "" {
		if seconds, err := strconv.Atoi(raw); err == nil && seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
	}
	return time.Duration(defaultSeconds) * time.Second
}
