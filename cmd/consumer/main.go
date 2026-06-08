package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"loglens/internal/db"
	"loglens/internal/telemetry"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("warning: .env file not found, using environment variables")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx)
	if err != nil {
		log.Fatalf("failed to initialize postgres: %v", err)
	}
	defer pool.Close()

	redisStore := db.ConnectRedis(ctx)

	repo := telemetry.NewRepository(pool)
	publisher := telemetry.NewPublisher(redisStore)
	consumer := telemetry.NewConsumer()
	defer consumer.Close()

	go runRetention(ctx, repo)

	log.Println("starting log consumer")
	for {
		select {
		case <-ctx.Done():
			log.Println("consumer shutting down")
			return
		default:
		}

		entry, msg, err := consumer.Fetch(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("kafka fetch error: %v", err)
			time.Sleep(time.Second)
			continue
		}

		if entry.ID == "" {
			log.Printf("skipping malformed kafka message at offset %d", msg.Offset)
			_ = consumer.Commit(ctx, msg)
			continue
		}

		if err := repo.InsertLogs(ctx, []telemetry.LogEntry{entry}); err != nil {
			log.Printf("postgres insert error: %v", err)
			time.Sleep(backoffDelay())
			continue
		}

		if err := consumer.Commit(ctx, msg); err != nil {
			log.Printf("kafka commit error: %v", err)
		}

		if err := publisher.PublishLog(ctx, entry); err != nil {
			log.Printf("redis publish error: %v", err)
		}
	}
}

func runRetention(ctx context.Context, repo *telemetry.Repository) {
	interval := retentionInterval()
	hours := retentionHours()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Printf("retention worker started: every %s delete logs older than %d hour(s)", interval, hours)

	runOnce(ctx, repo, hours)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runOnce(ctx, repo, hours)
		}
	}
}

func runOnce(ctx context.Context, repo *telemetry.Repository, hours int) {
	cutoff := time.Now().UTC().Add(-time.Duration(hours) * time.Hour)
	deleted, err := repo.DeleteOlderThan(ctx, cutoff)
	if err != nil {
		log.Printf("retention delete error: %v", err)
		return
	}
	if deleted > 0 {
		log.Printf("retention deleted %d log(s) older than %s", deleted, cutoff.Format(time.RFC3339))
	}
}

func retentionHours() int {
	if raw := os.Getenv("LOG_RETENTION_HOURS"); raw != "" {
		if hours, err := strconv.Atoi(raw); err == nil && hours > 0 {
			return hours
		}
	}
	return 2
}

func retentionInterval() time.Duration {
	if raw := os.Getenv("LOG_RETENTION_INTERVAL_MINUTES"); raw != "" {
		if minutes, err := strconv.Atoi(raw); err == nil && minutes > 0 {
			return time.Duration(minutes) * time.Minute
		}
	}
	return 15 * time.Minute
}

func backoffDelay() time.Duration {
	return 2 * time.Second
}
