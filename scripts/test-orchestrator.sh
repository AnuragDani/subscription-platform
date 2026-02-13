#!/bin/bash

set -euo pipefail

echo "🧪 Testing Payment Orchestrator (Deterministic)"
echo "==============================================="
echo ""

BASE_URL="${BASE_URL:-http://localhost:8001}"
PROCESSOR_A_URL="${PROCESSOR_A_URL:-http://localhost:8101}"
PROCESSOR_B_URL="${PROCESSOR_B_URL:-http://localhost:8102}"

# Deterministic fixture IDs (UUID only)
USER_MAIN_ID="11111111-1111-1111-1111-111111111111"
SUBSCRIPTION_ID="11111111-1111-1111-1111-111111111201"
PAYMENT_METHOD_ID="11111111-1111-1111-1111-111111111301"

USER_FAILOVER_ID="22222222-2222-2222-2222-222222222222"
FAILOVER_SUB_ID="22222222-2222-2222-2222-222222222201"
FAILOVER_PM_ID="22222222-2222-2222-2222-222222222301"

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

LAST_CHARGE_TXN_ID=""

cleanup() {
  # Restore baseline processor behavior for other scripts
  curl -s -X POST "$PROCESSOR_A_URL/admin/set-failure-rate?rate=20" > /dev/null || true
  curl -s -X POST "$PROCESSOR_B_URL/admin/set-failure-rate?rate=10" > /dev/null || true
  current=$(curl -sS "$PROCESSOR_A_URL/health" 2>/dev/null | jq -r '.status' 2>/dev/null || true)
  if [ "$current" = "unhealthy" ]; then
    curl -s -X POST "$PROCESSOR_A_URL/admin/toggle-status" >/dev/null || true
  fi
}
trap cleanup EXIT

require_tools() {
  for cmd in curl jq docker; do
    if ! command -v "$cmd" >/dev/null 2>&1; then
      echo -e "${RED}❌ Required tool missing: $cmd${NC}"
      exit 1
    fi
  done
}

seed_deterministic_fixtures() {
  echo "🌱 Ensuring deterministic DB fixtures..."

  local pg_container
  pg_container=$(docker compose ps -q postgres 2>/dev/null || true)
  if [ -z "$pg_container" ]; then
    pg_container=$(docker ps -q -f name=subscription-postgres-1 2>/dev/null || true)
  fi

  if [ -z "$pg_container" ]; then
    echo -e "${RED}❌ Could not find postgres container${NC}"
    exit 1
  fi

  docker exec -i "$pg_container" psql -U payment_user -d payment_db >/dev/null <<SQL
INSERT INTO payment_methods (
  id, user_id, network_token, processor_a_token, processor_b_token,
  token_type, last_four, exp_month, exp_year
) VALUES
  ('$PAYMENT_METHOD_ID', '$USER_MAIN_ID', 'ntk_det_main_4242', NULL, NULL, 'network', '4242', 12, 2030),
  ('$FAILOVER_PM_ID', '$USER_FAILOVER_ID', 'ntk_det_failover_9999', NULL, NULL, 'network', '9999', 12, 2030)
ON CONFLICT (id) DO NOTHING;

INSERT INTO subscriptions (
  id, user_id, status, plan_id, amount, currency, billing_cycle, next_billing_date
) VALUES
  ('$SUBSCRIPTION_ID', '$USER_MAIN_ID', 'active', 'premium_monthly', 1999, 'USD', 'monthly', NOW() + INTERVAL '30 days'),
  ('$FAILOVER_SUB_ID', '$USER_FAILOVER_ID', 'active', 'premium_monthly', 1501, 'USD', 'monthly', NOW() + INTERVAL '30 days')
ON CONFLICT (id) DO NOTHING;
SQL

  echo -e "${GREEN}✅ Deterministic fixtures ready${NC}"
}

assert_http_status() {
  local actual="$1"
  local expected="$2"
  local step="$3"

  if [ "$actual" != "$expected" ]; then
    echo -e "${RED}❌ $step failed${NC} - Expected HTTP $expected, got $actual"
    exit 1
  fi
  echo -e "${GREEN}✅ $step${NC} - HTTP $actual"
}

assert_http_status_one_of() {
  local actual="$1"
  local allowed_csv="$2"
  local step="$3"

  IFS=',' read -r -a allowed <<< "$allowed_csv"
  for expected in "${allowed[@]}"; do
    if [ "$actual" = "$expected" ]; then
      echo -e "${GREEN}✅ $step${NC} - HTTP $actual"
      return
    fi
  done

  echo -e "${RED}❌ $step failed${NC} - Expected one of [$allowed_csv], got $actual"
  exit 1
}

post_json() {
  local url="$1"
  local data="$2"
  curl -sS -w '\n%{http_code}' -X POST "$url" \
    -H "Content-Type: application/json" \
    -d "$data"
}

set_processor_a_health() {
  local target="$1" # healthy|unhealthy
  current=$(curl -sS "$PROCESSOR_A_URL/health" | jq -r '.status')
  if [ "$current" != "$target" ]; then
    curl -s -X POST "$PROCESSOR_A_URL/admin/toggle-status" >/dev/null
  fi
  final=$(curl -sS "$PROCESSOR_A_URL/health" | jq -r '.status')
  if [ "$final" != "$target" ]; then
    echo -e "${RED}❌ Could not set Processor A status to ${target}${NC}"
    exit 1
  fi
}

# Pre-flight
require_tools

