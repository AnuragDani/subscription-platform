# Makefile for Payment Orchestrator Prototype

.PHONY: help build up down logs test clean

# Default target
help:
	@echo "Payment Orchestrator Prototype"
	@echo "==============================="
	@echo ""
	@echo "Available commands:"
	@echo "  make build          - Build all services"
	@echo "  make up             - Start all services"
	@echo "  make down           - Stop all services"
	@echo "  make logs           - View logs from all services"
	@echo "  make test           - Run all test suites"
	@echo "  make clean          - Clean up containers and volumes"
	@echo ""
	@echo "Service-specific commands:"
	@echo "  make test-processors     - Test mock payment processors"
	@echo "  make test-network-tokens - Test network token service"
	@echo "  make test-bpas          - Test BPAS routing service"
	@echo ""
	@echo "Development commands:"
	@echo "  make dev            - Start core services for development"
	@echo "  make dev-processors - Start only processors"
	@echo "  make dev-tokens     - Start token service"
	@echo "  make dev-bpas       - Start BPAS service"

# Build all services
build:
	docker-compose build

# Start all services
up:
	docker-compose up -d
	@echo "Services starting... waiting for health checks..."
	@sleep 10
	@echo "Checking service health:"
	@curl -s http://localhost:8080/health | jq .service 2>/dev/null || echo "API Gateway: Starting..."
	@curl -s http://localhost:8101/health | jq .service 2>/dev/null || echo "Processor A: Starting..."
	@curl -s http://localhost:8102/health | jq .service 2>/dev/null || echo "Processor B: Starting..."
	@curl -s http://localhost:8103/health | jq .service 2>/dev/null || echo "Network Token Service: Starting..."
	@curl -s http://localhost:8003/health | jq .service 2>/dev/null || echo "BPAS Service: Starting..."

# Stop all services
down:
	docker-compose down

# View logs
logs:
	docker-compose logs -f

# Run all tests
test: test-processors test-network-tokens test-bpas
	@echo "All tests completed!"

# Test payment processors
test-processors:
	@echo "Testing Payment Processors..."
	@chmod +x scripts/test-processors.sh
	@./scripts/test-processors.sh

# Test network token service
test-network-tokens:
	@echo "Testing Network Token Service..."
	@chmod +x scripts/test-network-tokens.sh
	@./scripts/test-network-tokens.sh

# Test BPAS service
test-bpas:
	@echo "Testing BPAS Service..."
	@chmod +x scripts/test-bpas.sh
	@./scripts/test-bpas.sh

# Development mode - start core services only
dev:
	docker-compose up postgres redis -d
	@echo "Core services (postgres, redis) started for development"

# Start only processors for testing
dev-processors:
	docker-compose up postgres redis mock-processor-a mock-processor-b -d

# Start token service
dev-tokens:
	docker-compose up postgres redis network-token-service -d

# Start BPAS service
dev-bpas:
	docker-compose up postgres redis bpas-service -d

# Clean up everything
clean:
	docker-compose down -v
	docker system prune -f
	@echo "Cleanup completed"

# Show service status
status:
	@echo "Service Status:"
	@echo "==============="
	@curl -s http://localhost:8080/health | jq '.service + ": " + .status' 2>/dev/null || echo "API Gateway: Not running"
	@curl -s http://localhost:8001/health | jq '.service + ": " + .status' 2>/dev/null || echo "Payment Orchestrator: Not running"
	@curl -s http://localhost:8002/health | jq '.service + ": " + .status' 2>/dev/null || echo "Subscription Service: Not running"
	@curl -s http://localhost:8003/health | jq '.service + ": " + .status' 2>/dev/null || echo "BPAS Service: Not running"
	@curl -s http://localhost:8101/health | jq '.service + ": " + .status' 2>/dev/null || echo "Processor A: Not running"
	@curl -s http://localhost:8102/health | jq '.service + ": " + .status' 2>/dev/null || echo "Processor B: Not running"
	@curl -s http://localhost:8103/health | jq '.service + ": " + .status' 2>/dev/null || echo "Network Token Service: Not running"

