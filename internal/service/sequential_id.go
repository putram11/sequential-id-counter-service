package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/putram11/sequential-id-counter-service/internal/models"
	"github.com/putram11/sequential-id-counter-service/internal/repository"
	"github.com/sirupsen/logrus"
)

// SequentialIDService provides sequential ID generation functionality
type SequentialIDService struct {
	redisRepo    *repository.RedisRepository
	dbRepo       *repository.PostgresRepository
	rabbitRepo   *repository.RabbitMQRepository
	logger       *logrus.Logger
}

// NewSequentialIDService creates a new sequential ID service
func NewSequentialIDService(
	redisRepo *repository.RedisRepository,
	dbRepo *repository.PostgresRepository,
	rabbitRepo *repository.RabbitMQRepository,
	logger *logrus.Logger,
) *SequentialIDService {
	return &SequentialIDService{
		redisRepo:  redisRepo,
		dbRepo:     dbRepo,
		rabbitRepo: rabbitRepo,
		logger:     logger,
	}
}

// GetNext generates the next sequential ID for a given prefix
func (s *SequentialIDService) GetNext(ctx context.Context, prefix, clientID, generatedBy string) (*models.SequentialID, error) {
	// Get prefix configuration
	config, err := s.dbRepo.GetPrefixConfig(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to get prefix config: %w", err)
	}
	if config == nil {
		return nil, fmt.Errorf("prefix %s not configured", prefix)
	}
	
	// Increment counter in Redis (atomic operation)
	counter, err := s.redisRepo.IncrementCounter(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to increment counter: %w", err)
	}
	
	// Format the ID
	fullNumber := s.formatID(config, counter)
	
	// Create sequential ID
	seqID := &models.SequentialID{
		Prefix:      prefix,
		Counter:     counter,
		FullNumber:  fullNumber,
		GeneratedBy: generatedBy,
		ClientID:    clientID,
		MessageID:   uuid.New().String(),
		GeneratedAt: time.Now(),
	}
	
	// Publish event for audit logging (async)
	event := &models.Event{
		MessageID:     seqID.MessageID,
		Prefix:        seqID.Prefix,
		Counter:       seqID.Counter,
		FullNumber:    seqID.FullNumber,
		GeneratedBy:   seqID.GeneratedBy,
		ClientID:      seqID.ClientID,
		GeneratedAt:   seqID.GeneratedAt,
		RetryCount:    0,
	}
	
	if err := s.rabbitRepo.PublishEvent(ctx, event); err != nil {
		// Log error but don't fail the request - the ID was already generated
		s.logger.WithError(err).WithFields(logrus.Fields{
			"prefix":      prefix,
			"counter":     counter,
			"full_number": fullNumber,
			"message_id":  seqID.MessageID,
		}).Error("Failed to publish audit event")
	}
	
	s.logger.WithFields(logrus.Fields{
		"prefix":       prefix,
		"counter":      counter,
		"full_number":  fullNumber,
		"client_id":    clientID,
		"generated_by": generatedBy,
	}).Info("Generated sequential ID")
	
	return seqID, nil
}

