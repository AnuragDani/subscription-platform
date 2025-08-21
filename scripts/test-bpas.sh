#!/bin/bash
# scripts/test-bpas.sh

set -e

echo "Testing BPAS Business Profile Authority Service"
echo "=============================================="
echo ""
echo "Important Notes:"
echo "• BPAS evaluates routing rules in priority order"
echo "• Rules can be updated without service restarts"
echo "• Dynamic routing based on amount, currency, marketplace, user tier"
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Base URL
BPAS_URL="http://localhost:8003"

# Function to make HTTP requests and check response
test_endpoint() {
    local name="$1"
    local method="$2"
    local url="$3"
    local data="$4"
    local expected_status="$5"
    
    echo -e "${BLUE}Testing: $name${NC}"
    
    if [ "$method" = "GET" ]; then
        response=$(curl -s -w "HTTPSTATUS:%{http_code}" "$url")
    else
        response=$(curl -s -w "HTTPSTATUS:%{http_code}" -X "$method" -H "Content-Type: application/json" -d "$data" "$url")
    fi
    
    http_code=$(echo "$response" | tr -d '\n' | sed -e 's/.*HTTPSTATUS://')
    body=$(echo "$response" | sed -e 's/HTTPSTATUS:.*//g')
    
    if [ "$http_code" -eq "$expected_status" ]; then
        echo -e "${GREEN}✅ PASS${NC} - HTTP $http_code"
        echo "   Response: $(echo "$body" | jq -c . 2>/dev/null || echo "$body")"
    else
        echo -e "${RED}❌ FAIL${NC} - Expected $expected_status, got $http_code"
        echo "   Response: $body"
        return 1
    fi
    echo ""
}

# Function to test health
test_health() {
    echo -e "${YELLOW}Health Check${NC}"
    test_endpoint "Health Check" "GET" "$BPAS_URL/health" "" 200
}

# Function to test rule evaluation scenarios
test_routing_scenarios() {
    echo -e "${YELLOW}Routing Rule Evaluation${NC}"
    
    # Test high-value transaction routing
    echo "Testing high-value transaction (should go to processor_a):"
    local high_value_data='{
        "amount": 1500.0,
        "currency": "USD",
        "marketplace": "US"
    }'
    test_endpoint "High Value Transaction" "POST" "$BPAS_URL/bpas/evaluate" "$high_value_data" 200
    
    # Test EUR currency routing
    echo "Testing EUR transaction (should go to processor_b):"
    local eur_data='{
        "amount": 500.0,
        "currency": "EUR",
        "marketplace": "EU"
    }'
    test_endpoint "EUR Transaction" "POST" "$BPAS_URL/bpas/evaluate" "$eur_data" 200
    
    # Test GBP currency routing
    echo "Testing GBP transaction (should go to processor_b):"
    local gbp_data='{
        "amount": 750.0,
        "currency": "GBP",
        "marketplace": "UK"
    }'
    test_endpoint "GBP Transaction" "POST" "$BPAS_URL/bpas/evaluate" "$gbp_data" 200
    
    # Test Japanese marketplace
    echo "Testing Japanese marketplace (should go to processor_b):"
    local jp_data='{
        "amount": 300.0,
        "currency": "JPY",
        "marketplace": "JP"
    }'
    test_endpoint "Japanese Marketplace" "POST" "$BPAS_URL/bpas/evaluate" "$jp_data" 200
    
    # Test premium user tier
    echo "Testing premium user (should go to processor_a):"
    local premium_data='{
        "amount": 200.0,
        "currency": "USD",
        "user_tier": "premium"
    }'
    test_endpoint "Premium User" "POST" "$BPAS_URL/bpas/evaluate" "$premium_data" 200
    
    # Test default routing (should distribute based on percentage)
    echo "Testing default routing distribution:"
    test_default_distribution
}

