package models

import (
	"time"
)

// SequentialID represents a generated sequential ID
type SequentialID struct {
	Prefix      string    `json:"prefix"`
	Counter     int64     `json:"counter"`
	FullNumber  string    `json:"full_number"`
	GeneratedBy string    `json:"generated_by,omitempty"`
	ClientID    string    `json:"client_id,omitempty"`
	MessageID   string    `json:"message_id"`
	GeneratedAt time.Time `json:"generated_at"`
}

// PrefixConfig represents configuration for a prefix
type PrefixConfig struct {
	ID             int64      `json:"id" db:"id"`
	Prefix         string     `json:"prefix" db:"prefix"`
	PaddingLength  int        `json:"padding_length" db:"padding_length"`
	FormatTemplate string     `json:"format_template" db:"format_template"`
	ResetRule      string     `json:"reset_rule" db:"reset_rule"`
	LastResetAt    *time.Time `json:"last_reset_at,omitempty" db:"last_reset_at"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at" db:"updated_at"`
	CreatedBy      *string    `json:"created_by,omitempty" db:"created_by"`
	UpdatedBy      *string    `json:"updated_by,omitempty" db:"updated_by"`
}

// AuditLog represents an audit log entry
type AuditLog struct {
	ID            int64      `json:"id" db:"id"`
	Prefix        string     `json:"prefix" db:"prefix"`
	CounterValue  int64      `json:"counter_value" db:"counter_value"`
	FullNumber    string     `json:"full_number" db:"full_number"`
	GeneratedBy   *string    `json:"generated_by,omitempty" db:"generated_by"`
	ClientID      *string    `json:"client_id,omitempty" db:"client_id"`
	CorrelationID *string    `json:"correlation_id,omitempty" db:"correlation_id"`
	MessageID     string     `json:"message_id" db:"message_id"`
	GeneratedAt   time.Time  `json:"generated_at" db:"generated_at"`
	PublishedAt   *time.Time `json:"published_at,omitempty" db:"published_at"`
	InsertedAt    time.Time  `json:"inserted_at" db:"inserted_at"`
	BatchID       *string    `json:"batch_id,omitempty" db:"batch_id"`
}

// Checkpoint represents a counter checkpoint
type Checkpoint struct {
	Prefix            string    `json:"prefix" db:"prefix"`
	LastCounterSynced int64     `json:"last_counter_synced" db:"last_counter_synced"`
	SyncedAt          time.Time `json:"synced_at" db:"synced_at"`
	SyncedBy          *string   `json:"synced_by,omitempty" db:"synced_by"`
}

// ResetLog represents a counter reset operation
type ResetLog struct {
	ID        int64     `json:"id" db:"id"`
	Prefix    string    `json:"prefix" db:"prefix"`
	OldValue  int64     `json:"old_value" db:"old_value"`
	NewValue  int64     `json:"new_value" db:"new_value"`
	Reason    string    `json:"reason" db:"reason"`
	AdminUser string    `json:"admin_user" db:"admin_user"`
	ResetID   string    `json:"reset_id" db:"reset_id"`
	ResetAt   time.Time `json:"reset_at" db:"reset_at"`
}

// HealthStatus represents service health status
type HealthStatus struct {
	Healthy    bool              `json:"healthy"`
	Components map[string]string `json:"components"`
	Timestamp  time.Time         `json:"timestamp"`
}

// CounterStatus represents the status of a counter
type CounterStatus struct {
	Prefix           string `json:"prefix"`
	CurrentCounter   int64  `json:"current_counter"`
	NextCounter      int64  `json:"next_counter"`
	RedisHealthy     bool   `json:"redis_healthy"`
	QueueHealthy     bool   `json:"queue_healthy"`
	DatabaseHealthy  bool   `json:"database_healthy"`
	LastAuditCounter int64  `json:"last_audit_counter"`
}

// Event represents an event to be published to message queue
type Event struct {
	MessageID     string    `json:"message_id"`
	Prefix        string    `json:"prefix"`
	Counter       int64     `json:"counter"`
	FullNumber    string    `json:"full_number"`
	GeneratedBy   string    `json:"generated_by"`
	ClientID      string    `json:"client_id"`
	CorrelationID string    `json:"correlation_id,omitempty"`
	GeneratedAt   time.Time `json:"generated_at"`
	PublishedAt   time.Time `json:"published_at"`
	RetryCount    int       `json:"retry_count"`
	BatchID       string    `json:"batch_id,omitempty"`
}

// BatchRequest represents a request for multiple IDs
type BatchRequest struct {
	Prefix        string `json:"prefix"`
	Count         int    `json:"count"`
	ClientID      string `json:"client_id"`
	GeneratedBy   string `json:"generated_by"`
	CorrelationID string `json:"correlation_id,omitempty"`
}

// BatchResponse represents a response with multiple IDs
type BatchResponse struct {
	IDs         []SequentialID `json:"ids"`
	BatchID     string         `json:"batch_id"`
	Count       int            `json:"count"`
	GeneratedAt time.Time      `json:"generated_at"`
}

// ResetRequest represents a request to reset a counter
type ResetRequest struct {
	SetTo     int64  `json:"set_to"`
	Reason    string `json:"reason"`
	AdminUser string `json:"admin_user"`
	Force     bool   `json:"force,omitempty"`
}

// ResetResponse represents a response to a reset operation
type ResetResponse struct {
	Success  bool   `json:"success"`
	Message  string `json:"message"`
	OldValue int64  `json:"old_value"`
	NewValue int64  `json:"new_value"`
	ResetID  string `json:"reset_id"`
}

// ConfigUpdateRequest represents a request to update prefix configuration
type ConfigUpdateRequest struct {
	PaddingLength     *int    `json:"padding_length,omitempty"`
	FormatTemplate    *string `json:"format_template,omitempty"`
	ResetRule         *string `json:"reset_rule,omitempty"`
	AdminUser         string  `json:"admin_user"`
	CreateIfNotExists bool    `json:"create_if_not_exists,omitempty"`
}
