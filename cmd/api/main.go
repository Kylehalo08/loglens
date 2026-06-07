package main

import (
	"context"
	"log"
	"os"

	"loglens/internal/auth"
	"loglens/internal/db"
	"loglens/internal/middleware"
	"loglens/internal/org"
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

	service, err := user.NewService(repo, tokenService, cache)
	if err != nil {
		log.Fatalf("failed to initialize user service: %v", err)
	}

	userHandler := user.NewHandler(service)

	orgRepo := org.NewPostgresRepository(pool)
	orgCache := org.NewRedisInviteCache(redisStore)
	orgService := org.NewService(orgRepo, tokenService, orgCache)
	orgHandler := org.NewHandler(orgService)

	e := echo.New()
	e.HideBanner = true
	e.Use(echomiddleware.Recover())
	e.Use(echomiddleware.Logger())

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

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("starting server on :%s", port)
	if err := e.Start(":" + port); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}