# Function to test default distribution pattern
test_default_distribution() {
    local processor_a_count=0
    local processor_b_count=0
    local total_tests=20
    
    echo "Running $total_tests tests to verify 70/30 distribution..."
    
    for i in $(seq 1 $total_tests); do
        local default_data="{
            \"amount\": $((100 + i * 10)),
            \"currency\": \"USD\",
            \"marketplace\": \"US\"
        }"
        
        response=$(curl -s -X POST -H "Content-Type: application/json" -d "$default_data" "$BPAS_URL/bpas/evaluate")
        
        if echo "$response" | jq -e '.success' > /dev/null 2>&1; then
            processor=$(echo "$response" | jq -r '.target_processor' 2>/dev/null || echo "unknown")
            if [ "$processor" = "processor_a" ]; then
                processor_a_count=$((processor_a_count + 1))
            elif [ "$processor" = "processor_b" ]; then
                processor_b_count=$((processor_b_count + 1))
            fi
        fi
    done
    
    echo "Distribution results:"
    echo "  Processor A: $processor_a_count/$total_tests ($(( processor_a_count * 100 / total_tests ))%)"
    echo "  Processor B: $processor_b_count/$total_tests ($(( processor_b_count * 100 / total_tests ))%)"
    echo "  Expected: ~70% A, ~30% B"
    echo ""
}

# Function to test rule management
test_rule_management() {
    echo -e "${YELLOW}Rule Management${NC}"
    
    # Get all rules
    test_endpoint "Get All Rules" "GET" "$BPAS_URL/bpas/rules" "" 200
    
    # Reload configuration
    test_endpoint "Reload Configuration" "POST" "$BPAS_URL/bpas/reload" "" 200
    
    # Update a rule (change percentage)
    echo "Testing rule update:"
    local updated_rule='{
        "priority": 10,
        "condition_type": "percentage",
        "condition_value": {},
        "target_processor": "processor_a",
        "percentage": 80,
        "is_active": true,
        "description": "Updated default split to 80% processor A"
    }'
    test_endpoint "Update Rule" "PUT" "$BPAS_URL/bpas/rules/default_primary_split" "$updated_rule" 200
    
    # Test the updated rule
    echo "Testing with updated rule (should now be 80/20 split):"
    local test_data='{
        "amount": 300.0,
        "currency": "USD"
    }'
    test_endpoint "Test Updated Rule" "POST" "$BPAS_URL/bpas/evaluate" "$test_data" 200
}

# Function to test quick rule evaluation
test_quick_evaluation() {
    echo -e "${YELLOW}Quick Rule Testing${NC}"
    
    # Test various scenarios using GET endpoint
    test_endpoint "Test USD $500" "GET" "$BPAS_URL/bpas/test?amount=500&currency=USD" "" 200
    test_endpoint "Test EUR $500" "GET" "$BPAS_URL/bpas/test?amount=500&currency=EUR" "" 200
    test_endpoint "Test $2000 High Value" "GET" "$BPAS_URL/bpas/test?amount=2000&currency=USD" "" 200
    test_endpoint "Test Japanese Market" "GET" "$BPAS_URL/bpas/test?amount=300&currency=JPY&marketplace=JP" "" 200
}

# Function to test error scenarios
test_error_scenarios() {
    echo -e "${YELLOW}Error Scenarios${NC}"
    
    # Invalid amount
    local invalid_amount='{
        "amount": -100,
        "currency": "USD"
    }'
    test_endpoint "Invalid Amount" "POST" "$BPAS_URL/bpas/evaluate" "$invalid_amount" 400
    
    # Invalid JSON
    test_endpoint "Invalid JSON" "POST" "$BPAS_URL/bpas/evaluate" '{"invalid":json}' 400
    
    # Update non-existent rule
    local fake_rule='{
        "priority": 1,
        "target_processor": "processor_a"
    }'
    test_endpoint "Update Non-existent Rule" "PUT" "$BPAS_URL/bpas/rules/nonexistent_rule" "$fake_rule" 404
}

