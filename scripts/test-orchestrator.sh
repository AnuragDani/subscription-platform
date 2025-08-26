#!/bin/bash

echo "🧪 Testing Payment Orchestrator"
echo "================================"
echo ""

BASE_URL="http://localhost:8001"

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Use the seeded UUIDs
PAYMENT_METHOD_ID="323e4567-e89b-12d3-a456-426614174000"  # pm_manual_test equivalent
SUBSCRIPTION_ID="723e4567-e89b-12d3-a456-426614174000"    # sub_manual_test equivalent
FAILOVER_PM_ID="423e4567-e89b-12d3-a456-426614174000"     # pm_failover_test equivalent
FAILOVER_SUB_ID="823e4567-e89b-12d3-a456-426614174000"    # sub_failover_test equivalent


# Function to test endpoint
test_endpoint() {
    local name="$1"
    local method="$2"
    local url="$3"
    local data="$4"
    local expected_status="$5"
    
    echo -n "Testing: $name... "
    
    if [ "$method" = "GET" ]; then
        response=$(curl -s -o /dev/null -w "%{http_code}" "$url")
    else
        response=$(curl -s -o /dev/null -w "%{http_code}" -X "$method" \
            -H "Content-Type: application/json" \
            -d "$data" "$url")
    fi
    
    if [ "$response" = "$expected_status" ]; then
        echo -e "${GREEN}✅ PASS${NC} - HTTP $response"
    else
        echo -e "${RED}❌ FAIL${NC} - Expected $expected_status, got $response"
    fi
}

# Check if orchestrator is running
echo "🔍 Checking if Payment Orchestrator is running..."
if curl -s "$BASE_URL/health" > /dev/null; then
    echo -e "${GREEN}✅ Payment Orchestrator is running${NC}"
else
    echo -e "${RED}❌ Payment Orchestrator is not running${NC}"
    echo "Please start it with: docker-compose up payment-orchestrator -d"
    exit 1
fi

echo ""
echo "📋 Test Suite"
echo "============="
echo ""

# 1. Health Check
test_endpoint "Health Check" "GET" "$BASE_URL/health" "" "200"

# 2. Successful Payment with seeded data
echo ""
echo "💳 Testing Payment Processing"
charge_data='{
  "subscription_id": "'$SUBSCRIPTION_ID'",
  "payment_method_id": "'$PAYMENT_METHOD_ID'",
  "amount": 1999,
  "currency": "USD"
}'
test_endpoint "Process Payment" "POST" "$BASE_URL/orchestrator/charge" "$charge_data" "201"

# 3. Idempotency Test
echo ""
echo "🔄 Testing Idempotency"
idempotency_key="idem_test_$RANDOM"
idem_data='{
  "subscription_id": "sub_test_123",
  "payment_method_id": "pm_test_456",
  "amount": 500,
  "currency": "USD",
  "idempotency_key": "'$idempotency_key'"
}'

echo "First request..."
first_response=$(curl -s -X POST "$BASE_URL/orchestrator/charge" \
    -H "Content-Type: application/json" \
    -d "$idem_data")
echo "Response: $first_response"

echo "Duplicate request with same idempotency key..."
second_response=$(curl -s -X POST "$BASE_URL/orchestrator/charge" \
    -H "Content-Type: application/json" \
    -d "$idem_data")

if [ "$first_response" = "$second_response" ]; then
    echo -e "${GREEN}✅ Idempotency working - duplicate prevented${NC}"
else
    echo -e "${YELLOW}⚠️ Different responses for same idempotency key${NC}"
fi

# 4. Processor Failover Test
echo ""
echo "🔄 Testing Processor Failover"
echo "Simulating Processor A failure..."

# First, make Processor A fail
curl -s -X POST "http://localhost:8101/admin/set-failure-rate?rate=100" > /dev/null

# Try payment (should failover to Processor B)
failover_data='{
  "subscription_id": "sub_failover_test",
  "payment_method_id": "pm_failover_test",
  "amount": 750,
  "currency": "USD"
}'

response=$(curl -s -X POST "$BASE_URL/orchestrator/charge" \
    -H "Content-Type: application/json" \
    -d "$failover_data")

if echo "$response" | grep -q "processor_b"; then
    echo -e "${GREEN}✅ Failover successful - used processor_b${NC}"
else
    echo -e "${RED}❌ Failover may have failed${NC}"
fi

# Reset Processor A
curl -s -X POST "http://localhost:8101/admin/set-failure-rate?rate=20" > /dev/null

# 5. Refund Test
echo ""
echo "💰 Testing Refunds"
refund_data='{
  "transaction_id": "txn_test_123",
  "amount": 500,
  "reason": "customer_request"
}'
test_endpoint "Process Refund" "POST" "$BASE_URL/orchestrator/refund" "$refund_data" "200"

# 6. Stats Endpoint
echo ""
echo "📊 Testing Stats"
test_endpoint "Get Stats" "GET" "$BASE_URL/admin/stats" "" "200"

echo ""
echo "================================"
echo "🏁 Test Suite Complete"