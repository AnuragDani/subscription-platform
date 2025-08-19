# Mock Payment Processors Documentation

## Overview

This commit adds two realistic mock payment processors that simulate real payment gateway behavior:

- **Processor A (Primary)**: 80% success rate, 250ms response time, USD focused
- **Processor B (Secondary)**: 90% success rate, 300ms response time, multi-currency support

## Features

### Core Payment Operations
- **Charge processing** with configurable success rates
- **Refund handling** with original processor validation  
- **Card tokenization** for PCI compliance
- **Multi-currency support** (Processor B)
- **Network token simulation** for portability

### Admin & Testing Features
- **Configurable failure rates** for testing failover scenarios
- **Health status toggle** to simulate outages
- **Artificial latency injection** (Processor B)
- **Real-time statistics** for monitoring
- **Idempotency support** to prevent duplicate charges

## Quick Start

### 1. Start the Processors

```bash
# Start both processors
docker-compose up mock-processor-a mock-processor-b -d

# Check health
curl http://localhost:8101/health  # Processor A
curl http://localhost:8102/health  # Processor B
```

### 2. Test Basic Operations

```bash
# Process a charge on Processor A
curl -X POST http://localhost:8101/charge \
  -H "Content-Type: application/json" \
  -d '{
    "amount": 1000,
    "currency": "USD", 
    "network_token": "ntk_test_1234567890",
    "idempotency_key": "test_charge_123"
  }'

# Process a multi-currency charge on Processor B
curl -X POST http://localhost:8102/charge \
  -H "Content-Type: application/json" \
  -d '{
    "amount": 850,
    "currency": "EUR",
    "network_token": "ntk_test_eur_1234567890", 
    "idempotency_key": "test_eur_charge_123"
  }'
```

### 3. Run Full Test Suite

```bash
# Make script executable
chmod +x scripts/test-processors.sh

# Run comprehensive tests
./scripts/test-processors.sh
```

## API Reference

### Core Endpoints

#### POST /charge
Process a payment charge.

**Request:**
```json
{
  "amount": 1000,
  "currency": "USD",
  "network_token": "ntk_1234567890",
  "processor_token": "tok_a_4242_abcd1234", 
  "idempotency_key": "unique_key_123"
}
```

**Response (Success):**
```json
{
  "success": true,
  "transaction_id": "txn_a_12345678",
  "auth_code": "auth_789456",
  "processor_used": "processor_a",
  "token_type": "network"
}
```

**Response (Error):**
```json
{
  "success": false,
  "error_code": "CARD_DECLINED",
  "error_message": "Payment declined by issuing bank",
  "processor_used": "processor_a"
}
```

#### POST /refund
Process a refund for an existing transaction.

**Request:**
```json
{
  "original_transaction_id": "txn_a_12345678",
  "amount": 500,
  "currency": "USD",
  "reason": "customer_request",
  "idempotency_key": "refund_key_456"
}
```

#### POST /tokenize  
Create a payment token from card details.

**Request:**
```json
{
  "card_number": "4242424242424242",
  "exp_month": 12,
  "exp_year": 2026,
  "cvv": "123"
}
```

### Admin Endpoints

#### POST /admin/set-failure-rate?rate={percentage}
Set the processor failure rate for testing.

```bash
# Set 30% failure rate
curl -X POST http://localhost:8101/admin/set-failure-rate?rate=30
```

#### POST /admin/toggle-status
Toggle processor health status (healthy ↔ unhealthy).

```bash
curl -X POST http://localhost:8101/admin/toggle-status
```

#### POST /admin/set-latency?ms={milliseconds}
Set artificial latency (Processor B only).

```bash
# Add 2 second delay
curl -X POST http://localhost:8102/admin/set-latency?ms=2000
```

#### GET /admin/stats
Get processor performance statistics.

**Response:**
```json
{
  "processor_name": "processor_a",
  "is_healthy": true,
  "failure_rate": 20.0,
  "stats": {
    "total_requests": 150,
    "successful_charges": 120,
    "failed_charges": 30,
    "refunds": 5,
    "success_rate": 80.0,
    "avg_response_time_ms": 250
  },
  "timestamp": "2025-01-XX..."
}
```

## Processor Characteristics

