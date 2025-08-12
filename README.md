# Payment Orchestration Prototype

A microservices-based payment orchestration system demonstrating:

- **Automatic Failover**: Processor A fails → routes to Processor B
- **Network Tokens**: 95% portable tokens, 5% dual-vault fallback  
- **Dynamic Routing**: BPAS controls traffic distribution
- **Idempotency**: Zero duplicate charges
- **Refund Routing**: Always routes to original processor
- **MIT Billing**: Merchant-initiated recurring transactions

## Quick Start

```bash
# Start all services
make dev

# Run demo scenarios
make demo

# View logs
make logs

# Clean up
make clean
```

## Architecture

```
Client → API Gateway → Subscription Service → Payment Orchestrator
                                                    ↓
                          Mock Processor A ←→ Network Tokens
                          Mock Processor B ←→ BPAS Rules
```

## Services

| Service | Port | Purpose |
|---------|------|---------|
| API Gateway | 8080 | HTTP routing & auth |
| Payment Orchestrator | 8001 | Core payment logic |
| Subscription Service | 8002 | Subscription management |
| BPAS Service | 8003 | Business routing rules |
| Mock Processor A | 8101 | Payment processor simulation |
| Mock Processor B | 8102 | Payment processor simulation |
| Network Token Service | 8103 | Token portability simulation |

## Demo Scenarios

```bash
# 1. Happy path - subscription creation
curl -X POST http://localhost:8080/subscriptions \
  -H "Content-Type: application/json" \
  -d '{"user_id":"demo_user","plan_id":"premium_monthly"}'

# 2. Processor failover
curl -X POST http://localhost:8101/admin/set-failure-rate?rate=100
curl -X POST http://localhost:8080/subscriptions \
  -H "Content-Type: application/json" \
  -d '{"user_id":"demo_user_2","plan_id":"premium_monthly"}'

# 3. Refund to original processor
curl -X POST http://localhost:8080/refunds \
  -H "Content-Type: application/json" \
  -d '{"transaction_id":"txn_123","amount":999}'
```

## Development

- **Language**: Go 1.21+
- **Database**: PostgreSQL 15
- **Cache**: Redis 7
- **Architecture**: Microservices with REST APIs
