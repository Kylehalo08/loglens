package main

import (
	"context"
	"log"
	"os"

	"loglens/internal/audit"
	"loglens/internal/auth"
	"loglens/internal/db"
	"loglens/internal/ingest"
	"loglens/internal/middleware"
	"loglens/internal/org"
	appsvc "loglens/internal/service"
	"loglens/internal/stream"
	"loglens/internal/telemetry"
	"loglens/internal/user"

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

	tokenService, err := auth.NewJWTService()
	if err != nil {
		log.Fatalf("failed to initialize jwt service: %v", err)
	}

	repo := user.NewPostgresRepository(pool)
	cache := user.NewRedisRefreshCache(redisStore)

	userService, err := user.NewService(repo, tokenService, cache)
	if err != nil {
		log.Fatalf("failed to initialize user service: %v", err)
	}

	userHandler := user.NewHandler(userService)

	orgRepo := org.NewPostgresRepository(pool)
	orgCache := org.NewRedisInviteCache(redisStore)
	orgService := org.NewService(orgRepo, tokenService, orgCache)
	orgHandler := org.NewHandler(orgService)

	auditWriter := audit.NewPostgresWriter(pool)
	svcRepo := appsvc.NewPostgresRepository(pool)
	keyLookup := ingest.NewKeyLookup(svcRepo)
	keyCache := auth.NewAPIKeyCache(redisStore, keyLookup)
	svcService := appsvc.NewService(svcRepo, auditWriter, orgService, keyCache)
	svcHandler := appsvc.NewHandler(svcService)

	logRepo := telemetry.NewRepository(pool)
	logHandler := telemetry.NewHandler(logRepo)
	streamHandler := stream.NewHandler(redisStore, svcService)

	e := echo.New()
	e.HideBanner = true
	e.Use(echomiddleware.Recover())
	e.Use(echomiddleware.Logger())

	e.GET("/health", logHandler.Health)

	authGroup := e.Group("/auth")
	authGroup.POST("/register", userHandler.Register)
	authGroup.POST("/login", userHandler.Login)
	authGroup.POST("/refresh", userHandler.Refresh)
	authGroup.POST("/logout", userHandler.Logout, middleware.RequireAuth(tokenService))

	orgsGroup := e.Group("/orgs", middleware.RequireAuth(tokenService))
	orgsGroup.POST("", orgHandler.CreateOrganization)
	orgsGroup.GET("", orgHandler.ListMyOrgs)
	orgsGroup.POST("/join/token", orgHandler.JoinViaToken)
	orgsGroup.POST("/join/code", orgHandler.JoinViaCode)
	orgsGroup.GET("/:id", orgHandler.GetOrganization)
	orgsGroup.POST("/:id/invites", orgHandler.SendEmailInvite, org.RequireOrgAdmin(orgService))
	orgsGroup.POST("/:id/invite-codes", orgHandler.GenerateInviteCode, org.RequireOrgAdmin(orgService))

	servicesGroup := orgsGroup.Group("/:id/services", appsvc.RequireOrgMember(orgService))
	servicesGroup.GET("", svcHandler.ListServices)
	servicesGroup.GET("/:serviceId", svcHandler.GetService)
	servicesGroup.GET("/:serviceId/logs/:logId", logHandler.GetLog)
	servicesGroup.GET("/:serviceId/logs/stream", streamHandler.StreamServiceLogs)

	servicesWriteGroup := servicesGroup.Group("", appsvc.RequireOrgDeveloper(orgService))
	servicesWriteGroup.POST("", svcHandler.CreateService)
	servicesWriteGroup.PATCH("/:serviceId", svcHandler.UpdateService)
	servicesWriteGroup.DELETE("/:serviceId", svcHandler.DeleteService)
	servicesWriteGroup.POST("/:serviceId/api-keys", svcHandler.GenerateAPIKey)
	servicesWriteGroup.GET("/:serviceId/api-keys", svcHandler.ListAPIKeys)
	servicesWriteGroup.DELETE("/:serviceId/api-keys/:keyId", svcHandler.RevokeAPIKey)
	servicesWriteGroup.POST("/:serviceId/api-keys/:keyId/rotate", svcHandler.RotateAPIKey)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("starting server on :%s", port)
	if err := e.Start(":" + port); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}