# Demo routing scenarios
demo-routing:
	@echo "BPAS Routing Demonstration"
	@echo "=========================="
	@echo ""
	@echo "High-value transaction (\$$1500 USD):"
	@curl -s -X POST http://localhost:8003/bpas/evaluate -H "Content-Type: application/json" -d '{"amount":1500,"currency":"USD"}' | jq '.target_processor + " (rule: " + .rule_matched + ")"'
	@echo ""
	@echo "EUR transaction (\$$500 EUR):"
	@curl -s -X POST http://localhost:8003/bpas/evaluate -H "Content-Type: application/json" -d '{"amount":500,"currency":"EUR"}' | jq '.target_processor + " (rule: " + .rule_matched + ")"'
	@echo ""
	@echo "Premium user (\$$200 USD):"
	@curl -s -X POST http://localhost:8003/bpas/evaluate -H "Content-Type: application/json" -d '{"amount":200,"currency":"USD","user_tier":"premium"}' | jq '.target_processor + " (rule: " + .rule_matched + ")"'
	@echo ""
	@echo "Default routing (\$$300 USD):"
	@curl -s -X POST http://localhost:8003/bpas/evaluate -H "Content-Type: application/json" -d '{"amount":300,"currency":"USD"}' | jq '.target_processor + " (rule: " + .rule_matched + ")"'

# Quick health check of all services
health:
	@echo "Health Check Summary:"
	@echo "===================="
	@echo "Infrastructure:"
	@docker-compose ps postgres | grep -q "Up" && echo "  PostgreSQL: ✅ Running" || echo "  PostgreSQL: ❌ Not running"
	@docker-compose ps redis | grep -q "Up" && echo "  Redis: ✅ Running" || echo "  Redis: ❌ Not running"
	@echo ""
	@echo "Payment Services:"
	@curl -s http://localhost:8101/health > /dev/null && echo "  Processor A: ✅ Healthy" || echo "  Processor A: ❌ Unhealthy"
	@curl -s http://localhost:8102/health > /dev/null && echo "  Processor B: ✅ Healthy" || echo "  Processor B: ❌ Unhealthy"
	@curl -s http://localhost:8103/health > /dev/null && echo "  Network Tokens: ✅ Healthy" || echo "  Network Tokens: ❌ Unhealthy"
	@curl -s http://localhost:8003/health > /dev/null && echo "  BPAS: ✅ Healthy" || echo "  BPAS: ❌ Unhealthy"


# Payment Orchestrator specific targets
.PHONY: test-orchestrator build-orchestrator logs-orchestrator

# Test the Payment Orchestrator integration
test-orchestrator:
	@echo "Testing Payment Orchestrator integration..."
	@chmod +x scripts/test-orchestrator.sh
	@scripts/test-orchestrator.sh

# Build and restart just the payment orchestrator
build-orchestrator:
	@echo "Building Payment Orchestrator..."
	@docker-compose build payment-orchestrator
	@docker-compose up payment-orchestrator -d

# View Payment Orchestrator logs
logs-orchestrator:
	@docker-compose logs -f payment-orchestrator

# Test the complete payment flow (orchestrator + dependencies)
test-payment-flow:
	@echo "Testing complete payment processing flow..."
	@docker-compose up mock-processor-a mock-processor-b bpas-service network-token-service payment-orchestrator -d
	@sleep 10
	@chmod +x scripts/test-orchestrator.sh
	@scripts/test-orchestrator.sh

# Quick integration test (all core services)
test-integration:
	@echo "Running integration tests..."
	@docker-compose up postgres redis mock-processor-a mock-processor-b bpas-service network-token-service payment-orchestrator -d
	@sleep 15
	@make test-processors
	@make test-bpas  
	@make test-orchestrator

# Database inspection for payment testing
inspect-transactions:
	@echo "Inspecting recent transactions..."
	@docker exec -it $$(docker ps -q -f name=postgres) psql -U payment_user -d payment_db -c "SELECT id, amount, currency, status, processor_used, created_at FROM transactions ORDER BY created_at DESC LIMIT 10;"

# Cache inspection for idempotency testing  
inspect-cache:
	@echo "Inspecting idempotency cache..."
	@docker exec -it $$(docker ps -q -f name=redis) redis-cli KEYS "idempotency:*" | head -10

# Reset test data
reset-test-data:
	@echo "Resetting test data..."
	@docker exec -it $$(docker ps -q -f name=postgres) psql -U payment_user -d payment_db -c "DELETE FROM transactions WHERE subscription_id LIKE 'sub_test_%' OR subscription_id LIKE 'sub_failover_%' OR subscription_id LIKE 'sub_eur_%' OR subscription_id LIKE 'sub_highvalue_%' OR subscription_id LIKE 'sub_invalid_%' OR subscription_id LIKE 'sub_load_%';"
	@docker exec -it $$(docker ps -q -f name=redis) redis-cli FLUSHALL
	@echo "Test data reset complete"