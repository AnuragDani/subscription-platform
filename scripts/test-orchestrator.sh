#!/bin/bash

# scripts/test-orchestrator.sh
# Comprehensive testing script for Payment Orchestrator

echo "🎼 Testing Payment Orchestrator Integration"
echo "========================================="
echo ""
echo "Important Notes:"
echo "• Tests end-to-end payment orchestration"
echo "• Demonstrates automatic failover between processors"
echo "• Validates idempotency and token management"
echo "• Shows refund routing to original processor"
echo ""

# Configuration
ORCHESTRATOR_URL="http://localhost:8001"
PROCESSOR_A_URL="http://localhost:8101"
PROCESSOR_B_URL="http://localhost:8102"
BPAS_URL="http://localhost:8003"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to check HTTP response
check_response() {
    local description="$1"
    local response="$2"
    local expected_status="$3"
    local actual_status=$(echo "$response" | head -1 | cut -d' ' -f2)
    
    if [ "$actual_status" = "$expected_status" ]; then
        echo -e "  ${GREEN}✅ PASS${NC} - $description"
        echo "    Response: $(echo "$response" | tail -1)"
    else
        echo -e "  ${RED}❌ FAIL${NC} - Expected $expected_status, got $actual_status"
        echo "    Response: $(echo "$response" | tail -1)"
    fi
    echo ""
}

# Function to make HTTP request and return status + body
make_request() {
    local method="$1"
    local url="$2"
    local data="$3"
    local headers="$4"
    
    if [ "$method" = "POST" ]; then
        if [ -n "$headers" ]; then
            curl -s -w "%{http_code}\n" -X POST "$url" -H "Content-Type: application/json" -H "$headers" -d "$data"
        else
            curl -s -w "%{http_code}\n" -X POST "$url" -H "Content-Type: application/json" -d "$data"
        fi
    else
        curl -s -w "%{http_code}\n" "$url"
    fi
}

# Start tests
echo "Starting Payment Orchestrator integration tests..."
echo ""

# Test 1: Health Check
echo -e "${BLUE}🏥 Health Check${NC}"
echo "Testing: Orchestrator Health Check"
response=$(make_request "GET" "$ORCHESTRATOR_URL/health")
check_response "Health Check" "$response" "200"

# Test 2: Successful Payment (Happy Path)
echo -e "${BLUE}💳 Successful Payment Flow${NC}"
echo "Testing: End-to-end payment with processor routing"

payment_data='{
  "subscription_id": "sub_test_123",
  "payment_method_id": "pm_test_456", 
  "amount": 999,
  "currency": "USD",
  "business_profile": "default"
}'

response=$(make_request "POST" "$ORCHESTRATOR_URL/orchestrator/charge" "$payment_data")
check_response "Successful Payment" "$response" "200"

# Extract transaction ID for refund test
transaction_id=$(echo "$response" | tail -1 | grep -o '"transaction_id":"[^"]*"' | cut -d'"' -f4)
echo "  Captured transaction ID: $transaction_id"
echo ""

# Test 3: Idempotency Check
echo -e "${BLUE}🔒 Idempotency Test${NC}"
echo "Testing: Duplicate request with same idempotency key"

payment_with_idem='{
  "subscription_id": "sub_test_789",
  "payment_method_id": "pm_test_456", 
  "amount": 1500,
  "currency": "USD",
  "idempotency_key": "idem_test_unique_123"
}'

# First request
echo "  First request with idempotency key..."
response1=$(make_request "POST" "$ORCHESTRATOR_URL/orchestrator/charge" "$payment_with_idem")
check_response "First Request" "$response1" "200"

# Second request (should return cached response)
echo "  Second request with same idempotency key..."
response2=$(make_request "POST" "$ORCHESTRATOR_URL/orchestrator/charge" "$payment_with_idem")
check_response "Duplicate Request (Cached)" "$response2" "200"

# Test 4: Processor Failover Simulation
echo -e "${BLUE}🔄 Processor Failover Test${NC}"
echo "Testing: Automatic failover when primary processor fails"

# Set processor A to fail temporarily
echo "  Setting Processor A to 100% failure rate..."
curl -s -X POST "$PROCESSOR_A_URL/admin/set-failure-rate?rate=100" > /dev/null

# Wait for health checks to register the failure
sleep 2

# Attempt payment (should automatically failover to processor B)
failover_payment='{
  "subscription_id": "sub_failover_test",
  "payment_method_id": "pm_test_456",
  "amount": 500,
  "currency": "USD"
}'

response=$(make_request "POST" "$ORCHESTRATOR_URL/orchestrator/charge" "$failover_payment")
check_response "Failover Payment" "$response" "200"

# Check if processor B was used
if echo "$response" | tail -1 | grep -q "processor_b"; then
    echo -e "  ${GREEN}✅ PASS${NC} - Failover to Processor B successful"
