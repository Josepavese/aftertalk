.PHONY: build run test clean test-unit test-integration test-e2e test-performance test-coverage lint fmt help

# Variables
BINARY_NAME=aftertalk
MAIN_PATH=./cmd/aftertalk
GO=go
GOFLAGS=-v

# Build
build:
	$(GO) build $(GOFLAGS) -o bin/$(BINARY_NAME) $(MAIN_PATH)

# Run
run: build
	./bin/$(BINARY_NAME)

# Development run
dev:
	$(GO) run $(MAIN_PATH)

# Test - All tests
test:
	$(GO) test -v -race -count=1 ./...

# Unit tests
test-unit:
	$(GO) test -v -race -count=1 ./... -run Test[!I] -coverprofile=coverage_unit.out -covermode=atomic

# Integration tests
test-integration:
	$(GO) test -v -race -count=1 ./internal/storage/sqlite/... -run TestDB_ -coverprofile=coverage_integration.out -covermode=atomic

# E2E tests (run integration tests against a live server)
test-e2e:
	$(GO) test -v -race -count=1 ./internal/api/... -run TestIntegration

# Performance tests
test-performance:
	./scripts/run-performance-tests.sh

# Test coverage - All tests
test-coverage:
	$(GO) test -v -race -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	$(GO) tool cover -func=coverage.out

# Clean
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html coverage_unit.out coverage_integration.out coverage_unit.html coverage_integration.html coverage_total.txt

# Database migrations (SQLite auto-creates on first run)
migrate-up:
	@echo "SQLite database auto-creates on first run. No manual migration needed."
	@echo "Migrations are embedded in the application."

migrate-down:
	@echo "To reset database, delete aftertalk.db file"
	@echo "  rm aftertalk.db"

# Code quality
lint:
	golangci-lint run

fmt:
	$(GO) fmt ./...

# Docker
docker-build:
	docker build -t aftertalk:latest .

docker-run:
	docker-compose up -d

docker-stop:
	docker-compose down

# Help
help:
	@echo "Available targets:"
	@echo "  build              - Build the binary"
	@echo "  run                - Build and run"
	@echo "  dev                - Run without building"
	@echo ""
	@echo "  test               - Run all tests"
	@echo "  test-unit          - Run unit tests only"
	@echo "  test-integration   - Run integration tests only"
	@echo "  test-e2e           - Run E2E tests"
	@echo "  test-performance   - Run performance tests"
	@echo "  test-coverage      - Run tests with coverage report"
	@echo ""
	@echo "  clean              - Remove build artifacts"
	@echo "  migrate-up         - Run database migrations"
	@echo "  migrate-down       - Rollback migrations"
	@echo ""
	@echo "  lint               - Run strict linter"
	@echo "  fmt                - Format code"
	@echo ""
	@echo "  docker-build       - Build Docker image"
	@echo "  docker-run         - Run with Docker Compose"
	@echo "  docker-stop        - Stop Docker Compose"
