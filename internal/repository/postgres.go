package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/putram11/sequential-id-counter-service/internal/config"
	"github.com/putram11/sequential-id-counter-service/internal/models"
)

// PostgresRepository handles PostgreSQL operations
type PostgresRepository struct {
	db *sqlx.DB
}

// NewPostgresRepository creates a new PostgreSQL repository
func NewPostgresRepository(cfg config.DatabaseConfig) (*PostgresRepository, error) {
	db, err := sqlx.Connect("postgres", cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(time.Hour)

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &PostgresRepository{
		db: db,
	}, nil
}

// GetPrefixConfig retrieves configuration for a prefix
func (r *PostgresRepository) GetPrefixConfig(ctx context.Context, prefix string) (*models.PrefixConfig, error) {
	var config models.PrefixConfig
	query := `
		SELECT id, prefix, padding_length, format_template, reset_rule, 
		       last_reset_at, created_at, updated_at, created_by, updated_by
		FROM seq_config 
		WHERE prefix = $1
	`

	err := r.db.GetContext(ctx, &config, query, prefix)
	if err == sql.ErrNoRows {
		return nil, nil // Not found
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get prefix config for %s: %w", prefix, err)
	}

	return &config, nil
}

// CreatePrefixConfig creates a new prefix configuration
func (r *PostgresRepository) CreatePrefixConfig(ctx context.Context, config *models.PrefixConfig) error {
	query := `
		INSERT INTO seq_config (prefix, padding_length, format_template, reset_rule, created_by)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at
	`

	err := r.db.QueryRowContext(ctx, query,
		config.Prefix,
		config.PaddingLength,
		config.FormatTemplate,
		config.ResetRule,
		config.CreatedBy,
	).Scan(&config.ID, &config.CreatedAt, &config.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create prefix config: %w", err)
	}

	return nil
}

// UpdatePrefixConfig updates an existing prefix configuration
func (r *PostgresRepository) UpdatePrefixConfig(ctx context.Context, prefix string, updates map[string]interface{}) error {
	// Build dynamic update query
	setParts := []string{}
	args := []interface{}{}
	argIndex := 1

	for field, value := range updates {
		setParts = append(setParts, fmt.Sprintf("%s = $%d", field, argIndex))
		args = append(args, value)
		argIndex++
	}

	// Always update the updated_at field
	setParts = append(setParts, fmt.Sprintf("updated_at = $%d", argIndex))
	args = append(args, time.Now())
	argIndex++

	// Add WHERE clause
	args = append(args, prefix)

	query := fmt.Sprintf(`
		UPDATE seq_config 
		SET %s
		WHERE prefix = $%d
	`,
		fmt.Sprintf("%s", setParts),
		argIndex,
	)

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update prefix config: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check affected rows: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("prefix %s not found", prefix)
	}

	return nil
}

// GetAllPrefixConfigs retrieves all prefix configurations
func (r *PostgresRepository) GetAllPrefixConfigs(ctx context.Context) ([]models.PrefixConfig, error) {
	var configs []models.PrefixConfig
	query := `
		SELECT id, prefix, padding_length, format_template, reset_rule,
		       last_reset_at, created_at, updated_at, created_by, updated_by
		FROM seq_config
		ORDER BY prefix
	`

	err := r.db.SelectContext(ctx, &configs, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get all prefix configs: %w", err)
	}

	return configs, nil
}

// InsertAuditLog inserts an audit log entry
func (r *PostgresRepository) InsertAuditLog(ctx context.Context, log *models.AuditLog) error {
	query := `
		INSERT INTO seq_log (prefix, counter_value, full_number, generated_by, client_id,
		                    correlation_id, message_id, generated_at, published_at, batch_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (prefix, counter_value) DO NOTHING
		RETURNING id, inserted_at
	`

	err := r.db.QueryRowContext(ctx, query,
		log.Prefix,
		log.CounterValue,
		log.FullNumber,
		log.GeneratedBy,
		log.ClientID,
		log.CorrelationID,
		log.MessageID,
		log.GeneratedAt,
		log.PublishedAt,
		log.BatchID,
	).Scan(&log.ID, &log.InsertedAt)

	if err != nil {
		// Check if it's a conflict (duplicate)
		if err == sql.ErrNoRows {
			// ON CONFLICT DO NOTHING was triggered
			return nil
		}
		return fmt.Errorf("failed to insert audit log: %w", err)
	}

	return nil
}

