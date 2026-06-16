package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"loglens/internal/db"
)

var ErrRateLimited = errors.New("rate limit exceeded")

type Limiter struct {
	store   *db.RedisStore
	enabled bool
}

func NewLimiter(store *db.RedisStore, cfg Config) *Limiter {
	enabled := cfg.Enabled && store != nil && store.IsAvailable()
	if cfg.Enabled && !enabled {
		log.Println("warning: rate limiting disabled: redis unavailable")
	}
	return &Limiter{store: store, enabled: enabled}
}

func (l *Limiter) Enabled() bool {
	return l.enabled
}

func (l *Limiter) Allow(ctx context.Context, key string, limit int, window time.Duration) error {
	if !l.enabled || limit <= 0 {
		return nil
	}

	pipe := l.store.Client().Pipeline()
	countCmd := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, window)
	if _, err := pipe.Exec(ctx); err != nil {
		return nil
	}

	if countCmd.Val() > int64(limit) {
		return ErrRateLimited
	}
	return nil
}

func (l *Limiter) AllowIngest(ctx context.Context, orgID string, cfg Config) error {
	if err := l.Allow(ctx, ingestHourKey(orgID), cfg.IngestPerOrgHour, time.Hour); err != nil {
		return err
	}
	return l.Allow(ctx, ingestDayKey(orgID), cfg.IngestPerOrgDay, 24*time.Hour)
}

func ingestHourKey(orgID string) string {
	bucket := time.Now().UTC().Format("2006010215")
	return fmt.Sprintf("rl:ingest:org:%s:hour:%s", orgID, bucket)
}

func ingestDayKey(orgID string) string {
	bucket := time.Now().UTC().Format("20060102")
	return fmt.Sprintf("rl:ingest:org:%s:day:%s", orgID, bucket)
}

func IPMinuteKey(prefix, ip string) string {
	bucket := time.Now().UTC().Format("200601021504")
	return fmt.Sprintf("rl:%s:ip:%s:min:%s", prefix, ip, bucket)
}

func UserMinuteKey(prefix, userID string) string {
	bucket := time.Now().UTC().Format("200601021504")
	return fmt.Sprintf("rl:%s:user:%s:min:%s", prefix, userID, bucket)
}
