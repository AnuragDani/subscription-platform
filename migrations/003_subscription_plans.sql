-- Migration 003: Subscription Plans and Enhanced Subscriptions
-- This migration adds the plans table and enhances the subscriptions table

-- Plans table - subscription plan definitions
CREATE TABLE IF NOT EXISTS plans (
    id VARCHAR(50) PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    display_name VARCHAR(100) NOT NULL,
    amount BIGINT NOT NULL, -- Amount in cents
    currency VARCHAR(3) NOT NULL DEFAULT 'USD',
    interval VARCHAR(20) NOT NULL, -- monthly, yearly
    trial_days INTEGER NOT NULL DEFAULT 0,
    features JSONB DEFAULT '[]'::jsonb,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Add missing columns to subscriptions table
ALTER TABLE subscriptions
    ADD COLUMN IF NOT EXISTS payment_method_id UUID REFERENCES payment_methods(id),
    ADD COLUMN IF NOT EXISTS current_period_start TIMESTAMP,
    ADD COLUMN IF NOT EXISTS current_period_end TIMESTAMP,
    ADD COLUMN IF NOT EXISTS cancel_at_period_end BOOLEAN DEFAULT false,
    ADD COLUMN IF NOT EXISTS canceled_at TIMESTAMP,
    ADD COLUMN IF NOT EXISTS trial_start TIMESTAMP,
    ADD COLUMN IF NOT EXISTS trial_end TIMESTAMP;

-- Indexes for plans
CREATE INDEX IF NOT EXISTS idx_plans_interval ON plans(interval);
CREATE INDEX IF NOT EXISTS idx_plans_active ON plans(is_active) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_plans_amount ON plans(amount);

-- Additional indexes for subscriptions
CREATE INDEX IF NOT EXISTS idx_subscriptions_plan_id ON subscriptions(plan_id);
CREATE INDEX IF NOT EXISTS idx_subscriptions_payment_method ON subscriptions(payment_method_id);
CREATE INDEX IF NOT EXISTS idx_subscriptions_cancel_period_end ON subscriptions(cancel_at_period_end) WHERE cancel_at_period_end = true;

-- Seed default plans
INSERT INTO plans (id, name, display_name, amount, currency, interval, trial_days, features, is_active)
VALUES
    -- Monthly plans
    ('basic_monthly', 'basic', 'Basic Monthly', 2900, 'USD', 'monthly', 14,
     '["Up to 1,000 transactions/month", "Email support", "Basic analytics", "1 team member"]'::jsonb, true),

    ('pro_monthly', 'pro', 'Pro Monthly', 7900, 'USD', 'monthly', 14,
     '["Up to 10,000 transactions/month", "Priority support", "Advanced analytics", "5 team members", "Custom webhooks"]'::jsonb, true),

    ('enterprise_monthly', 'enterprise', 'Enterprise Monthly', 19900, 'USD', 'monthly', 30,
     '["Unlimited transactions", "24/7 dedicated support", "Real-time analytics", "Unlimited team members", "Custom integrations", "SLA guarantee"]'::jsonb, true),

    -- Annual plans (with discount)
    ('basic_yearly', 'basic', 'Basic Annual', 29000, 'USD', 'yearly', 14,
     '["Up to 1,000 transactions/month", "Email support", "Basic analytics", "1 team member", "2 months free"]'::jsonb, true),

    ('pro_yearly', 'pro', 'Pro Annual', 79000, 'USD', 'yearly', 14,
     '["Up to 10,000 transactions/month", "Priority support", "Advanced analytics", "5 team members", "Custom webhooks", "2 months free"]'::jsonb, true),

    ('enterprise_yearly', 'enterprise', 'Enterprise Annual', 199000, 'USD', 'yearly', 30,
     '["Unlimited transactions", "24/7 dedicated support", "Real-time analytics", "Unlimited team members", "Custom integrations", "SLA guarantee", "2 months free"]'::jsonb, true)
ON CONFLICT (id) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    amount = EXCLUDED.amount,
    features = EXCLUDED.features,
    updated_at = NOW();

-- Comments
COMMENT ON TABLE plans IS 'Subscription plan definitions with pricing and features';
COMMENT ON COLUMN plans.amount IS 'Price in cents (e.g., 2900 = $29.00)';
COMMENT ON COLUMN plans.interval IS 'Billing interval: monthly or yearly';
COMMENT ON COLUMN plans.features IS 'JSON array of feature descriptions';
