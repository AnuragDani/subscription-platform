-- Seed test payment methods
INSERT INTO payment_methods (id, user_id, network_token, processor_a_token, processor_b_token, token_type, last_four)
VALUES 
  ('pm_test_123', 'user_test_123', 'ntk_test_123', 'tok_a_123', 'tok_b_123', 'network', '4242'),
  ('pm_test_456', 'user_test_456', 'ntk_test_456', 'tok_a_456', 'tok_b_456', 'network', '5555'),
  ('pm_manual_test', 'user_manual_test', 'ntk_manual_test', 'tok_a_manual', 'tok_b_manual', 'network', '1234'),
  ('pm_failover_test', 'user_failover_test', 'ntk_failover_test', 'tok_a_failover', 'tok_b_failover', 'network', '9999')
ON CONFLICT (id) DO NOTHING;

-- Seed test subscriptions
INSERT INTO subscriptions (id, user_id, status, plan_id, amount, currency, billing_cycle)
VALUES
  ('sub_test_123', 'user_test_123', 'active', 'premium_monthly', 999, 'USD', 'monthly'),
  ('sub_test_456', 'user_test_456', 'active', 'basic_monthly', 499, 'USD', 'monthly'),
  ('sub_manual_test', 'user_manual_test', 'active', 'premium_monthly', 1999, 'USD', 'monthly'),
  ('sub_failover_test', 'user_failover_test', 'active', 'premium_monthly', 999, 'EUR', 'monthly')
ON CONFLICT (id) DO NOTHING;