# Database Schema Documentation

## Overview
The payment orchestration system uses PostgreSQL with a carefully designed schema that supports:
- Network token portability (95% of cases)
- Dual vault fallback (5% of cases) 
- Automatic processor failover
- MIT (Merchant Initiated Transactions)
- BPAS dynamic routing
- Complete audit trail

## Core Tables

### `subscriptions`
User billing agreements with plan details and billing cycles.

| Column | Type | Description |
|--------|------|-------------|
| `id` | UUID | Primary key |
| `user_id` | UUID | User identifier |
| `status` | VARCHAR(50) | active, cancelled, expired, pending, failed |
| `plan_id` | VARCHAR(100) | Plan identifier (premium_monthly, etc.) |
| `amount` | DECIMAL(10,2) | Subscription amount in cents |
| `currency` | VARCHAR(3) | ISO currency code |
| `billing_cycle` | VARCHAR(20) | monthly, yearly |
| `next_billing_date` | TIMESTAMP | When next MIT charge is due |

**Key Indexes:**
- `idx_subscriptions_status_billing` - For MIT scheduler efficiency
- `idx_subscriptions_user_id` - User lookups

### `payment_methods`
Tokenized payment information supporting both network tokens and dual vault.

| Column | Type | Description |
|--------|------|-------------|
| `id` | UUID | Primary key |
| `user_id` | UUID | Owner of payment method |
| `network_token` | VARCHAR(255) | Portable token (works with any processor) |
| `processor_a_token` | VARCHAR(255) | Processor A specific token |
| `processor_b_token` | VARCHAR(255) | Processor B specific token |
| `token_type` | VARCHAR(50) | 'network' or 'dual_vault' |
| `last_four` | VARCHAR(4) | Last 4 digits for display |
| `exp_month` | INTEGER | Expiration month |
| `exp_year` | INTEGER | Expiration year |

**Token Strategy:**
- **95% Network Tokens**: Portable across processors, enable seamless failover
- **5% Dual Vault**: When network tokens unavailable, store tokens for both processors

### `transactions` 
Complete record of all payment attempts, charges, and refunds.

| Column | Type | Description |
|--------|------|-------------|
| `id` | UUID | Primary key |
| `subscription_id` | UUID | Related subscription |
| `payment_method_id` | UUID | Payment method used |
| `processor_used` | VARCHAR(50) | 'processor_a' or 'processor_b' |
| `amount` | DECIMAL(10,2) | Transaction amount |
| `status` | VARCHAR(50) | pending, success, failed |
| `transaction_type` | VARCHAR(50) | charge, refund |
| `idempotency_key` | VARCHAR(255) | Prevents duplicate charges |
| `processor_transaction_id` | VARCHAR(255) | Processor's transaction ID |
| `original_transaction_id` | UUID | For refunds, points to original charge |

**Key Features:**
- Idempotency keys prevent duplicate charges during retries
- Tracks which processor handled each transaction
- Refunds always route to original processor via `original_transaction_id`

### `routing_rules`
BPAS (Business Profile Authority Service) configuration for dynamic routing.

| Column | Type | Description |
|--------|------|-------------|
| `rule_name` | VARCHAR(100) | Unique rule identifier |
| `priority` | INTEGER | Lower number = higher priority |
| `condition_type` | VARCHAR(50) | amount_threshold, plan_based, percentage, failover |
| `condition_value` | JSONB | Flexible condition configuration |
| `target_processor` | VARCHAR(50) | Which processor to route to |
| `percentage` | INTEGER | Traffic percentage for this rule |

**Example Rules:**
```sql
-- High value transactions to Processor A
{
  "rule_name": "high_value_transactions",
  "condition_type": "amount_threshold", 
  "condition_value": {"amount": 1000, "operator": "greater_than"},
  "target_processor": "processor_a"
}

-- Premium plans get priority routing
{
  "rule_name": "premium_plan_routing",
  "condition_type": "plan_based",
  "condition_value": {"plans": ["premium_monthly", "enterprise_yearly"]},
  "target_processor": "processor_a"
}
```

### `processor_health`
Real-time processor status for circuit breaker logic.

| Column | Type | Description |
|--------|------|-------------|
| `processor_name` | VARCHAR(50) | processor_a, processor_b |
| `is_healthy` | BOOLEAN | Current health status |
| `success_rate` | DECIMAL(5,2) | Recent success percentage |
| `avg_response_time_ms` | INTEGER | Average response time |
| `failure_count` | INTEGER | Consecutive failures |

## Data Flow Examples

### 1. New Subscription Creation
```sql
-- 1. Create payment method (95% get network token)
INSERT INTO payment_methods (user_id, network_token, token_type) 
VALUES ('user123', 'ntk_abc123', 'network');

-- 2. Create subscription
INSERT INTO subscriptions (user_id, plan_id, amount) 
VALUES ('user123', 'premium_monthly', 999);

-- 3. Process initial payment
INSERT INTO transactions (subscription_id, processor_used, status, idempotency_key)
VALUES ('sub123', 'processor_a', 'success', 'idem_initial_abc123');
```

### 2. MIT Recurring Billing
```sql
-- MIT scheduler finds due subscriptions
SELECT s.*, pm.network_token, pm.token_type 
FROM subscriptions s
JOIN payment_methods pm ON pm.user_id = s.user_id 
WHERE s.status = 'active' 
AND s.next_billing_date <= NOW();

-- Process renewal without user interaction
INSERT INTO transactions (subscription_id, processor_used, transaction_type, idempotency_key)
VALUES ('sub123', 'processor_a', 'charge', 'idem_mit_202501_sub123');
```

### 3. Refund to Original Processor
```sql
-- Find original transaction
SELECT processor_used, processor_transaction_id 
FROM transactions 
WHERE id = 'txn_to_refund';

-- Create refund routed to same processor
INSERT INTO transactions (
  original_transaction_id, processor_used, transaction_type, amount
) VALUES (
  'txn_to_refund', 'processor_a', 'refund', -999
);
```

## Performance Considerations

### Indexes
- **Composite indexes** for common query patterns
- **Partial indexes** for filtered queries (active subscriptions only)
- **JSONB indexes** on routing rule conditions for fast BPAS lookups

### Scaling Strategy
1. **Vertical scaling** first (more CPU/RAM)
2. **Read replicas** for reporting queries  
3. **Horizontal sharding** by `user_id` when exceeding 50M users
4. **Archival strategy** for old transactions (>2 years)

## Security & Compliance

### PCI Compliance
- **No raw card data** stored anywhere
- **Tokenized references** only in payment_methods table  
- **Audit trail** of all transactions
- **Encrypted at rest** (PostgreSQL TDE)

### Data Retention
- **Active subscriptions**: Indefinite retention
- **Cancelled subscriptions**: 90 days then anonymize
- **Transaction records**: 7 years for tax compliance
- **Audit logs**: 10 years for dispute resolution

## Migration Strategy

### Schema Evolution
- **Backward compatible** changes preferred
- **Feature flags** for new functionality
- **Blue-green deployments** for major changes
- **Data migrations** run during maintenance windows

### Version History
- `001_initial_schema.sql` - Core tables and constraints
- `002_seed_demo_data.sql` - Sample data for development
- Future migrations will be numbered sequentially

This schema provides a solid foundation for the payment orchestration system while maintaining flexibility for future enhancements.