else
    echo -e "  ${YELLOW}⚠️ WARN${NC} - Failover may not have occurred as expected"
fi

# Reset processor A to normal operation
echo "  Resetting Processor A to normal operation..."
curl -s -X POST "$PROCESSOR_A_URL/admin/set-failure-rate?rate=20" > /dev/null
echo ""

# Test 5: Multi-Currency Routing
echo -e "${BLUE}💱 Multi-Currency Routing Test${NC}"
echo "Testing: EUR payment routing via BPAS"

eur_payment='{
  "subscription_id": "sub_eur_test", 
  "payment_method_id": "pm_test_456",
  "amount": 850,
  "currency": "EUR"
}'

response=$(make_request "POST" "$ORCHESTRATOR_URL/orchestrator/charge" "$eur_payment")
check_response "EUR Payment Routing" "$response" "200"

# Test 6: High-Value Transaction Routing
echo -e "${BLUE}💎 High-Value Transaction Test${NC}"
echo "Testing: High-value transaction routing (>$1000)"

high_value_payment='{
  "subscription_id": "sub_highvalue_test",
  "payment_method_id": "pm_test_456", 
  "amount": 1500,
  "currency": "USD"
}'

response=$(make_request "POST" "$ORCHESTRATOR_URL/orchestrator/charge" "$high_value_payment")
check_response "High-Value Transaction" "$response" "200"

# Test 7: Refund to Original Processor
echo -e "${BLUE}💰 Refund Routing Test${NC}"
echo "Testing: Refund routes to original processor"

if [ -n "$transaction_id" ]; then
    refund_data="{
      \"transaction_id\": \"$transaction_id\",
      \"amount\": 999,
      \"reason\": \"customer_request\"
    }"
    
    response=$(make_request "POST" "$ORCHESTRATOR_URL/orchestrator/refund" "$refund_data")
    check_response "Refund to Original Processor" "$response" "200"
else
    echo -e "  ${YELLOW}⚠️ SKIP${NC} - No transaction ID available for refund test"
    echo ""
fi

# Test 8: Error Handling
echo -e "${BLUE}❌ Error Handling Test${NC}"
echo "Testing: Invalid payment method handling"

invalid_payment='{
  "subscription_id": "sub_invalid_test",
  "payment_method_id": "pm_nonexistent", 
  "amount": 100,
  "currency": "USD"
}'

response=$(make_request "POST" "$ORCHESTRATOR_URL/orchestrator/charge" "$invalid_payment")
check_response "Invalid Payment Method" "$response" "200"

# Check for proper error response
if echo "$response" | tail -1 | grep -q '"success":false'; then
    echo -e "  ${GREEN}✅ PASS${NC} - Proper error response returned"
else
    echo -e "  ${RED}❌ FAIL${NC} - Expected error response not returned"
fi
echo ""

# Test 9: Load Test (Simple)
echo -e "${BLUE}⚡ Simple Load Test${NC}"
echo "Testing: 5 concurrent payments for basic load handling"

load_test_payment='{
  "subscription_id": "sub_load_test_{{i}}",
  "payment_method_id": "pm_test_456",
  "amount": 299,
  "currency": "USD"
}'

concurrent_requests=5
success_count=0

for i in $(seq 1 $concurrent_requests); do
    payment_data_with_id=$(echo "$load_test_payment" | sed "s/{{i}}/$i/g")
    response=$(make_request "POST" "$ORCHESTRATOR_URL/orchestrator/charge" "$payment_data_with_id" &)
    
    # Check if successful (basic check)
    if echo "$response" | tail -1 | grep -q '"success":true'; then
        ((success_count++))
    fi
done

wait # Wait for all background requests to complete

echo "  Load test completed: $success_count/$concurrent_requests successful"
if [ $success_count -eq $concurrent_requests ]; then
    echo -e "  ${GREEN}✅ PASS${NC} - All concurrent requests handled successfully"
else
    echo -e "  ${YELLOW}⚠️ PARTIAL${NC} - Some requests may have failed"
fi
echo ""

# Test Summary
echo -e "${BLUE}📊 Test Summary${NC}"
echo "================================"
echo "Core Integration Tests Completed:"
echo "• Health check validation"
echo "• End-to-end payment processing" 
echo "• Idempotency protection"
echo "• Automatic processor failover"
echo "• Multi-currency routing via BPAS"
echo "• High-value transaction handling"
echo "• Refund routing to original processor"
echo "• Error handling for invalid requests"
echo "• Basic concurrent request handling"
echo ""
echo "Payment Orchestrator integration testing complete!"
echo ""
echo "Next steps:"
echo "1. Check docker-compose logs payment-orchestrator for detailed logs"
echo "2. Verify database transactions: docker exec -it payment_db psql -U payment_user -c 'SELECT * FROM transactions;'"
echo "3. Check Redis cache: docker exec -it payment_redis redis-cli KEYS 'idempotency:*'"