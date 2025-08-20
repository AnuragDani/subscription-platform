#!/bin/bash
# scripts/debug-network-tokens.sh

echo "🔍 Debugging Network Token Service"
echo "=================================="

NETWORK_TOKEN_URL="http://localhost:8103"

echo "1. Testing service health..."
curl -s "$NETWORK_TOKEN_URL/health" | jq . 2>/dev/null || curl -s "$NETWORK_TOKEN_URL/health"

echo ""
echo "2. Testing available routes..."
echo "Trying to hit non-existent route to see available endpoints:"
curl -s "$NETWORK_TOKEN_URL/nonexistent" 

echo ""
echo "3. Testing token creation endpoint directly..."
curl -v -X POST "$NETWORK_TOKEN_URL/network-tokens/create" \
  -H "Content-Type: application/json" \
  -d '{
    "card_number": "4242424242424242",
    "exp_month": 12,
    "exp_year": 2026,
    "cvv": "123"
  }'

echo ""
echo "4. Checking service logs..."
echo "Run: docker-compose logs network-token-service"