-- Migration 004: Retry Queue for failed payments
-- Creates the retry_queue table for managing payment retry attempts

-- Create retry_queue table
CREATE TABLE IF NOT EXISTS retry_queue (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subscription_id UUID NOT NULL REFERENCES subscriptions(id),

    -- Retry state
    attempt INT NOT NULL DEFAULT 1,
    max_attempts INT NOT NULL DEFAULT 3,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',

    -- Error information from last attempt
    last_error_code VARCHAR(100),
    last_error_message TEXT,
    decline_type VARCHAR(50), -- 'soft' (retry) or 'hard' (don't retry)

    -- Scheduling
    next_retry_at TIMESTAMP NOT NULL,
    last_attempt_at TIMESTAMP,

    -- Result tracking
    transaction_id VARCHAR(255),
    processor_used VARCHAR(100),

    -- Metadata
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMP,

    CONSTRAINT valid_retry_status CHECK (status IN ('pending', 'processing', 'succeeded', 'failed', 'canceled', 'exhausted')),
    CONSTRAINT valid_decline_type CHECK (decline_type IN ('soft', 'hard') OR decline_type IS NULL),
    CONSTRAINT valid_attempt_count CHECK (attempt >= 1 AND attempt <= max_attempts + 1)
);

-- Indexes for retry queue queries
CREATE INDEX IF NOT EXISTS idx_retry_queue_status ON retry_queue(status);
CREATE INDEX IF NOT EXISTS idx_retry_queue_next_retry ON retry_queue(next_retry_at) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_retry_queue_subscription ON retry_queue(subscription_id);
CREATE INDEX IF NOT EXISTS idx_retry_queue_created ON retry_queue(created_at);

-- Add retry-related columns to subscriptions if not exists
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name = 'subscriptions' AND column_name = 'retry_count') THEN
        ALTER TABLE subscriptions ADD COLUMN retry_count INT NOT NULL DEFAULT 0;
    END IF;

    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name = 'subscriptions' AND column_name = 'last_payment_error') THEN
        ALTER TABLE subscriptions ADD COLUMN last_payment_error TEXT;
    END IF;
END $$;

-- Function to calculate next retry time based on exponential backoff
-- Retry intervals: 1 hour, 24 hours, 72 hours
CREATE OR REPLACE FUNCTION calculate_next_retry(attempt_num INT) RETURNS INTERVAL AS $$
BEGIN
    CASE attempt_num
        WHEN 1 THEN RETURN INTERVAL '1 hour';
        WHEN 2 THEN RETURN INTERVAL '24 hours';
        WHEN 3 THEN RETURN INTERVAL '72 hours';
        ELSE RETURN INTERVAL '72 hours';
    END CASE;
END;
$$ LANGUAGE plpgsql IMMUTABLE;

-- View for active retries (pending and due)
CREATE OR REPLACE VIEW active_retries AS
SELECT
    r.*,
    s.user_id,
    s.plan_id,
    s.amount,
    s.currency,
    s.status as subscription_status,
    CASE
        WHEN r.next_retry_at <= NOW() THEN true
        ELSE false
    END as is_due
FROM retry_queue r
JOIN subscriptions s ON r.subscription_id = s.id
WHERE r.status = 'pending';

-- Comment on table
COMMENT ON TABLE retry_queue IS 'Manages failed payment retry attempts with exponential backoff';
