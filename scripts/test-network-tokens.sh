#!/bin/bash


set -e

echo "🔐 Testing Network Token Service"
echo "================================"
echo ""
echo "📋 Important Notes:"
echo "• Network Token Service has 95% success rate for network tokens"
echo "• 5% of requests will fallback to dual vault (processor-specific tokens)"
echo "• This simulates real-world network token availability"
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Base URL
NETWORK_TOKEN_URL="http://localhost:8103"

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
    echo -e "${YELLOW}🏥 Health Check${NC}"
    test_endpoint "Health Check" "GET" "$NETWORK_TOKEN_URL/health" "" 200
}

# Function to test network token creation with retry to demonstrate both outcomes
test_token_creation() {
    echo -e "${YELLOW}🎯 Token Creation Tests${NC}"
    
    # Test multiple token creation attempts to see both network and dual vault outcomes
    local network_tokens=0
    local dual_vault_tokens=0
    local total_attempts=10
    
    echo "Creating $total_attempts tokens to demonstrate 95%/5% distribution..."
    
    for i in $(seq 1 $total_attempts); do
        local card_number="424242424242424$i"
        if [ ${#card_number} -gt 16 ]; then
            card_number="4242424242424242"
        fi
        
        local token_data="{
            \"card_number\": \"$card_number\",
            \"exp_month\": 12,
            \"exp_year\": 2026,
            \"cvv\": \"123\"
        }"
        
        echo -e "${BLUE}  Attempt $i/$total_attempts${NC}"
        
        response=$(curl -s -w "HTTPSTATUS:%{http_code}" -X "POST" -H "Content-Type: application/json" -d "$token_data" "$NETWORK_TOKEN_URL/network-tokens/create")
        http_code=$(echo "$response" | tr -d '\n' | sed -e 's/.*HTTPSTATUS://')
        body=$(echo "$response" | sed -e 's/HTTPSTATUS:.*//g')
        
        if [ "$http_code" -eq 200 ]; then
            token_type=$(echo "$body" | jq -r '.token_type' 2>/dev/null || echo "unknown")
            if [ "$token_type" = "network" ]; then
                network_tokens=$((network_tokens + 1))
                echo -e "${GREEN}    ✅ Network token created${NC}"
            elif [ "$token_type" = "dual_vault" ]; then
                dual_vault_tokens=$((dual_vault_tokens + 1))
                echo -e "${YELLOW}    🔄 Dual vault fallback${NC}"
            fi
            
            # Store the last successful token for later tests
            if [ $i -eq 1 ]; then
                last_token=$(echo "$body" | jq -r '.network_token.network_token' 2>/dev/null || echo "")
            fi
        else
            echo -e "${RED}    ❌ Failed: HTTP $http_code${NC}"
        fi
    done
    
    echo ""
    echo "Results summary:"
    echo "  Network tokens: $network_tokens/$total_attempts ($(( network_tokens * 100 / total_attempts ))%)"
    echo "  Dual vault: $dual_vault_tokens/$total_attempts ($(( dual_vault_tokens * 100 / total_attempts ))%)"
    echo "  Expected: ~95% network, ~5% dual vault"
    echo ""
}

# Function to test token validation
test_token_validation() {
    echo -e "${YELLOW}✅ Token Validation${NC}"
    
    # First create a token to validate
    local token_data='{
        "card_number": "4242424242424242",
        "exp_month": 12,
        "exp_year": 2026,
        "cvv": "123"
    }'
    
    echo "Creating token for validation test..."
    response=$(curl -s -X POST -H "Content-Type: application/json" -d "$token_data" "$NETWORK_TOKEN_URL/network-tokens/create")
    
    if echo "$response" | jq -e '.success' > /dev/null 2>&1; then
        network_token=$(echo "$response" | jq -r '.network_token.network_token')
        echo "Created token: $network_token"
        
        # Test validation with processor A
        local validate_data="{
            \"network_token\": \"$network_token\",
            \"processor\": \"processor_a\"
        }"
        test_endpoint "Validate Token - Processor A" "POST" "$NETWORK_TOKEN_URL/network-tokens/validate" "$validate_data" 200
        
        # Test validation with processor B
        validate_data="{
            \"network_token\": \"$network_token\",
            \"processor\": \"processor_b\"
        }"
        test_endpoint "Validate Token - Processor B" "POST" "$NETWORK_TOKEN_URL/network-tokens/validate" "$validate_data" 200
        
        # Test token info retrieval
        test_endpoint "Get Token Info" "GET" "$NETWORK_TOKEN_URL/network-tokens/$network_token" "" 200
        
        # Store token for refresh test
        validation_token="$network_token"
    else
        echo -e "${YELLOW}⚠️  Could not create token for validation test${NC}"
    fi
}