if ! curl -s "$BASE_URL/health" >/dev/null; then
  echo -e "${RED}❌ Payment Orchestrator is not running at $BASE_URL${NC}"
  exit 1
fi

echo -e "${GREEN}✅ Payment Orchestrator is running${NC}"
seed_deterministic_fixtures

echo ""
echo "📋 Deterministic Test Suite"
echo "==========================="

# 1) Health
health_status=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/health")
assert_http_status "$health_status" "200" "Health Check"

# Set deterministic success path first
curl -s -X POST "$PROCESSOR_A_URL/admin/set-failure-rate?rate=0" >/dev/null
curl -s -X POST "$PROCESSOR_B_URL/admin/set-failure-rate?rate=0" >/dev/null
set_processor_a_health "healthy"

# 2) Successful Charge
charge_data='{
  "subscription_id": "'$SUBSCRIPTION_ID'",
  "payment_method_id": "'$PAYMENT_METHOD_ID'",
  "amount": 1999,
  "currency": "USD"
}'

charge_result=$(post_json "$BASE_URL/orchestrator/charge" "$charge_data")
charge_body=$(echo "$charge_result" | sed '$d')
charge_status=$(echo "$charge_result" | tail -n1)
assert_http_status "$charge_status" "201" "Process Payment"

charge_success=$(echo "$charge_body" | jq -r '.success')
if [ "$charge_success" != "true" ]; then
  echo -e "${RED}❌ Process Payment returned success=false${NC}"
  echo "$charge_body"
  exit 1
fi

LAST_CHARGE_TXN_ID=$(echo "$charge_body" | jq -r '.transaction_id')
if [ -z "$LAST_CHARGE_TXN_ID" ] || [ "$LAST_CHARGE_TXN_ID" = "null" ]; then
  echo -e "${RED}❌ Missing transaction_id from charge response${NC}"
  exit 1
fi

# 3) Idempotency
idempotency_key="idem_det_$(date +%s)_$RANDOM"
idem_data='{
  "subscription_id": "'$SUBSCRIPTION_ID'",
  "payment_method_id": "'$PAYMENT_METHOD_ID'",
  "amount": 500,
  "currency": "USD",
  "idempotency_key": "'$idempotency_key'"
}'

first_result=$(post_json "$BASE_URL/orchestrator/charge" "$idem_data")
second_result=$(post_json "$BASE_URL/orchestrator/charge" "$idem_data")

first_status=$(echo "$first_result" | tail -n1)
second_status=$(echo "$second_result" | tail -n1)
assert_http_status "$first_status" "201" "Idempotency First Request"
assert_http_status_one_of "$second_status" "200,201" "Idempotency Replay Request"

first_body=$(echo "$first_result" | sed '$d')
second_body=$(echo "$second_result" | sed '$d')
if [ "$first_body" = "$second_body" ]; then
  echo -e "${GREEN}✅ Idempotency working - replay response is identical${NC}"
else
  echo -e "${RED}❌ Idempotency failed - replay response differs${NC}"
  echo "First:  $first_body"
  echo "Second: $second_body"
  exit 1
fi

# 4) Deterministic failover
# Failover in orchestrator triggers on processor errors/unhealthy, not on decline responses.
# Force processor A unhealthy to get a deterministic failover path.
set_processor_a_health "unhealthy"
curl -s -X POST "$PROCESSOR_B_URL/admin/set-failure-rate?rate=0" >/dev/null

failover_data='{
  "subscription_id": "'$FAILOVER_SUB_ID'",
  "payment_method_id": "'$FAILOVER_PM_ID'",
  "amount": 1501,
  "currency": "USD"
}'

failover_result=$(post_json "$BASE_URL/orchestrator/charge" "$failover_data")
failover_body=$(echo "$failover_result" | sed '$d')
failover_status=$(echo "$failover_result" | tail -n1)
assert_http_status "$failover_status" "201" "Failover Charge"

failover_processor=$(echo "$failover_body" | jq -r '.processor_used')
if [ "$failover_processor" = "processor_b" ]; then
  echo -e "${GREEN}✅ Failover successful - charge routed to processor_b${NC}"
else
  echo -e "${RED}❌ Failover check failed - expected processor_b, got $failover_processor${NC}"
  echo "$failover_body"
  exit 1
fi

# Restore deterministic success path for refund step
set_processor_a_health "healthy"
curl -s -X POST "$PROCESSOR_B_URL/admin/set-failure-rate?rate=0" >/dev/null

# 5) Refund uses real transaction ID from successful charge
refund_data='{
  "transaction_id": "'$LAST_CHARGE_TXN_ID'",
  "amount": 100,
  "reason": "deterministic_test_refund"
}'

refund_result=$(post_json "$BASE_URL/orchestrator/refund" "$refund_data")
refund_body=$(echo "$refund_result" | sed '$d')
refund_status=$(echo "$refund_result" | tail -n1)
assert_http_status "$refund_status" "200" "Process Refund"

refund_success=$(echo "$refund_body" | jq -r '.success')
if [ "$refund_success" = "true" ]; then
  echo -e "${GREEN}✅ Refund successful for transaction $LAST_CHARGE_TXN_ID${NC}"
else
  echo -e "${RED}❌ Refund returned success=false${NC}"
  echo "$refund_body"
  exit 1
fi

# 6) Stats endpoint
stats_status=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/admin/stats")
assert_http_status "$stats_status" "200" "Get Stats"

echo ""
echo "================================"
echo -e "${GREEN}🏁 Deterministic test suite passed${NC}"
