-- Payment Orchestration System - Initial Schema
-- This migration creates all core tables for the payment system

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Subscriptions table - user billing agreements
CREATE TABLE subscriptions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'active', -- active, cancelled, expired, pending, failed
    plan_id VARCHAR(100) NOT NULL,
    amount DECIMAL(10,2) NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'USD',
    billing_cycle VARCHAR(20) NOT NULL DEFAULT 'monthly', -- monthly, yearly
    next_billing_date TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Payment methods table - tokenized payment information
-- Supports both network tokens (portable) and dual vault (processor-specific)
CREATE TABLE payment_methods (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL,
    network_token VARCHAR(255), -- Primary portable token (95% cases)
    processor_a_token VARCHAR(255), -- Fallback token for processor A
    processor_b_token VARCHAR(255), -- Fallback token for processor B
    token_type VARCHAR(50) NOT NULL, -- 'network' or 'dual_vault'
    last_four VARCHAR(4),
    exp_month INTEGER,
    exp_year INTEGER,
    is_default BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Transactions table - all payment attempts and refunds
CREATE TABLE transactions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    subscription_id UUID REFERENCES subscriptions(id),
    payment_method_id UUID REFERENCES payment_methods(id),
    processor_used VARCHAR(50) NOT NULL, -- 'processor_a' or 'processor_b'
    amount DECIMAL(10,2) NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'USD',
    status VARCHAR(50) NOT NULL, -- pending, success, failed
    transaction_type VARCHAR(50) NOT NULL DEFAULT 'charge', -- charge, refund
    idempotency_key VARCHAR(255) UNIQUE NOT NULL,
    processor_transaction_id VARCHAR(255),
    error_code VARCHAR(100),
    error_message TEXT,
    original_transaction_id UUID REFERENCES transactions(id), -- For refunds
    created_at TIMESTAMP DEFAULT NOW()
);

