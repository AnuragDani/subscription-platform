-- Demo data for payment orchestration prototype
-- This migration adds sample data for testing and demonstrations

-- Insert demo users and subscriptions
INSERT INTO subscriptions (id, user_id, status, plan_id, amount, currency, billing_cycle, next_billing_date)
VALUES 
    ('550e8400-e29b-41d4-a716-446655440001', '550e8400-e29b-41d4-a716-446655440101', 'active', 'premium_monthly', 999, 'USD', 'monthly', NOW() + INTERVAL '30 days'),
    ('550e8400-e29b-41d4-a716-446655440002', '550e8400-e29b-41d4-a716-446655440102', 'active', 'basic_monthly', 499, 'USD', 'monthly', NOW() + INTERVAL '30 days'),
    ('550e8400-e29b-41d4-a716-446655440003', '550e8400-e29b-41d4-a716-446655440103', 'active', 'enterprise_yearly', 9999, 'USD', 'yearly', NOW() + INTERVAL '365 days'),
    ('550e8400-e29b-41d4-a716-446655440004', '550e8400-e29b-41d4-a716-446655440104', 'active', 'premium_yearly', 9999, 'USD', 'yearly', NOW() + INTERVAL '365 days'),
    ('550e8400-e29b-41d4-a716-446655440005', '550e8400-e29b-41d4-a716-446655440105', 'cancelled', 'basic_monthly', 499, 'USD', 'monthly', NULL)
ON CONFLICT (id) DO NOTHING;

-- Insert demo payment methods (mix of network tokens and dual vault)
INSERT INTO payment_methods (id, user_id, network_token, processor_a_token, processor_b_token, token_type, last_four, exp_month, exp_year)
VALUES 
    -- Network token examples (95% of cases)
    ('550e8400-e29b-41d4-a716-446655440201', '550e8400-e29b-41d4-a716-446655440101', 'ntk_demo_1234567890abcdef', null, null, 'network', '4242', 12, 2025),
    ('550e8400-e29b-41d4-a716-446655440203', '550e8400-e29b-41d4-a716-446655440103', 'ntk_demo_9876543210fedcba', null, null, 'network', '9999', 3, 2027),
    ('550e8400-e29b-41d4-a716-446655440204', '550e8400-e29b-41d4-a716-446655440104', 'ntk_demo_1111222233334444', null, null, 'network', '0123', 9, 2028),
    
    -- Dual vault examples (5% fallback cases)
    ('550e8400-e29b-41d4-a716-446655440202', '550e8400-e29b-41d4-a716-446655440102', null, 'tok_a_demo_5678901234567890', 'tok_b_demo_5678901234567890', 'dual_vault', '1111', 6, 2026),
    ('550e8400-e29b-41d4-a716-446655440205', '550e8400-e29b-41d4-a716-446655440105', null, 'tok_a_demo_9999888877776666', 'tok_b_demo_9999888877776666', 'dual_vault', '5678', 1, 2025)
ON CONFLICT (id) DO NOTHING;

-- Insert historical transactions for demo (mix of processors and outcomes)
INSERT INTO transactions (id, subscription_id, payment_method_id, processor_used, amount, currency, status, transaction_type, idempotency_key, processor_transaction_id)
VALUES 
    -- Successful charges
    ('550e8400-e29b-41d4-a716-446655440301', '550e8400-e29b-41d4-a716-446655440001', '550e8400-e29b-41d4-a716-446655440201', 'processor_a', 999, 'USD', 'success', 'charge', 'idem_1_initial_charge_2024_01', 'txn_a_1234567890abcdef'),
    ('550e8400-e29b-41d4-a716-446655440302', '550e8400-e29b-41d4-a716-446655440002', '550e8400-e29b-41d4-a716-446655440202', 'processor_b', 499, 'USD', 'success', 'charge', 'idem_2_initial_charge_2024_01', 'txn_b_2345678901bcdefg'),
    ('550e8400-e29b-41d4-a716-446655440303', '550e8400-e29b-41d4-a716-446655440003', '550e8400-e29b-41d4-a716-446655440203', 'processor_a', 9999, 'USD', 'success', 'charge', 'idem_3_initial_charge_2024_01', 'txn_a_3456789012cdefgh'),
    
    -- Recurring charges (MIT)
    ('550e8400-e29b-41d4-a716-446655440304', '550e8400-e29b-41d4-a716-446655440001', '550e8400-e29b-41d4-a716-446655440201', 'processor_a', 999, 'USD', 'success', 'charge', 'idem_1_recurring_2024_02', 'txn_a_4567890123defghi'),
    ('550e8400-e29b-41d4-a716-446655440305', '550e8400-e29b-41d4-a716-446655440002', '550e8400-e29b-41d4-a716-446655440202', 'processor_b', 499, 'USD', 'success', 'charge', 'idem_2_recurring_2024_02', 'txn_b_5678901234efghij'),
    
    -- Failed transaction (shows failover scenario)
    ('550e8400-e29b-41d4-a716-446655440306', '550e8400-e29b-41d4-a716-446655440005', '550e8400-e29b-41d4-a716-446655440205', 'processor_a', 499, 'USD', 'failed', 'charge', 'idem_5_failed_charge_2024_01', null),
    
    -- Refund example
    ('550e8400-e29b-41d4-a716-446655440307', '550e8400-e29b-41d4-a716-446655440005', '550e8400-e29b-41d4-a716-446655440205', 'processor_a', 50, 'USD', 'success', 'refund', 'idem_refund_partial_001', 'ref_a_abcdef1234567890')
ON CONFLICT (id) DO NOTHING;

-- Update the refund to reference its original transaction
UPDATE transactions 
SET original_transaction_id = '550e8400-e29b-41d4-a716-446655440301'
WHERE id = '550e8400-e29b-41d4-a716-446655440307';

-- Add some realistic error messages to failed transactions
UPDATE transactions 
SET error_code = 'CARD_DECLINED', 
    error_message = 'Payment declined by issuing bank'
WHERE status = 'failed';

-- Display summary of seeded data
DO $$
DECLARE
    sub_count INTEGER;
    pm_count INTEGER;
    txn_count INTEGER;
    rule_count INTEGER;
    health_count INTEGER;
BEGIN
    SELECT COUNT(*) INTO sub_count FROM subscriptions;
    SELECT COUNT(*) INTO pm_count FROM payment_methods;
    SELECT COUNT(*) INTO txn_count FROM transactions;
    SELECT COUNT(*) INTO rule_count FROM routing_rules;
    SELECT COUNT(*) INTO health_count FROM processor_health;
    
    RAISE NOTICE '🌱 Demo data seeded successfully:';
    RAISE NOTICE '  • % subscriptions (% active)', sub_count, (SELECT COUNT(*) FROM subscriptions WHERE status = 'active');
    RAISE NOTICE '  • % payment methods (% network, % dual vault)', pm_count, 
        (SELECT COUNT(*) FROM payment_methods WHERE token_type = 'network'),
        (SELECT COUNT(*) FROM payment_methods WHERE token_type = 'dual_vault');
    RAISE NOTICE '  • % transactions (% successful)', txn_count, (SELECT COUNT(*) FROM transactions WHERE status = 'success');
    RAISE NOTICE '  • % routing rules', rule_count;
    RAISE NOTICE '  • % processor health records', health_count;
    RAISE NOTICE '';
    RAISE NOTICE '💡 Demo user IDs: 550e8400-e29b-41d4-a716-446655440101 to 446655440105';
END
$$;