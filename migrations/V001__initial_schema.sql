-- V001__initial_schema.sql
-- Sequential ID Counter Service - Initial Database Schema

-- Configuration table for prefix settings
CREATE TABLE seq_config (
    id BIGSERIAL PRIMARY KEY,
    prefix VARCHAR(50) UNIQUE NOT NULL,
    padding_length INTEGER NOT NULL DEFAULT 6,
    format_template TEXT NOT NULL DEFAULT '%s%06d',
    reset_rule VARCHAR(20) NOT NULL DEFAULT 'never' CHECK (reset_rule IN ('never', 'daily', 'monthly', 'yearly')),
    last_reset_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_by VARCHAR(100),
    updated_by VARCHAR(100)
);

-- Audit log table for all generated IDs
CREATE TABLE seq_log (
    id BIGSERIAL PRIMARY KEY,
    prefix VARCHAR(50) NOT NULL,
    counter_value BIGINT NOT NULL,
    full_number VARCHAR(255) NOT NULL,
    generated_by VARCHAR(100),
    client_id VARCHAR(100),
    correlation_id VARCHAR(255),
    message_id VARCHAR(255) UNIQUE,
    generated_at TIMESTAMP WITH TIME ZONE NOT NULL,
    published_at TIMESTAMP WITH TIME ZONE,
    inserted_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    batch_id VARCHAR(255),
    
    -- Ensure uniqueness per prefix
    UNIQUE(prefix, counter_value)
);

-- Checkpoint table for fast recovery and sync
CREATE TABLE seq_checkpoint (
    prefix VARCHAR(50) PRIMARY KEY,
    last_counter_synced BIGINT NOT NULL,
    synced_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    synced_by VARCHAR(100)
);

-- Reset operations audit table
CREATE TABLE seq_reset_log (
    id BIGSERIAL PRIMARY KEY,
    prefix VARCHAR(50) NOT NULL,
    old_value BIGINT NOT NULL,
    new_value BIGINT NOT NULL,
    reason TEXT NOT NULL,
    admin_user VARCHAR(100) NOT NULL,
    reset_id VARCHAR(255) UNIQUE NOT NULL,
    reset_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Configuration change audit table
CREATE TABLE seq_config_audit (
    id BIGSERIAL PRIMARY KEY,
    prefix VARCHAR(50) NOT NULL,
    old_config JSONB,
    new_config JSONB NOT NULL,
    change_type VARCHAR(20) NOT NULL CHECK (change_type IN ('CREATE', 'UPDATE', 'DELETE')),
    admin_user VARCHAR(100) NOT NULL,
    changed_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes for performance
CREATE INDEX idx_seq_log_prefix_counter ON seq_log(prefix, counter_value);
CREATE INDEX idx_seq_log_generated_at ON seq_log(generated_at);
CREATE INDEX idx_seq_log_full_number ON seq_log(full_number);
CREATE INDEX idx_seq_log_message_id ON seq_log(message_id);
CREATE INDEX idx_seq_log_batch_id ON seq_log(batch_id) WHERE batch_id IS NOT NULL;
CREATE INDEX idx_seq_log_client_id ON seq_log(client_id) WHERE client_id IS NOT NULL;

CREATE INDEX idx_seq_reset_log_prefix ON seq_reset_log(prefix);
CREATE INDEX idx_seq_reset_log_reset_at ON seq_reset_log(reset_at);

CREATE INDEX idx_seq_config_audit_prefix ON seq_config_audit(prefix);
CREATE INDEX idx_seq_config_audit_changed_at ON seq_config_audit(changed_at);

-- Trigger to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_seq_config_updated_at 
    BEFORE UPDATE ON seq_config 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Function to get next counter value (used by reconciliation)
CREATE OR REPLACE FUNCTION get_max_counter(p_prefix VARCHAR(50))
RETURNS BIGINT AS $$
DECLARE
    max_counter BIGINT;
BEGIN
    SELECT COALESCE(MAX(counter_value), 0) INTO max_counter
    FROM seq_log
    WHERE prefix = p_prefix;
    
    RETURN max_counter;
END;
$$ LANGUAGE plpgsql;

-- Function to validate format template
CREATE OR REPLACE FUNCTION validate_format_template(template TEXT)
RETURNS BOOLEAN AS $$
BEGIN
    -- Basic validation - should contain %s and %d placeholders
    IF template IS NULL OR template = '' THEN
        RETURN FALSE;
    END IF;
    
    -- Should contain at least one %s or %d
    IF position('%' in template) = 0 THEN
        RETURN FALSE;
    END IF;
    
    RETURN TRUE;
EXCEPTION
    WHEN OTHERS THEN
        RETURN FALSE;
END;
$$ LANGUAGE plpgsql;

-- Add validation constraint
ALTER TABLE seq_config ADD CONSTRAINT chk_format_template 
    CHECK (validate_format_template(format_template));

-- Insert default configurations
INSERT INTO seq_config (prefix, padding_length, format_template, reset_rule, created_by) VALUES 
('SG', 6, '%s%06d', 'never', 'system'),
('INV', 4, 'INV%d-%04d', 'yearly', 'system'),
('PO', 8, '%s%08d', 'monthly', 'system'),
('SO', 6, 'SO%06d', 'never', 'system'),
('QUO', 6, 'QUO%06d', 'monthly', 'system');

-- Insert initial checkpoints
INSERT INTO seq_checkpoint (prefix, last_counter_synced, synced_by) VALUES 
('SG', 0, 'system'),
('INV', 0, 'system'),
('PO', 0, 'system'),
('SO', 0, 'system'),
('QUO', 0, 'system');

-- Comments for documentation
COMMENT ON TABLE seq_config IS 'Configuration settings for each prefix';
COMMENT ON TABLE seq_log IS 'Audit trail of all generated sequential IDs';
COMMENT ON TABLE seq_checkpoint IS 'Checkpoints for fast recovery and Redis sync';
COMMENT ON TABLE seq_reset_log IS 'Audit trail of counter reset operations';
COMMENT ON TABLE seq_config_audit IS 'Audit trail of configuration changes';

COMMENT ON COLUMN seq_config.prefix IS 'Unique prefix identifier (e.g., SG, INV)';
COMMENT ON COLUMN seq_config.padding_length IS 'Number of digits for zero-padding';
COMMENT ON COLUMN seq_config.format_template IS 'Printf-style format template';
COMMENT ON COLUMN seq_config.reset_rule IS 'When to reset counters: never, daily, monthly, yearly';

COMMENT ON COLUMN seq_log.counter_value IS 'Numeric part of the sequential ID';
COMMENT ON COLUMN seq_log.full_number IS 'Complete formatted sequential ID';
COMMENT ON COLUMN seq_log.message_id IS 'Unique message identifier for idempotency';
COMMENT ON COLUMN seq_log.batch_id IS 'Batch identifier for bulk operations';