// GetNextBatch generates multiple sequential IDs in a single operation
func (s *SequentialIDService) GetNextBatch(ctx context.Context, req *models.BatchRequest) (*models.BatchResponse, error) {
	if req.Count <= 0 || req.Count > 1000 {
		return nil, fmt.Errorf("invalid count: must be between 1 and 1000")
	}
	
	// Get prefix configuration
	config, err := s.dbRepo.GetPrefixConfig(ctx, req.Prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to get prefix config: %w", err)
	}
	if config == nil {
		return nil, fmt.Errorf("prefix %s not configured", req.Prefix)
	}
	
	// Increment counter by batch size (atomic operation)
	endCounter, err := s.redisRepo.IncrementCounterBy(ctx, req.Prefix, int64(req.Count))
	if err != nil {
		return nil, fmt.Errorf("failed to increment counter: %w", err)
	}
	
	startCounter := endCounter - int64(req.Count) + 1
	batchID := uuid.New().String()
	generatedAt := time.Now()
	
	// Generate all IDs in the batch
	ids := make([]models.SequentialID, req.Count)
	for i := 0; i < req.Count; i++ {
		counter := startCounter + int64(i)
		fullNumber := s.formatID(config, counter)
		
		ids[i] = models.SequentialID{
			Prefix:      req.Prefix,
			Counter:     counter,
			FullNumber:  fullNumber,
			GeneratedBy: req.GeneratedBy,
			ClientID:    req.ClientID,
			MessageID:   uuid.New().String(),
			GeneratedAt: generatedAt,
		}
		
		// Publish individual events for audit
		event := &models.Event{
			MessageID:     ids[i].MessageID,
			Prefix:        ids[i].Prefix,
			Counter:       ids[i].Counter,
			FullNumber:    ids[i].FullNumber,
			GeneratedBy:   ids[i].GeneratedBy,
			ClientID:      ids[i].ClientID,
			CorrelationID: req.CorrelationID,
			GeneratedAt:   ids[i].GeneratedAt,
			BatchID:       batchID,
			RetryCount:    0,
		}
		
		if err := s.rabbitRepo.PublishEvent(ctx, event); err != nil {
			s.logger.WithError(err).WithFields(logrus.Fields{
				"prefix":      req.Prefix,
				"counter":     counter,
				"batch_id":    batchID,
				"message_id":  ids[i].MessageID,
			}).Error("Failed to publish batch audit event")
		}
	}
	
	response := &models.BatchResponse{
		IDs:         ids,
		BatchID:     batchID,
		Count:       req.Count,
		GeneratedAt: generatedAt,
	}
	
	s.logger.WithFields(logrus.Fields{
		"prefix":    req.Prefix,
		"count":     req.Count,
		"batch_id":  batchID,
		"start":     startCounter,
		"end":       endCounter,
	}).Info("Generated batch of sequential IDs")
	
	return response, nil
}

// GetStatus returns the current status of a counter
func (s *SequentialIDService) GetStatus(ctx context.Context, prefix string) (*models.CounterStatus, error) {
	// Get current counter from Redis
	currentCounter, err := s.redisRepo.GetCounter(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to get current counter: %w", err)
	}
	
	// Get last audit counter from database
	lastAuditCounter, err := s.dbRepo.GetMaxCounter(ctx, prefix)
	if err != nil {
		// Don't fail if we can't get audit counter
		s.logger.WithError(err).Warn("Failed to get last audit counter")
		lastAuditCounter = 0
	}
	
	// Check component health
	redisHealthy := true
	if err := s.redisRepo.Ping(ctx); err != nil {
		redisHealthy = false
	}
	
	queueHealthy := true
	if err := s.rabbitRepo.Ping(ctx); err != nil {
		queueHealthy = false
	}
	
	dbHealthy := true
	if err := s.dbRepo.Ping(ctx); err != nil {
		dbHealthy = false
	}
	
	status := &models.CounterStatus{
		Prefix:           prefix,
		CurrentCounter:   currentCounter,
		NextCounter:      currentCounter + 1,
		RedisHealthy:     redisHealthy,
		QueueHealthy:     queueHealthy,
		DatabaseHealthy:  dbHealthy,
		LastAuditCounter: lastAuditCounter,
	}
	
	return status, nil
}

