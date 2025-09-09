package grpc

import (
	"context"
	"strconv"
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
	proto.UnimplementedSequentialIDServiceServer
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
func (s *Server) GetNext(ctx context.Context, req *proto.GetNextRequest) (*proto.GetNextResponse, error) {
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

	return &proto.GetNextResponse{
		FullNumber:  result.FullNumber,
		Prefix:      result.Prefix,
		Counter:     result.Counter,
		GeneratedAt: result.GeneratedAt.Format(time.RFC3339),
		MessageId:   result.MessageID,
	}, nil
}

// GetNextBatch gets a batch of sequential IDs
func (s *Server) GetNextBatch(ctx context.Context, req *proto.GetNextBatchRequest) (*proto.GetNextBatchResponse, error) {
	if req.Prefix == "" {
		return nil, status.Error(codes.InvalidArgument, "prefix is required")
	}
	
	if req.Count <= 0 || req.Count > 1000 {
		return nil, status.Error(codes.InvalidArgument, "count must be between 1 and 1000")
	}

	result, err := s.sequentialIDService.GetNextBatch(ctx, req.Prefix, int(req.Count), req.ClientId, req.CorrelationId)
	if err != nil {
		s.logger.WithError(err).WithFields(logrus.Fields{
			"prefix":         req.Prefix,
			"count":          req.Count,
			"client_id":      req.ClientId,
			"correlation_id": req.CorrelationId,
		}).Error("Failed to get batch of sequential IDs")
		
		return nil, status.Error(codes.Internal, "failed to generate batch of sequential IDs")
	}

	return &proto.GetNextBatchResponse{
		FullNumbers:  result.FullNumbers,
		Prefix:       result.Prefix,
		StartCounter: result.StartCounter,
		EndCounter:   result.EndCounter,
		Count:        int32(result.Count),
		GeneratedAt:  result.GeneratedAt.Format(time.RFC3339),
		BatchId:      result.BatchID,
	}, nil
}

// ResetCounter resets the counter for a prefix
func (s *Server) ResetCounter(ctx context.Context, req *proto.ResetCounterRequest) (*proto.ResetCounterResponse, error) {
	if req.Prefix == "" {
		return nil, status.Error(codes.InvalidArgument, "prefix is required")
	}

	if req.NewValue < 0 {
		return nil, status.Error(codes.InvalidArgument, "new_value must be non-negative")
	}

	oldValue, err := s.sequentialIDService.ResetCounter(ctx, req.Prefix, req.NewValue, req.Reason, req.ClientId, req.CorrelationId)
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

	return &proto.ResetCounterResponse{
		Success:  true,
		Message:  "Counter reset successfully",
		OldValue: oldValue,
		NewValue: req.NewValue,
	}, nil
}

// GetStatus gets the status of a counter
func (s *Server) GetStatus(ctx context.Context, req *proto.GetStatusRequest) (*proto.GetStatusResponse, error) {
	if req.Prefix == "" {
		return nil, status.Error(codes.InvalidArgument, "prefix is required")
	}

	status, err := s.sequentialIDService.GetStatus(ctx, req.Prefix)
	if err != nil {
		s.logger.WithError(err).WithField("prefix", req.Prefix).Error("Failed to get status")
		return nil, status.Error(codes.Internal, "failed to get counter status")
	}

	var configInfo *proto.ConfigInfo
	if status.Config != nil {
		configInfo = &proto.ConfigInfo{
			Prefix:       status.Config.Prefix,
			Format:       status.Config.Format,
			Padding:      int32(status.Config.Padding),
			Separator:    status.Config.Separator,
			InitialValue: status.Config.InitialValue,
			MaxValue:     status.Config.MaxValue,
			IsActive:     status.Config.IsActive,
			Description:  status.Config.Description,
		}
	}

	return &proto.GetStatusResponse{
		Prefix:         status.Prefix,
		CurrentCounter: status.CurrentCounter,
		IsActive:       status.IsActive,
		LastGenerated:  status.LastGenerated.Format(time.RFC3339),
		TotalGenerated: status.TotalGenerated,
		Config:         configInfo,
	}, nil
}

// Health performs a health check
func (s *Server) Health(ctx context.Context, req *proto.HealthRequest) (*proto.HealthResponse, error) {
	healthy, details := s.sequentialIDService.HealthCheck(ctx)
	
	status := proto.HealthResponse_SERVING
	message := "Service is healthy"
	
	if !healthy {
		status = proto.HealthResponse_NOT_SERVING
		message = "Service is unhealthy"
	}

	detailsMap := make(map[string]string)
	for key, value := range details {
		detailsMap[key] = value
	}

	return &proto.HealthResponse{
		Status:  status,
		Message: message,
		Details: detailsMap,
	}, nil
}

// GetConfig gets the configuration for a prefix
func (s *Server) GetConfig(ctx context.Context, req *proto.GetConfigRequest) (*proto.GetConfigResponse, error) {
	if req.Prefix == "" {
		return nil, status.Error(codes.InvalidArgument, "prefix is required")
	}

	config, err := s.sequentialIDService.GetConfig(ctx, req.Prefix)
	if err != nil {
		s.logger.WithError(err).WithField("prefix", req.Prefix).Error("Failed to get config")
		return nil, status.Error(codes.Internal, "failed to get configuration")
	}

	if config == nil {
		return &proto.GetConfigResponse{
			Found: false,
		}, nil
	}

	return &proto.GetConfigResponse{
		Config: &proto.ConfigInfo{
			Prefix:       config.Prefix,
			Format:       config.Format,
			Padding:      int32(config.Padding),
			Separator:    config.Separator,
			InitialValue: config.InitialValue,
			MaxValue:     config.MaxValue,
			IsActive:     config.IsActive,
			Description:  config.Description,
		},
		Found: true,
	}, nil
}

// UpdateConfig updates the configuration for a prefix
func (s *Server) UpdateConfig(ctx context.Context, req *proto.UpdateConfigRequest) (*proto.UpdateConfigResponse, error) {
	if req.Config == nil {
		return nil, status.Error(codes.InvalidArgument, "config is required")
	}

	if req.Config.Prefix == "" {
		return nil, status.Error(codes.InvalidArgument, "prefix is required")
	}

	config := &models.PrefixConfig{
		Prefix:       req.Config.Prefix,
		Format:       req.Config.Format,
		Padding:      int(req.Config.Padding),
		Separator:    req.Config.Separator,
		InitialValue: req.Config.InitialValue,
		MaxValue:     req.Config.MaxValue,
		IsActive:     req.Config.IsActive,
		Description:  req.Config.Description,
	}

	err := s.sequentialIDService.UpdateConfig(ctx, config, req.ClientId, req.CorrelationId)
	if err != nil {
		s.logger.WithError(err).WithFields(logrus.Fields{
			"prefix":         req.Config.Prefix,
			"client_id":      req.ClientId,
			"correlation_id": req.CorrelationId,
		}).Error("Failed to update config")
		
		return nil, status.Error(codes.Internal, "failed to update configuration")
	}

	return &proto.UpdateConfigResponse{
		Success: true,
		Message: "Configuration updated successfully",
		Config:  req.Config,
	}, nil
}
