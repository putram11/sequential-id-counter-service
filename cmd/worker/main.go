package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/putram11/sequential-id-counter-service/internal/config"
	"github.com/putram11/sequential-id-counter-service/internal/models"
	"github.com/putram11/sequential-id-counter-service/internal/repository"
	"github.com/sirupsen/logrus"
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

	logger.Info("Starting Sequential ID Worker Service")

	// Initialize repositories
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

	// Create worker
	worker := &Worker{
		dbRepo:     dbRepo,
		rabbitRepo: rabbitRepo,
		logger:     logger,
	}

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start worker
	go func() {
		if err := worker.Start(ctx); err != nil {
			logger.Fatalf("Worker failed: %v", err)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	logger.Info("Shutting down worker gracefully...")
	cancel()

	// Give worker time to finish processing current messages
	time.Sleep(5 * time.Second)
	logger.Info("Worker stopped")
}

// Worker processes events from RabbitMQ and inserts them into PostgreSQL
type Worker struct {
	dbRepo     *repository.PostgresRepository
	rabbitRepo *repository.RabbitMQRepository
	logger     *logrus.Logger
}

// Start begins processing messages from the queue
func (w *Worker) Start(ctx context.Context) error {
	w.logger.Info("Worker started, waiting for messages")

	// Define event handler
	handler := func(event *models.Event) error {
		return w.processEvent(ctx, event)
	}

	// Start consuming events
	return w.rabbitRepo.ConsumeEvents(ctx, handler)
}

// processEvent processes a single event and inserts it into the database
func (w *Worker) processEvent(ctx context.Context, event *models.Event) error {
	startTime := time.Now()

	// Create audit log entry
	auditLog := &models.AuditLog{
		Prefix:        event.Prefix,
		CounterValue:  event.Counter,
		FullNumber:    event.FullNumber,
		GeneratedBy:   &event.GeneratedBy,
		ClientID:      &event.ClientID,
		CorrelationID: &event.CorrelationID,
		MessageID:     event.MessageID,
		GeneratedAt:   event.GeneratedAt,
		PublishedAt:   &event.PublishedAt,
		BatchID:       &event.BatchID,
	}

	// Insert into database
	if err := w.dbRepo.InsertAuditLog(ctx, auditLog); err != nil {
		w.logger.WithError(err).WithFields(logrus.Fields{
			"message_id":  event.MessageID,
			"prefix":      event.Prefix,
			"counter":     event.Counter,
			"full_number": event.FullNumber,
			"retry_count": event.RetryCount,
		}).Error("Failed to insert audit log")
		return err
	}

	processingTime := time.Since(startTime)

	w.logger.WithFields(logrus.Fields{
		"message_id":      event.MessageID,
		"prefix":          event.Prefix,
		"counter":         event.Counter,
		"full_number":     event.FullNumber,
		"processing_time": processingTime.String(),
		"batch_id":        event.BatchID,
	}).Debug("Successfully processed audit event")

	return nil
}
