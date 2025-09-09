package rest

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/putram11/sequential-id-counter-service/internal/models"
	"github.com/putram11/sequential-id-counter-service/internal/service"
	"github.com/sirupsen/logrus"
)

// Handler handles REST API requests
type Handler struct {
	service *service.SequentialIDService
	logger  *logrus.Logger
}

// NewHandler creates a new REST API handler
func NewHandler(service *service.SequentialIDService, logger *logrus.Logger) *Handler {
	return &Handler{
		service: service,
		logger:  logger,
	}
}

// GetNext generates the next sequential ID
// @Summary Generate next sequential ID
// @Description Generate the next sequential ID for a given prefix
// @Tags sequential-id
// @Accept json
// @Produce json
// @Param prefix path string true "Prefix identifier"
// @Param client_id query string false "Client identifier"
// @Param generated_by query string false "User or system that generated the ID"
// @Success 200 {object} models.SequentialID
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/next/{prefix} [get]
func (h *Handler) GetNext(c *gin.Context) {
	prefix := c.Param("prefix")
	if prefix == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "prefix is required"})
		return
	}

	clientID := c.Query("client_id")
	generatedBy := c.Query("generated_by")

	seqID, err := h.service.GetNext(c.Request.Context(), prefix, clientID, generatedBy)
	if err != nil {
		h.logger.WithError(err).WithField("prefix", prefix).Error("Failed to generate sequential ID")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, seqID)
}

// GetNextBatch generates multiple sequential IDs
// @Summary Generate batch of sequential IDs
// @Description Generate multiple sequential IDs for a given prefix
// @Tags sequential-id
// @Accept json
// @Produce json
// @Param prefix path string true "Prefix identifier"
// @Param request body models.BatchRequest true "Batch request"
// @Success 200 {object} models.BatchResponse
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/batch/{prefix} [post]
func (h *Handler) GetNextBatch(c *gin.Context) {
	prefix := c.Param("prefix")
	if prefix == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "prefix is required"})
		return
	}

	var req models.BatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req.Prefix = prefix // Override with path parameter

	resp, err := h.service.GetNextBatch(c.Request.Context(), &req)
	if err != nil {
		h.logger.WithError(err).WithField("prefix", prefix).Error("Failed to generate batch of sequential IDs")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetStatus returns the current status of a counter
// @Summary Get counter status
// @Description Get the current status and health of a counter
// @Tags sequential-id
// @Accept json
// @Produce json
// @Param prefix path string true "Prefix identifier"
// @Success 200 {object} models.CounterStatus
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/status/{prefix} [get]
func (h *Handler) GetStatus(c *gin.Context) {
	prefix := c.Param("prefix")
	if prefix == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "prefix is required"})
		return
	}

	status, err := h.service.GetStatus(c.Request.Context(), prefix)
	if err != nil {
		h.logger.WithError(err).WithField("prefix", prefix).Error("Failed to get counter status")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, status)
}

// ResetCounter resets a counter to a specific value (admin operation)
// @Summary Reset counter
// @Description Reset a counter to a specific value (requires admin authentication)
// @Tags admin
// @Accept json
// @Produce json
// @Param prefix path string true "Prefix identifier"
// @Param request body models.ResetRequest true "Reset request"
// @Security BearerAuth
// @Success 200 {object} models.ResetResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/reset/{prefix} [post]
func (h *Handler) ResetCounter(c *gin.Context) {
	// TODO: Add authentication middleware
	prefix := c.Param("prefix")
	if prefix == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "prefix is required"})
		return
	}

	var req models.ResetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.service.ResetCounter(c.Request.Context(), prefix, &req)
	if err != nil {
		h.logger.WithError(err).WithFields(logrus.Fields{
			"prefix":     prefix,
			"set_to":     req.SetTo,
			"admin_user": req.AdminUser,
		}).Error("Failed to reset counter")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetConfig retrieves configuration for a prefix
// @Summary Get prefix configuration
// @Description Get configuration settings for a prefix
// @Tags configuration
// @Accept json
// @Produce json
// @Param prefix path string true "Prefix identifier"
// @Success 200 {object} models.PrefixConfig
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/config/{prefix} [get]
func (h *Handler) GetConfig(c *gin.Context) {
	prefix := c.Param("prefix")
	if prefix == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "prefix is required"})
		return
	}

	config, err := h.service.GetConfig(c.Request.Context(), prefix)
	if err != nil {
		h.logger.WithError(err).WithField("prefix", prefix).Error("Failed to get prefix config")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if config == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "prefix not found"})
		return
	}

	c.JSON(http.StatusOK, config)
}

// UpdateConfig updates configuration for a prefix (admin operation)
// @Summary Update prefix configuration
// @Description Update configuration settings for a prefix (requires admin authentication)
// @Tags admin
// @Accept json
// @Produce json
// @Param prefix path string true "Prefix identifier"
// @Param request body models.ConfigUpdateRequest true "Configuration update request"
// @Security BearerAuth
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/config/{prefix} [post]
func (h *Handler) UpdateConfig(c *gin.Context) {
	// TODO: Add authentication middleware
	prefix := c.Param("prefix")
	if prefix == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "prefix is required"})
		return
	}

	var req models.ConfigUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.service.UpdateConfig(c.Request.Context(), prefix, &req)
	if err != nil {
		h.logger.WithError(err).WithFields(logrus.Fields{
			"prefix":     prefix,
			"admin_user": req.AdminUser,
		}).Error("Failed to update prefix config")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "configuration updated successfully"})
}

// GetAuditLogs retrieves audit logs for a prefix
// @Summary Get audit logs
// @Description Get audit logs for a prefix with pagination
// @Tags audit
// @Accept json
// @Produce json
// @Param prefix path string true "Prefix identifier"
// @Param limit query int false "Number of records to return (default: 100)"
// @Param offset query int false "Number of records to skip (default: 0)"
// @Success 200 {array} models.AuditLog
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/audit/{prefix} [get]
func (h *Handler) GetAuditLogs(c *gin.Context) {
	prefix := c.Param("prefix")
	if prefix == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "prefix is required"})
		return
	}

	limit := 100
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 1000 {
			limit = parsedLimit
		}
	}

	offset := 0
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err == nil && parsedOffset >= 0 {
			offset = parsedOffset
		}
	}

	// This would require implementing GetAuditLogs in the service
	// For now, return a placeholder response
	c.JSON(http.StatusOK, gin.H{
		"message": "audit logs endpoint - to be implemented",
		"prefix":  prefix,
		"limit":   limit,
		"offset":  offset,
	})
}

// HealthCheck returns service health status
// @Summary Health check
// @Description Get the health status of the service and its components
// @Tags health
// @Accept json
// @Produce json
// @Success 200 {object} models.HealthStatus
// @Failure 503 {object} models.HealthStatus
// @Router /health [get]
func (h *Handler) HealthCheck(c *gin.Context) {
	health := h.service.HealthCheck(c.Request.Context())

	if health.Healthy {
		c.JSON(http.StatusOK, health)
	} else {
		c.JSON(http.StatusServiceUnavailable, health)
	}
}

// Metrics returns service metrics (for Prometheus)
// @Summary Get metrics
// @Description Get service metrics in Prometheus format
// @Tags monitoring
// @Produce text/plain
// @Success 200 {string} string "Prometheus metrics"
// @Router /metrics [get]
func (h *Handler) Metrics(c *gin.Context) {
	// This would integrate with Prometheus metrics
	// For now, return a placeholder
	c.Header("Content-Type", "text/plain")
	c.String(http.StatusOK, "# Metrics endpoint - to be implemented with Prometheus client\n")
}