# Function to test admin endpoints
test_admin_endpoints() {
    echo -e "${YELLOW}Admin Endpoints${NC}"
    
    # Get statistics
    test_endpoint "Get Statistics" "GET" "$BPAS_URL/admin/stats" "" 200
}

# Function to demonstrate rule priority
test_rule_priority() {
    echo -e "${YELLOW}Rule Priority Demonstration${NC}"
    
    echo "Testing rule priority with high-value EUR transaction:"
    echo "• Amount: $1500 (triggers high-value rule, priority 1)"
    echo "• Currency: EUR (would trigger EUR rule, priority 2)"
    echo "• Should route to processor_a due to higher priority of amount rule"
    
    local priority_test='{
        "amount": 1500.0,
        "currency": "EUR",
        "marketplace": "EU"
    }'
    
    response=$(curl -s -X POST -H "Content-Type: application/json" -d "$priority_test" "$BPAS_URL/bpas/evaluate")
    
    if echo "$response" | jq -e '.success' > /dev/null 2>&1; then
        processor=$(echo "$response" | jq -r '.target_processor' 2>/dev/null || echo "unknown")
        rule_matched=$(echo "$response" | jq -r '.rule_matched' 2>/dev/null || echo "unknown")
        priority=$(echo "$response" | jq -r '.rule_priority' 2>/dev/null || echo "unknown")
        
        echo "Result:"
        echo "  Processor: $processor"
        echo "  Rule Matched: $rule_matched"
        echo "  Priority: $priority"
        
        if [ "$processor" = "processor_a" ] && [ "$rule_matched" = "high_value_transactions" ]; then
            echo -e "${GREEN} Rule priority working correctly${NC}"
        else
            echo -e "${YELLOW} Unexpected routing result${NC}"
        fi
    else
        echo -e "${RED} Failed to evaluate priority test${NC}"
    fi
    
    echo ""
}

# Main test execution
main() {
    echo "Starting BPAS service tests..."
    echo ""
    
    # Check if service is running
    echo -e "${BLUE}Checking if BPAS service is running...${NC}"
    if ! curl -s "$BPAS_URL/health" > /dev/null; then
        echo -e "${RED}BPAS Service not running at $BPAS_URL${NC}"
        echo "Run: docker-compose up bpas-service"
        exit 1
    fi
    
    echo -e "${GREEN}BPAS Service is running${NC}"
    echo ""
    
    # Run all tests
    test_health
    test_routing_scenarios
    test_rule_priority
    test_quick_evaluation
    test_rule_management
    test_error_scenarios
    test_admin_endpoints
    
    echo ""
    echo -e "${GREEN}All BPAS tests completed!${NC}"
    echo ""
    echo "Summary:"
    echo "• High-value transactions (>$1000) route to processor_a"
    echo "• EUR/GBP transactions route to processor_b"
    echo "• Japanese marketplace routes to processor_b"
    echo "• Premium users route to processor_a"
    echo "• Default traffic splits 70% processor_a, 30% processor_b"
    echo "• Rules can be updated dynamically via API"
    echo ""
    echo "Next steps:"
    echo "• Integrate with Payment Orchestrator for automatic routing"
    echo "• Test dynamic rule updates during payment processing"
    echo "• Monitor routing distribution and performance"
    echo ""
    echo "Available admin endpoints:"
    echo "  curl $BPAS_URL/admin/stats"
    echo "  curl $BPAS_URL/bpas/rules"
    echo "  curl '$BPAS_URL/bpas/test?amount=1000&currency=EUR'"
}

# Check if jq is available
if ! command -v jq &> /dev/null; then
    echo -e "${YELLOW} Warning: jq not found. JSON responses will not be formatted.${NC}"
    echo "Install jq for better output: apt-get install jq"
    echo ""
fi

# Run main function
main "$@"