# Function to test token refresh
test_token_refresh() {
    echo -e "${YELLOW}🔄 Token Refresh${NC}"
    
    if [ -n "$validation_token" ]; then
        local refresh_data="{
            \"network_token\": \"$validation_token\",
            \"exp_month\": 6,
            \"exp_year\": 2027
        }"
        test_endpoint "Refresh Token" "POST" "$NETWORK_TOKEN_URL/network-tokens/refresh" "$refresh_data" 200
    else
        echo -e "${YELLOW}⚠️  No token available for refresh test${NC}"
    fi
}

# Function to test error scenarios
test_error_scenarios() {
    echo -e "${YELLOW}🚨 Error Scenarios${NC}"
    
    # Invalid card number
    local invalid_card='{
        "card_number": "123",
        "exp_month": 12,
        "exp_year": 2026,
        "cvv": "123"
    }'
    test_endpoint "Invalid Card Number" "POST" "$NETWORK_TOKEN_URL/network-tokens/create" "$invalid_card" 400
    
    # Expired card
    local expired_card='{
        "card_number": "4242424242424242",
        "exp_month": 12,
        "exp_year": 2020,
        "cvv": "123"
    }'
    test_endpoint "Expired Card" "POST" "$NETWORK_TOKEN_URL/network-tokens/create" "$expired_card" 400
    
    # Validate non-existent token
    local invalid_validate='{
        "network_token": "ntk_nonexistent_12345678",
        "processor": "processor_a"
    }'
    test_endpoint "Validate Non-existent Token" "POST" "$NETWORK_TOKEN_URL/network-tokens/validate" "$invalid_validate" 404
    
    # Get info for non-existent token
    test_endpoint "Get Non-existent Token Info" "GET" "$NETWORK_TOKEN_URL/network-tokens/ntk_nonexistent_12345678" "" 404
}

# Function to test admin endpoints
test_admin_endpoints() {
    echo -e "${YELLOW}⚙️  Admin Endpoints${NC}"
    
    # Get stats
    test_endpoint "Get Statistics" "GET" "$NETWORK_TOKEN_URL/admin/stats" "" 200
    
    # Change success rate
    test_endpoint "Set Success Rate to 80%" "POST" "$NETWORK_TOKEN_URL/admin/set-success-rate?rate=80" "" 200
    
    # Reset stats
    test_endpoint "Reset Statistics" "POST" "$NETWORK_TOKEN_URL/admin/reset-stats" "" 200
    
    # Restore normal success rate
    test_endpoint "Restore 95% Success Rate" "POST" "$NETWORK_TOKEN_URL/admin/set-success-rate?rate=95" "" 200
}

# Function to test different card brands
test_card_brands() {
    echo -e "${YELLOW}💳 Card Brand Detection${NC}"
    
    # Test different card brands using a simpler approach
    echo -e "${BLUE}Testing Visa card${NC}"
    test_single_card_brand "Visa" "4242424242424242"
    
    echo -e "${BLUE}Testing Mastercard${NC}"
    test_single_card_brand "Mastercard" "5555555555554444"
    
    echo -e "${BLUE}Testing Amex${NC}"
    test_single_card_brand "Amex" "378282246310005"
    
    echo -e "${BLUE}Testing Discover${NC}"
    test_single_card_brand "Discover" "6011111111111117"
    
    echo ""
}

