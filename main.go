package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/SAP-F-2025/assessment-service/internal/config"
	"github.com/SAP-F-2025/assessment-service/internal/handlers"
	"github.com/SAP-F-2025/assessment-service/internal/repositories/casdoor"
	"github.com/SAP-F-2025/assessment-service/internal/repositories/postgres"
	"github.com/SAP-F-2025/assessment-service/internal/services"
	"github.com/SAP-F-2025/assessment-service/internal/utils"
	"github.com/SAP-F-2025/assessment-service/internal/validator"
	"github.com/SAP-F-2025/assessment-service/pkg"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize logger
	slogLogger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	}))
	logger := utils.NewSlogLogger(slogLogger)

	// Initialize database
	db, err := pkg.InitDatabase(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Initialize Redis (if configured)
	var redisClient *redis.Client
	if cfg.RedisURL != "" {
		redisClient, err = pkg.NewRedisClient(cfg)
		if err != nil {
			log.Printf("Warning: Failed to initialize Redis: %v", err)
		}
	}

	// Initialize repositories
	repoConfig := postgres.RepositoryConfig{
		DB:          db,
		RedisClient: redisClient,
		CasdoorConfig: casdoor.CasdoorConfig{
			Endpoint:         cfg.Casdoor.Endpoint,
			ClientID:         cfg.Casdoor.ClientID,
			ClientSecret:     cfg.Casdoor.ClientSecret,
			Certificate:      cfg.Casdoor.Cert,
			OrganizationName: cfg.Casdoor.Organization,
			ApplicationName:  cfg.Casdoor.Application,
		},
	}
	repoManager := postgres.NewRepositoryManager(repoConfig)
	if err := repoManager.Initialize(); err != nil {
		log.Fatalf("Failed to initialize repositories: %v", err)
	}

	// Initialize validator
	validator := validator.New()

	// Initialize services
	serviceManager := services.NewDefaultServiceManager(db, repoManager.GetRepository(), slogLogger, validator)
	if err := serviceManager.Initialize(context.Background()); err != nil {
		log.Fatalf("Failed to initialize services: %v", err)
	}

	// Initialize handlers
	handlerManager := handlers.NewHandlerManager(serviceManager, validator, logger, cfg.Casdoor, repoManager.GetRepository().User())

	// Setup Gin router
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// Setup middleware
	handlers.SetupMiddleware(router, logger)

	// Note: Authentication middleware is now applied per route group in SetupRoutes

	// Setup routes
	handlerManager.SetupRoutes(router)

	// Create HTTP server
	server := &http.Server{
		Addr:    fmt.Sprintf(":%s", cfg.Port),
		Handler: router,
	}

	// Start server in a goroutine
	go func() {
		logger.Info("Starting server", "port", cfg.Port, "environment", cfg.Environment)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown HTTP server
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	// Shutdown services
	if err := serviceManager.Shutdown(ctx); err != nil {
		log.Printf("Failed to shutdown services: %v", err)
	}

	// Close database connection
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.Close()
	}

	// Close Redis connection
	if redisClient != nil {
		redisClient.Close()
	}

	logger.Info("Server exited")
}
