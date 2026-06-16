package main

import (
	"context"
	"log"
	"os"

	"loglens/internal/auth"
	"loglens/internal/db"
	"loglens/internal/ingest"
	"loglens/internal/middleware"
	"loglens/internal/ratelimit"
	appsvc "loglens/internal/service"
	"loglens/internal/telemetry"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("warning: .env file not found, using environment variables")
	}

	ctx := context.Background()

	pool, err := db.Connect(ctx)
	if err != nil {
		log.Fatalf("failed to initialize postgres: %v", err)
	}
	defer pool.Close()

	redisStore := db.ConnectRedis(ctx)
	rateLimitCfg := ratelimit.LoadConfig()
	rateLimiter := ratelimit.NewLimiter(redisStore, rateLimitCfg)

	svcRepo := appsvc.NewPostgresRepository(pool)
	keyLookup := ingest.NewKeyLookup(svcRepo)
	keyCache := auth.NewAPIKeyCache(redisStore, keyLookup)

	producer := telemetry.NewProducer()
	defer producer.Close()

	handler := ingest.NewHandler(keyCache, producer, rateLimiter, rateLimitCfg)

	e := echo.New()
	e.HideBanner = true
	e.Use(echomiddleware.Recover())
	e.Use(echomiddleware.Logger())
	e.Use(middleware.CORS())
	e.Use(middleware.RateLimitByIP(rateLimiter, "ingest", rateLimitCfg.IngestRequestsPerIPMinute))

	e.GET("/health", handler.Health)
	e.POST("/v1/logs", handler.IngestLog)

	port := os.Getenv("INGESTOR_PORT")
	if port == "" {
		port = "8081"
	}

	log.Printf("starting ingestor on :%s", port)
	if err := e.Start(":" + port); err != nil {
		log.Fatalf("ingestor stopped: %v", err)
	}
}