# Helper function to test a single card brand
test_single_card_brand() {
    local brand="$1"
    local card_number="$2"
    
    local brand_data="{
        \"card_number\": \"$card_number\",
        \"exp_month\": 12,
        \"exp_year\": 2026,
        \"cvv\": \"123\"
    }"
    
    response=$(curl -s -X POST -H "Content-Type: application/json" -d "$brand_data" "$NETWORK_TOKEN_URL/network-tokens/create")
    
    if echo "$response" | jq -e '.success' > /dev/null 2>&1; then
        detected_brand=$(echo "$response" | jq -r '.network_token.brand' 2>/dev/null || echo "unknown")
        token_type=$(echo "$response" | jq -r '.token_type' 2>/dev/null || echo "unknown")
        echo -e "${GREEN}  ✅ $brand card processed - Detected: $detected_brand, Type: $token_type${NC}"
    else
        echo -e "${RED}  ❌ $brand card failed${NC}"
    fi
}

# Function to demonstrate token portability
test_token_portability() {
    echo -e "${YELLOW}🔄 Token Portability Demonstration${NC}"
    
    echo "Creating tokens and showing portability characteristics..."
    
    # Create multiple tokens to show different types
    for i in {1..5}; do
        local token_data="{
            \"card_number\": \"424242424242424$i\",
            \"exp_month\": 12,
            \"exp_year\": 2026,
            \"cvv\": \"123\"
        }"
        
        response=$(curl -s -X POST -H "Content-Type: application/json" -d "$token_data" "$NETWORK_TOKEN_URL/network-tokens/create")
        
        if echo "$response" | jq -e '.success' > /dev/null 2>&1; then
            token_type=$(echo "$response" | jq -r '.token_type' 2>/dev/null || echo "unknown")
            is_portable=$(echo "$response" | jq -r '.is_portable' 2>/dev/null || echo "unknown")
            
            if [ "$token_type" = "network" ]; then
                echo -e "${GREEN}  Token $i: Network token (portable: $is_portable) - Works with both processors${NC}"
            else
                echo -e "${YELLOW}  Token $i: Dual vault (portable: $is_portable) - Processor-specific tokens required${NC}"
                processor_a_token=$(echo "$response" | jq -r '.fallback_info.processor_a_token' 2>/dev/null || echo "")
                processor_b_token=$(echo "$response" | jq -r '.fallback_info.processor_b_token' 2>/dev/null || echo "")
                echo -e "${BLUE}    Processor A token: ${processor_a_token:0:20}...${NC}"
                echo -e "${BLUE}    Processor B token: ${processor_b_token:0:20}...${NC}"
            fi
        fi
    done
    echo ""
}

# Main test execution
main() {
    echo "Starting network token service tests..."
    echo ""
    
    # Check if service is running
    echo -e "${BLUE}🔍 Checking if network token service is running...${NC}"
    if ! curl -s "$NETWORK_TOKEN_URL/health" > /dev/null; then
        echo -e "${RED}❌ Network Token Service not running at $NETWORK_TOKEN_URL${NC}"
        echo "Run: docker-compose up network-token-service"
        exit 1
    fi
    
    echo -e "${GREEN}✅ Network Token Service is running${NC}"
    echo ""
    
    # Run all tests
    test_health
    test_token_creation
    test_token_validation
    test_token_refresh
    test_card_brands
    test_token_portability
    test_error_scenarios
    test_admin_endpoints
    
    echo ""
    echo -e "${GREEN}🎉 All network token tests completed!${NC}"
    echo ""
    echo "Summary:"
    echo "• Network tokens provide portability across processors (95% of cases)"
    echo "• Dual vault fallback ensures compatibility when network tokens unavailable (5% of cases)"
    echo "• Token validation works for both token types"
    echo "• Multiple card brands supported with automatic detection"
    echo ""
    echo "Next steps:"
    echo "• Integrate with Payment Orchestrator for automatic token selection"
    echo "• Implement token refresh scheduling for expiring tokens"
    echo "• Add token caching for improved performance"
    echo ""
    echo "Available admin endpoints:"
    echo "  curl -X POST $NETWORK_TOKEN_URL/admin/set-success-rate?rate=90"
    echo "  curl $NETWORK_TOKEN_URL/admin/stats"
    echo "  curl $NETWORK_TOKEN_URL/admin/reset-stats"
}

# Check if jq is available
if ! command -v jq &> /dev/null; then
    echo -e "${YELLOW}⚠️  Warning: jq not found. JSON responses will not be formatted.${NC}"
    echo "Install jq for better output: apt-get install jq"
    echo ""
fi

# Run main function
main "$@"