-- Routing rules table for BPAS (Business Profile Authority Service)
CREATE TABLE routing_rules (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    rule_name VARCHAR(100) NOT NULL UNIQUE,
    priority INTEGER NOT NULL, -- Lower number = higher priority
    condition_type VARCHAR(50) NOT NULL, -- 'amount_threshold', 'plan_based', 'percentage', 'failover'
    condition_value JSONB, -- Flexible condition storage
    target_processor VARCHAR(50) NOT NULL, -- 'processor_a' or 'processor_b'
    percentage INTEGER DEFAULT 100, -- Traffic percentage for this rule
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Processor health tracking for monitoring and circuit breaker logic
CREATE TABLE processor_health (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    processor_name VARCHAR(50) NOT NULL UNIQUE,
    is_healthy BOOLEAN DEFAULT true,
    last_check TIMESTAMP DEFAULT NOW(),
    failure_count INTEGER DEFAULT 0,
    success_rate DECIMAL(5,2) DEFAULT 100.00,
    avg_response_time_ms INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Indexes for performance optimization
CREATE INDEX idx_subscriptions_user_id ON subscriptions(user_id);
CREATE INDEX idx_subscriptions_status ON subscriptions(status);
CREATE INDEX idx_subscriptions_next_billing ON subscriptions(next_billing_date);
CREATE INDEX idx_subscriptions_status_billing ON subscriptions(status, next_billing_date) WHERE status = 'active';

CREATE INDEX idx_payment_methods_user_id ON payment_methods(user_id);
CREATE INDEX idx_payment_methods_token_type ON payment_methods(token_type);
CREATE INDEX idx_payment_methods_user_default ON payment_methods(user_id, is_default) WHERE is_default = true;

CREATE INDEX idx_transactions_subscription_id ON transactions(subscription_id);
CREATE INDEX idx_transactions_status ON transactions(status);
CREATE INDEX idx_transactions_processor ON transactions(processor_used);
CREATE INDEX idx_transactions_idempotency ON transactions(idempotency_key);
CREATE INDEX idx_transactions_created_at ON transactions(created_at);
CREATE INDEX idx_transactions_type_status ON transactions(transaction_type, status);

CREATE INDEX idx_routing_rules_priority ON routing_rules(priority);
CREATE INDEX idx_routing_rules_active ON routing_rules(is_active) WHERE is_active = true;
CREATE INDEX idx_routing_rules_processor ON routing_rules(target_processor);

CREATE INDEX idx_processor_health_name ON processor_health(processor_name);
CREATE INDEX idx_processor_health_healthy ON processor_health(is_healthy);

-- Constraints for data integrity
ALTER TABLE transactions ADD CONSTRAINT chk_amount_positive CHECK (amount > 0);
ALTER TABLE subscriptions ADD CONSTRAINT chk_amount_positive CHECK (amount > 0);
ALTER TABLE routing_rules ADD CONSTRAINT chk_percentage_valid CHECK (percentage >= 0 AND percentage <= 100);
ALTER TABLE payment_methods ADD CONSTRAINT chk_exp_month_valid CHECK (exp_month >= 1 AND exp_month <= 12);
ALTER TABLE payment_methods ADD CONSTRAINT chk_exp_year_valid CHECK (exp_year >= EXTRACT(YEAR FROM NOW()));

-- Ensure at least one token type is available
ALTER TABLE payment_methods ADD CONSTRAINT chk_token_available 
    CHECK (
        (token_type = 'network' AND network_token IS NOT NULL) OR
        (token_type = 'dual_vault' AND processor_a_token IS NOT NULL AND processor_b_token IS NOT NULL)
    );

-- Comments for documentation
COMMENT ON TABLE subscriptions IS 'User subscription agreements with billing details';
COMMENT ON TABLE payment_methods IS 'Tokenized payment methods supporting network tokens and dual vault';
COMMENT ON TABLE transactions IS 'All payment attempts, charges, and refunds with processor tracking';
COMMENT ON TABLE routing_rules IS 'BPAS business rules for dynamic payment routing';
COMMENT ON TABLE processor_health IS 'Real-time processor status for circuit breaker logic';

COMMENT ON COLUMN payment_methods.network_token IS 'Portable token that works across all processors (95% of cases)';
COMMENT ON COLUMN payment_methods.processor_a_token IS 'Processor A specific token for dual vault fallback';
COMMENT ON COLUMN payment_methods.processor_b_token IS 'Processor B specific token for dual vault fallback';
COMMENT ON COLUMN transactions.idempotency_key IS 'Unique key to prevent duplicate charges';
COMMENT ON COLUMN transactions.original_transaction_id IS 'Reference to original transaction for refunds';
COMMENT ON COLUMN routing_rules.condition_value IS 'JSON configuration for rule conditions';

-- Insert initial system data
INSERT INTO processor_health (processor_name, is_healthy, success_rate, avg_response_time_ms)
VALUES 
    ('processor_a', true, 80.00, 250),
    ('processor_b', true, 90.00, 300)
ON CONFLICT (processor_name) DO NOTHING;

-- Insert default routing rules
INSERT INTO routing_rules (rule_name, priority, condition_type, condition_value, target_processor, percentage, is_active)
VALUES 
    ('high_value_transactions', 1, 'amount_threshold', '{"amount": 1000, "operator": "greater_than"}', 'processor_a', 100, true),
    ('premium_plan_routing', 2, 'plan_based', '{"plans": ["premium_monthly", "premium_yearly", "enterprise_yearly"]}', 'processor_a', 80, true),
    ('default_primary_split', 10, 'percentage', '{}', 'processor_a', 70, true),
    ('default_secondary_split', 11, 'percentage', '{}', 'processor_b', 30, true),
    ('processor_b_failover', 20, 'failover', '{"primary_processor": "processor_a"}', 'processor_b', 100, true)
ON CONFLICT (rule_name) DO NOTHING;