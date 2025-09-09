package grpc

import (
	"context"
	"time"

	pb "github.com/putram11/sequential-id-counter-service/api/proto"
	"github.com/putram11/sequential-id-counter-service/internal/models"
	"github.com/putram11/sequential-id-counter-service/internal/service"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Server implements the SequentialIDServiceServer interface
type Server struct {
	pb.UnimplementedSequentialIDServiceServer
	sequentialIDService *service.SequentialIDService
	logger              *logrus.Logger
}

// NewServer creates a new gRPC server instance
func NewServer(sequentialIDService *service.SequentialIDService, logger *logrus.Logger) *Server {
	return &Server{
		sequentialIDService: sequentialIDService,
		logger:              logger,
	}
}

// GetNext gets the next sequential ID for a prefix
func (s *Server) GetNext(ctx context.Context, req *pb.GetNextRequest) (*pb.GetNextResponse, error) {
	if req.Prefix == "" {
		return nil, status.Error(codes.InvalidArgument, "prefix is required")
	}

	result, err := s.sequentialIDService.GetNext(ctx, req.Prefix, req.ClientId, req.CorrelationId)
	if err != nil {
		s.logger.WithError(err).WithFields(logrus.Fields{
			"prefix":         req.Prefix,
			"client_id":      req.ClientId,
			"correlation_id": req.CorrelationId,
		}).Error("Failed to get next sequential ID")

		return nil, status.Error(codes.Internal, "failed to generate sequential ID")
	}

	return &pb.GetNextResponse{
		FullNumber:  result.FullNumber,
		Prefix:      result.Prefix,
		Counter:     result.Counter,
		GeneratedAt: result.GeneratedAt.Format(time.RFC3339),
		MessageId:   result.MessageID,
	}, nil
}

// GetNextBatch gets a batch of sequential IDs
func (s *Server) GetNextBatch(ctx context.Context, req *pb.GetNextBatchRequest) (*pb.GetNextBatchResponse, error) {
	if req.Prefix == "" {
		return nil, status.Error(codes.InvalidArgument, "prefix is required")
	}

	if req.Count <= 0 || req.Count > 1000 {
		return nil, status.Error(codes.InvalidArgument, "count must be between 1 and 1000")
	}

	batchReq := &models.BatchRequest{
		Prefix:        req.Prefix,
		Count:         int(req.Count),
		ClientID:      req.ClientId,
		CorrelationID: req.CorrelationId,
	}

	result, err := s.sequentialIDService.GetNextBatch(ctx, batchReq)
	if err != nil {
		s.logger.WithError(err).WithFields(logrus.Fields{
			"prefix":         req.Prefix,
			"count":          req.Count,
			"client_id":      req.ClientId,
			"correlation_id": req.CorrelationId,
		}).Error("Failed to get batch of sequential IDs")

		return nil, status.Error(codes.Internal, "failed to generate batch of sequential IDs")
	}

	return &pb.GetNextBatchResponse{
		FullNumbers:  extractFullNumbers(result.IDs),
		Prefix:       req.Prefix,
		StartCounter: result.IDs[0].Counter,
		EndCounter:   result.IDs[len(result.IDs)-1].Counter,
		Count:        int32(result.Count),
		GeneratedAt:  result.GeneratedAt.Format(time.RFC3339),
		BatchId:      result.BatchID,
	}, nil
}

// extractFullNumbers extracts full numbers from SequentialID slice
func extractFullNumbers(ids []models.SequentialID) []string {
	fullNumbers := make([]string, len(ids))
	for i, id := range ids {
		fullNumbers[i] = id.FullNumber
	}
	return fullNumbers
}

// ResetCounter resets the counter for a prefix
func (s *Server) ResetCounter(ctx context.Context, req *pb.ResetCounterRequest) (*pb.ResetCounterResponse, error) {
	if req.Prefix == "" {
		return nil, status.Error(codes.InvalidArgument, "prefix is required")
	}

	if req.NewValue < 0 {
		return nil, status.Error(codes.InvalidArgument, "new_value must be non-negative")
	}

	resetReq := &models.ResetRequest{
		SetTo:     req.NewValue,
		Reason:    req.Reason,
		AdminUser: req.ClientId,
		Force:     false,
	}

	result, err := s.sequentialIDService.ResetCounter(ctx, req.Prefix, resetReq)
	if err != nil {
		s.logger.WithError(err).WithFields(logrus.Fields{
			"prefix":         req.Prefix,
			"new_value":      req.NewValue,
			"reason":         req.Reason,
			"client_id":      req.ClientId,
			"correlation_id": req.CorrelationId,
		}).Error("Failed to reset counter")

		return nil, status.Error(codes.Internal, "failed to reset counter")
	}

	return &pb.ResetCounterResponse{
		Success:  result.Success,
		Message:  result.Message,
		OldValue: result.OldValue,
		NewValue: result.NewValue,
	}, nil
}

// GetStatus gets the status of a counter (simplified for compatibility)
func (s *Server) GetStatus(ctx context.Context, req *pb.GetStatusRequest) (*pb.GetStatusResponse, error) {
	if req.Prefix == "" {
		return nil, status.Error(codes.InvalidArgument, "prefix is required")
	}

	statusResult, err := s.sequentialIDService.GetStatus(ctx, req.Prefix)
	if err != nil {
		s.logger.WithError(err).WithField("prefix", req.Prefix).Error("Failed to get status")
		return nil, status.Error(codes.Internal, "failed to get counter status")
	}

	return &pb.GetStatusResponse{
		Prefix:         statusResult.Prefix,
		CurrentCounter: statusResult.CurrentCounter,
		IsActive:       true,                            // Default value since not in CounterStatus
		LastGenerated:  time.Now().Format(time.RFC3339), // Default since not in CounterStatus
		TotalGenerated: statusResult.LastAuditCounter,
		Config:         nil, // Will be nil since CounterStatus doesn't include config
	}, nil
}

// Health performs a health check
func (s *Server) Health(ctx context.Context, req *pb.HealthRequest) (*pb.HealthResponse, error) {
	healthStatus := s.sequentialIDService.HealthCheck(ctx)

	statusVal := pb.HealthResponse_SERVING
	message := "Service is healthy"

	if !healthStatus.Healthy {
		statusVal = pb.HealthResponse_NOT_SERVING
		message = "Service is unhealthy"
	}

	return &pb.HealthResponse{
		Status:  statusVal,
		Message: message,
		Details: healthStatus.Components,
	}, nil
}

// GetConfig gets the configuration for a prefix
func (s *Server) GetConfig(ctx context.Context, req *pb.GetConfigRequest) (*pb.GetConfigResponse, error) {
	if req.Prefix == "" {
		return nil, status.Error(codes.InvalidArgument, "prefix is required")
	}

	config, err := s.sequentialIDService.GetConfig(ctx, req.Prefix)
	if err != nil {
		s.logger.WithError(err).WithField("prefix", req.Prefix).Error("Failed to get config")
		return nil, status.Error(codes.Internal, "failed to get configuration")
	}

	if config == nil {
		return &pb.GetConfigResponse{
			Found: false,
		}, nil
	}

	return &pb.GetConfigResponse{
		Config: &pb.ConfigInfo{
			Prefix:       config.Prefix,
			Format:       config.FormatTemplate,
			Padding:      int32(config.PaddingLength),
			Separator:    "",               // Not in model
			InitialValue: 0,                // Not in model
			MaxValue:     0,                // Not in model
			IsActive:     true,             // Not in model
			Description:  config.ResetRule, // Using reset rule as description
		},
		Found: true,
	}, nil
}

// UpdateConfig updates the configuration for a prefix (simplified implementation)
func (s *Server) UpdateConfig(ctx context.Context, req *pb.UpdateConfigRequest) (*pb.UpdateConfigResponse, error) {
	if req.Config == nil {
		return nil, status.Error(codes.InvalidArgument, "config is required")
	}

	if req.Config.Prefix == "" {
		return nil, status.Error(codes.InvalidArgument, "prefix is required")
	}

	updateReq := &models.ConfigUpdateRequest{
		AdminUser:         req.ClientId,
		CreateIfNotExists: true,
	}

	if req.Config.Format != "" {
		updateReq.FormatTemplate = &req.Config.Format
	}
	if req.Config.Padding > 0 {
		padding := int(req.Config.Padding)
		updateReq.PaddingLength = &padding
	}

	err := s.sequentialIDService.UpdateConfig(ctx, req.Config.Prefix, updateReq)
	if err != nil {
		s.logger.WithError(err).WithFields(logrus.Fields{
			"prefix":         req.Config.Prefix,
			"client_id":      req.ClientId,
			"correlation_id": req.CorrelationId,
		}).Error("Failed to update config")

		return nil, status.Error(codes.Internal, "failed to update configuration")
	}

	return &pb.UpdateConfigResponse{
		Success: true,
		Message: "Configuration updated successfully",
		Config:  req.Config,
	}, nil
}