// ResetCounter resets a counter to a specific value (admin operation)
func (s *SequentialIDService) ResetCounter(ctx context.Context, prefix string, req *models.ResetRequest) (*models.ResetResponse, error) {
	// Validate request
	if req.SetTo < 0 {
		return nil, fmt.Errorf("counter value cannot be negative")
	}
	
	if req.Reason == "" {
		return nil, fmt.Errorf("reason is required for counter reset")
	}
	
	if req.AdminUser == "" {
		return nil, fmt.Errorf("admin user is required for counter reset")
	}
	
	// Get current value
	currentValue, err := s.redisRepo.GetCounter(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to get current counter: %w", err)
	}
	
	// Check if reset is safe (unless forced)
	if !req.Force && req.SetTo <= currentValue {
		return nil, fmt.Errorf("new value %d is not greater than current value %d (use force=true to override)", req.SetTo, currentValue)
	}
	
	// Reset counter in Redis
	oldValue, err := s.redisRepo.ResetCounter(ctx, prefix, req.SetTo)
	if err != nil {
		return nil, fmt.Errorf("failed to reset counter: %w", err)
	}
	
	// Log the reset operation
	resetID := uuid.New().String()
	resetLog := &models.ResetLog{
		Prefix:    prefix,
		OldValue:  oldValue,
		NewValue:  req.SetTo,
		Reason:    req.Reason,
		AdminUser: req.AdminUser,
		ResetID:   resetID,
	}
	
	if err := s.dbRepo.InsertResetLog(ctx, resetLog); err != nil {
		s.logger.WithError(err).Error("Failed to log counter reset")
	}
	
	// Update checkpoint
	checkpoint := &models.Checkpoint{
		Prefix:            prefix,
		LastCounterSynced: req.SetTo,
		SyncedBy:          &req.AdminUser,
	}
	
	if err := s.dbRepo.UpdateCheckpoint(ctx, checkpoint); err != nil {
		s.logger.WithError(err).Error("Failed to update checkpoint")
	}
	
	s.logger.WithFields(logrus.Fields{
		"prefix":     prefix,
		"old_value":  oldValue,
		"new_value":  req.SetTo,
		"admin_user": req.AdminUser,
		"reason":     req.Reason,
		"reset_id":   resetID,
	}).Warn("Counter reset performed")
	
	return &models.ResetResponse{
		Success:  true,
		Message:  fmt.Sprintf("Counter reset from %d to %d", oldValue, req.SetTo),
		OldValue: oldValue,
		NewValue: req.SetTo,
		ResetID:  resetID,
	}, nil
}

// GetConfig retrieves configuration for a prefix
func (s *SequentialIDService) GetConfig(ctx context.Context, prefix string) (*models.PrefixConfig, error) {
	config, err := s.dbRepo.GetPrefixConfig(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to get prefix config: %w", err)
	}
	
	return config, nil
}

// UpdateConfig updates configuration for a prefix
func (s *SequentialIDService) UpdateConfig(ctx context.Context, prefix string, req *models.ConfigUpdateRequest) error {
	// Validate request
	if req.AdminUser == "" {
		return fmt.Errorf("admin user is required for config update")
	}
	
	// Check if prefix exists
	existing, err := s.dbRepo.GetPrefixConfig(ctx, prefix)
	if err != nil {
		return fmt.Errorf("failed to get existing config: %w", err)
	}
	
	if existing == nil && !req.CreateIfNotExists {
		return fmt.Errorf("prefix %s does not exist", prefix)
	}
	
	// Create new prefix if it doesn't exist
	if existing == nil {
		newConfig := &models.PrefixConfig{
			Prefix:         prefix,
			PaddingLength:  6,
			FormatTemplate: "%s%06d",
			ResetRule:      "never",
			CreatedBy:      &req.AdminUser,
		}
		
		// Apply updates
		if req.PaddingLength != nil {
			newConfig.PaddingLength = *req.PaddingLength
		}
		if req.FormatTemplate != nil {
			newConfig.FormatTemplate = *req.FormatTemplate
		}
		if req.ResetRule != nil {
			newConfig.ResetRule = *req.ResetRule
		}
		
		return s.dbRepo.CreatePrefixConfig(ctx, newConfig)
	}
	
	// Update existing config
	updates := make(map[string]interface{})
	if req.PaddingLength != nil {
		updates["padding_length"] = *req.PaddingLength
	}
	if req.FormatTemplate != nil {
		updates["format_template"] = *req.FormatTemplate
	}
	if req.ResetRule != nil {
		updates["reset_rule"] = *req.ResetRule
	}
	if req.AdminUser != "" {
		updates["updated_by"] = req.AdminUser
	}
	
	if len(updates) == 0 {
		return fmt.Errorf("no updates provided")
	}
	
	return s.dbRepo.UpdatePrefixConfig(ctx, prefix, updates)
}