### Processor A (Primary)
- **Success Rate**: 80% (20% failure rate)
- **Response Time**: 250ms average
- **Currencies**: Primarily USD, basic support for others
- **Strengths**: Faster response time, lower fees
- **Weaknesses**: Higher failure rate, limited international support

**Typical Error Codes:**
- `CARD_DECLINED` - Payment declined by bank
- `INSUFFICIENT_FUNDS` - Insufficient funds
- `CARD_EXPIRED` - Card has expired
- `NETWORK_ERROR` - Network connectivity issue
- `TIMEOUT` - Request timeout

### Processor B (Secondary/Backup)
- **Success Rate**: 90% (10% failure rate)  
- **Response Time**: 300ms average
- **Currencies**: USD, EUR, GBP, JPY, AUD, CAD, CHF, SEK, NOK, DKK
- **Strengths**: Higher reliability, excellent multi-currency support
- **Weaknesses**: Slightly slower, higher fees

**Additional Features:**
- Exchange rate simulation for non-USD currencies
- Enhanced foreign card support
- Better refund success rate (98% vs 95%)

**Typical Error Codes:**
- `FOREIGN_CARD_DECLINED` - Foreign card declined
- `CURRENCY_CONVERSION_FAILED` - Currency conversion issue
- `RATE_LIMITED` - Too many requests

## Testing Scenarios

### 1. Happy Path Testing
```bash
# Test successful charge → refund flow
./scripts/test-processors.sh
```

### 2. Failover Simulation
```bash
# Make Processor A unhealthy
curl -X POST http://localhost:8101/admin/toggle-status

# Verify health status
curl http://localhost:8101/health  # Should return 503

# Restore health
curl -X POST http://localhost:8101/admin/toggle-status
```

### 3. Multi-Currency Testing
```bash
# Test different currencies on Processor B
for currency in EUR GBP JPY; do
  curl -X POST http://localhost:8102/charge \
    -H "Content-Type: application/json" \
    -d "{
      \"amount\": 1000,
      \"currency\": \"$currency\",
      \"network_token\": \"ntk_test_$currency\",
      \"idempotency_key\": \"test_${currency}_$(date +%s)\"
    }"
done
```

### 4. Latency Testing
```bash
# Add artificial latency to Processor B
curl -X POST http://localhost:8102/admin/set-latency?ms=3000

# Test charge (should take ~3 seconds)
time curl -X POST http://localhost:8102/charge \
  -H "Content-Type: application/json" \
  -d '{
    "amount": 1000,
    "currency": "USD",
    "network_token": "ntk_latency_test",
    "idempotency_key": "latency_test_123"
  }'

# Reset latency
curl -X POST http://localhost:8102/admin/set-latency?ms=300
```

### 5. Error Rate Testing
```bash
# Increase failure rate to test error handling
curl -X POST http://localhost:8101/admin/set-failure-rate?rate=80

# Make multiple requests to see various error types
for i in {1..10}; do
  curl -X POST http://localhost:8101/charge \
    -H "Content-Type: application/json" \
    -d "{
      \"amount\": 1000,
      \"currency\": \"USD\",
      \"network_token\": \"ntk_error_test_$i\",
      \"idempotency_key\": \"error_test_$i\"
    }"
done

# Reset to normal failure rate
curl -X POST http://localhost:8101/admin/set-failure-rate?rate=20
```

## Integration with Payment Orchestrator

The processors are designed to work with the payment orchestrator:

1. **Health Monitoring**: Orchestrator can check `/health` endpoints
2. **Failover Logic**: When Processor A fails, orchestrator switches to B
3. **Token Portability**: Network tokens work on both processors
4. **Refund Routing**: Refunds automatically route to original processor
5. **Error Handling**: Structured error responses enable smart retry logic

## Monitoring & Observability

### Health Checks
- Both processors expose `/health` endpoints
- Health status can be toggled via admin API for testing
- Docker Compose includes health check configuration

### Performance Metrics
- Request counts (total, successful, failed)
- Success rates calculated in real-time  
- Average response times
- Refund statistics
- Multi-currency transaction counts (Processor B)

### Error Tracking
- Structured error codes for better handling
- Error messages designed for both debugging and user display
- Processor-specific error patterns


The processors provide a realistic foundation for testing all payment orchestration features without needing real payment gateways.