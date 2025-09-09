package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/putram11/sequential-id-counter-service/internal/api/grpc"
	"github.com/putram11/sequential-id-counter-service/internal/api/rest"
	"github.com/putram11/sequential-id-counter-service/internal/config"
	"github.com/putram11/sequential-id-counter-service/internal/repository"
	"github.com/putram11/sequential-id-counter-service/internal/service"
	"github.com/sirupsen/logrus"
	grpc_server "google.golang.org/grpc"
)

func main() {
	// Initialize logger
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Fatalf("Failed to load configuration: %v", err)
	}

	// Set log level
	if level, err := logrus.ParseLevel(cfg.LogLevel); err == nil {
		logger.SetLevel(level)
	}

	logger.Info("Starting Sequential ID Counter Service")

	// Initialize repositories
	redisRepo, err := repository.NewRedisRepository(cfg.Redis)
	if err != nil {
		logger.Fatalf("Failed to initialize Redis repository: %v", err)
	}
	defer redisRepo.Close()

	dbRepo, err := repository.NewPostgresRepository(cfg.Database)
	if err != nil {
		logger.Fatalf("Failed to initialize database repository: %v", err)
	}
	defer dbRepo.Close()

	rabbitRepo, err := repository.NewRabbitMQRepository(cfg.RabbitMQ)
	if err != nil {
		logger.Fatalf("Failed to initialize RabbitMQ repository: %v", err)
	}
	defer rabbitRepo.Close()

	// Initialize service
	seqService := service.NewSequentialIDService(
		redisRepo,
		dbRepo,
		rabbitRepo,
		logger,
	)

	// Sync Redis with database on startup
	if err := seqService.SyncCountersOnStartup(context.Background()); err != nil {
		logger.Errorf("Failed to sync counters on startup: %v", err)
		// Continue anyway - service can still work with Redis
	}

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start REST API server
	restHandler := rest.NewHandler(seqService, logger)
	restServer := &http.Server{
		Addr:    fmt.Sprintf(":%s", cfg.Port),
		Handler: setupGinRouter(restHandler),
	}

	go func() {
		logger.Infof("Starting REST API server on port %s", cfg.Port)
		if err := restServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Failed to start REST server: %v", err)
		}
	}()

	// Start gRPC server
	grpcHandler := grpc.NewHandler(seqService, logger)
	grpcServer := grpc_server.NewServer()
	grpcHandler.RegisterService(grpcServer)

	grpcListener, err := net.Listen("tcp", fmt.Sprintf(":%s", cfg.GRPCPort))
	if err != nil {
		logger.Fatalf("Failed to listen on gRPC port: %v", err)
	}

	go func() {
		logger.Infof("Starting gRPC server on port %s", cfg.GRPCPort)
		if err := grpcServer.Serve(grpcListener); err != nil {
			logger.Fatalf("Failed to start gRPC server: %v", err)
		}
	}()

	// Start health check server
	healthServer := &http.Server{
		Addr:    fmt.Sprintf(":%s", cfg.HealthPort),
		Handler: setupHealthRouter(seqService),
	}

	go func() {
		logger.Infof("Starting health check server on port %s", cfg.HealthPort)
		if err := healthServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Errorf("Health server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	logger.Info("Shutting down gracefully...")

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Shutdown REST server
	if err := restServer.Shutdown(shutdownCtx); err != nil {
		logger.Errorf("Failed to shutdown REST server: %v", err)
	}

	// Shutdown gRPC server
	grpcServer.GracefulStop()

	// Shutdown health server
	if err := healthServer.Shutdown(shutdownCtx); err != nil {
		logger.Errorf("Failed to shutdown health server: %v", err)
	}

	logger.Info("Server stopped")
}

func setupGinRouter(handler *rest.Handler) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	// Middleware
	router.Use(gin.Recovery())
	router.Use(gin.Logger())

	// API routes
	v1 := router.Group("/api/v1")
	{
		v1.GET("/next/:prefix", handler.GetNext)
		v1.GET("/status/:prefix", handler.GetStatus)
		v1.POST("/reset/:prefix", handler.ResetCounter)
		v1.GET("/config/:prefix", handler.GetConfig)
		v1.POST("/config/:prefix", handler.UpdateConfig)
	}

	return router
}

func setupHealthRouter(seqService *service.SequentialIDService) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	router.GET("/health", func(c *gin.Context) {
		health := seqService.HealthCheck(c.Request.Context())
		if health.Healthy {
			c.JSON(http.StatusOK, health)
		} else {
			c.JSON(http.StatusServiceUnavailable, health)
		}
	})

	router.GET("/health/live", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "alive"})
	})

	router.GET("/health/ready", func(c *gin.Context) {
		health := seqService.HealthCheck(c.Request.Context())
		if health.Healthy {
			c.JSON(http.StatusOK, gin.H{"status": "ready"})
		} else {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready"})
		}
	})

	return router
}