// SyncCountersOnStartup syncs Redis counters with database values on service startup
func (s *SequentialIDService) SyncCountersOnStartup(ctx context.Context) error {
	s.logger.Info("Starting counter synchronization on startup")
	
	// Get all prefix configurations
	configs, err := s.dbRepo.GetAllPrefixConfigs(ctx)
	if err != nil {
		return fmt.Errorf("failed to get prefix configs: %w", err)
	}
	
	for _, config := range configs {
		// Get max counter from database
		maxCounter, err := s.dbRepo.GetMaxCounter(ctx, config.Prefix)
		if err != nil {
			s.logger.WithError(err).WithField("prefix", config.Prefix).Error("Failed to get max counter for prefix")
			continue
		}
		
		// Set Redis counter (only if greater than current value)
		currentRedisCounter, err := s.redisRepo.GetCounter(ctx, config.Prefix)
		if err != nil {
			s.logger.WithError(err).WithField("prefix", config.Prefix).Error("Failed to get Redis counter for prefix")
			continue
		}
		
		if maxCounter > currentRedisCounter {
			if err := s.redisRepo.SetCounter(ctx, config.Prefix, maxCounter); err != nil {
				s.logger.WithError(err).WithFields(logrus.Fields{
					"prefix":      config.Prefix,
					"max_counter": maxCounter,
				}).Error("Failed to sync Redis counter")
				continue
			}
			
			s.logger.WithFields(logrus.Fields{
				"prefix":         config.Prefix,
				"synced_counter": maxCounter,
				"redis_counter":  currentRedisCounter,
			}).Info("Synced Redis counter with database")
		}
		
		// Update checkpoint
		checkpoint := &models.Checkpoint{
			Prefix:            config.Prefix,
			LastCounterSynced: maxCounter,
			SyncedBy:          stringPtr("system"),
		}
		
		if err := s.dbRepo.UpdateCheckpoint(ctx, checkpoint); err != nil {
			s.logger.WithError(err).WithField("prefix", config.Prefix).Error("Failed to update checkpoint")
		}
	}
	
	s.logger.Info("Counter synchronization completed")
	return nil
}

// HealthCheck performs a comprehensive health check
func (s *SequentialIDService) HealthCheck(ctx context.Context) *models.HealthStatus {
	components := make(map[string]string)
	healthy := true
	
	// Check Redis
	if err := s.redisRepo.Ping(ctx); err != nil {
		components["redis"] = fmt.Sprintf("unhealthy: %v", err)
		healthy = false
	} else {
		components["redis"] = "healthy"
	}
	
	// Check Database
	if err := s.dbRepo.Ping(ctx); err != nil {
		components["database"] = fmt.Sprintf("unhealthy: %v", err)
		healthy = false
	} else {
		components["database"] = "healthy"
	}
	
	// Check RabbitMQ
	if err := s.rabbitRepo.Ping(ctx); err != nil {
		components["rabbitmq"] = fmt.Sprintf("unhealthy: %v", err)
		healthy = false
	} else {
		components["rabbitmq"] = "healthy"
	}
	
	return &models.HealthStatus{
		Healthy:    healthy,
		Components: components,
		Timestamp:  time.Now(),
	}
}

// formatID formats a counter value according to the prefix configuration
func (s *SequentialIDService) formatID(config *models.PrefixConfig, counter int64) string {
	template := config.FormatTemplate
	
	// Handle different template formats
	if strings.Contains(template, "%s") && strings.Contains(template, "%d") {
		// Template like "%s%06d" or "INV%d-%04d"
		if strings.Contains(template, "%06d") {
			return fmt.Sprintf(template, config.Prefix, counter)
		} else if strings.Contains(template, "%04d") {
			return fmt.Sprintf(template, time.Now().Year(), counter)
		} else {
			// Generic case
			return fmt.Sprintf(template, config.Prefix, counter)
		}
	} else if strings.Contains(template, "%d") {
		// Template like "INV%06d"
		return fmt.Sprintf(template, counter)
	} else {
		// Fallback to default format
		format := "%s%0" + strconv.Itoa(config.PaddingLength) + "d"
		return fmt.Sprintf(format, config.Prefix, counter)
	}
}

// Helper function to create string pointer
func stringPtr(s string) *string {
	return &s
}
