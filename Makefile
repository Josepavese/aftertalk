.PHONY: build run test clean migrate-up migrate-down lint fmt help

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

# Test
test:
	$(GO) test -v ./...

test-coverage:
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

# Clean
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html

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
	@echo "  build        - Build the binary"
	@echo "  run          - Build and run"
	@echo "  dev          - Run without building"
	@echo "  test         - Run tests"
	@echo "  test-coverage - Run tests with coverage"
	@echo "  clean        - Remove build artifacts"
	@echo "  migrate-up   - Run database migrations"
	@echo "  migrate-down - Rollback migrations"
	@echo "  lint         - Run linter"
	@echo "  fmt          - Format code"
	@echo "  docker-build - Build Docker image"
	@echo "  docker-run   - Run with Docker Compose"
	@echo "  docker-stop  - Stop Docker Compose"