// GetMaxCounter retrieves the maximum counter value for a prefix
func (r *PostgresRepository) GetMaxCounter(ctx context.Context, prefix string) (int64, error) {
	var maxCounter sql.NullInt64
	query := `
		SELECT MAX(counter_value)
		FROM seq_log
		WHERE prefix = $1
	`

	err := r.db.QueryRowContext(ctx, query, prefix).Scan(&maxCounter)
	if err != nil {
		return 0, fmt.Errorf("failed to get max counter for prefix %s: %w", prefix, err)
	}

	if !maxCounter.Valid {
		return 0, nil // No records found
	}

	return maxCounter.Int64, nil
}

// UpdateCheckpoint updates or creates a checkpoint
func (r *PostgresRepository) UpdateCheckpoint(ctx context.Context, checkpoint *models.Checkpoint) error {
	query := `
		INSERT INTO seq_checkpoint (prefix, last_counter_synced, synced_by)
		VALUES ($1, $2, $3)
		ON CONFLICT (prefix) 
		DO UPDATE SET 
			last_counter_synced = EXCLUDED.last_counter_synced,
			synced_at = NOW(),
			synced_by = EXCLUDED.synced_by
	`

	_, err := r.db.ExecContext(ctx, query,
		checkpoint.Prefix,
		checkpoint.LastCounterSynced,
		checkpoint.SyncedBy,
	)

	if err != nil {
		return fmt.Errorf("failed to update checkpoint: %w", err)
	}

	return nil
}

// GetCheckpoint retrieves a checkpoint for a prefix
func (r *PostgresRepository) GetCheckpoint(ctx context.Context, prefix string) (*models.Checkpoint, error) {
	var checkpoint models.Checkpoint
	query := `
		SELECT prefix, last_counter_synced, synced_at, synced_by
		FROM seq_checkpoint
		WHERE prefix = $1
	`

	err := r.db.GetContext(ctx, &checkpoint, query, prefix)
	if err == sql.ErrNoRows {
		return nil, nil // Not found
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get checkpoint for prefix %s: %w", prefix, err)
	}

	return &checkpoint, nil
}

// InsertResetLog logs a counter reset operation
func (r *PostgresRepository) InsertResetLog(ctx context.Context, resetLog *models.ResetLog) error {
	query := `
		INSERT INTO seq_reset_log (prefix, old_value, new_value, reason, admin_user, reset_id)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, reset_at
	`

	err := r.db.QueryRowContext(ctx, query,
		resetLog.Prefix,
		resetLog.OldValue,
		resetLog.NewValue,
		resetLog.Reason,
		resetLog.AdminUser,
		resetLog.ResetID,
	).Scan(&resetLog.ID, &resetLog.ResetAt)

	if err != nil {
		return fmt.Errorf("failed to insert reset log: %w", err)
	}

	return nil
}

// GetAuditLogs retrieves audit logs with pagination
func (r *PostgresRepository) GetAuditLogs(ctx context.Context, prefix string, limit, offset int) ([]models.AuditLog, error) {
	var logs []models.AuditLog
	query := `
		SELECT id, prefix, counter_value, full_number, generated_by, client_id,
		       correlation_id, message_id, generated_at, published_at, inserted_at, batch_id
		FROM seq_log
		WHERE prefix = $1
		ORDER BY counter_value DESC
		LIMIT $2 OFFSET $3
	`

	err := r.db.SelectContext(ctx, &logs, query, prefix, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get audit logs for prefix %s: %w", prefix, err)
	}

	return logs, nil
}

// Ping checks database connectivity
func (r *PostgresRepository) Ping(ctx context.Context) error {
	return r.db.PingContext(ctx)
}

// Close closes the database connection
func (r *PostgresRepository) Close() error {
	return r.db.Close()
}

// BeginTx starts a new transaction
func (r *PostgresRepository) BeginTx(ctx context.Context) (*sqlx.Tx, error) {
	return r.db.BeginTxx(ctx, nil)
}

// GetStats returns database statistics
func (r *PostgresRepository) GetStats() sql.DBStats {
	return r.db.Stats()
}
