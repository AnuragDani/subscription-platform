.PHONY: build up down logs clean test seed demo

# Build all services
build:
	docker-compose build

# Start all services
up:
	docker-compose up -d

# Stop all services
down:
	docker-compose down

# View logs
logs:
	docker-compose logs -f

# Clean everything
clean:
	docker-compose down -v
	docker system prune -f

# Run tests
test:
	go test ./...

# Seed database with demo data
seed:
	./scripts/seed-data.sh

# Run demo scenarios
demo:
	./scripts/demo-scenarios.sh

# Health check all services
health:
	./scripts/health-check.sh

# Reset demo data
reset:
	curl -X POST http://localhost:8080/admin/demo/reset

# Quick dev setup
dev: clean build up
	@echo "Waiting for services to start..."
	@sleep 10
	@make seed
	@echo "Development environment ready!"
	@echo "API Gateway: http://localhost:8080"
	@echo "Run 'make demo' to test scenarios"