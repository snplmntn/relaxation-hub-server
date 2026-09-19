# Load .env file if it exists
ifneq (,$(wildcard .env))
    include .env
    export
endif

.PHONY: run dev build test test-unit test-integration test-coverage clean help docs db-push db-push-dry-run sqlc-generate sqlc-vet

# Run the application

# Run the application
run:
	go run ./cmd/server/main.go

# Run with hot reload (requires air)
dev:
	air

# Build the application
build:
	go build -o server.exe ./cmd/server/main.go

# Run all tests
test:
	go test -v ./...



# Generate HTML coverage report
test-coverage-html:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run tests with race detection
test-race:
	go test -race ./...

# Run specific test
# Usage: make test-specific TEST=TestSignup_Success PKG=./internal/service
test-specific:
	go test -v -run $(TEST) $(PKG)

# Clean build artifacts and test files
clean:
	rm -f server.exe
	rm -f coverage.out
	rm -f coverage.html

# Install dependencies
deps:
	go mod download
	go mod tidy

# Format code
fmt:
	go fmt ./...

# Run linter (requires golangci-lint)
lint:
	golangci-lint run

# Generate sqlc repository code
sqlc-generate:
	sqlc generate

# Vet sqlc queries
sqlc-vet:
	sqlc vet

# Generate Mocks (requires mockery)
mocks:
	go run github.com/vektra/mockery/v2@v2.40.1 --all --keeptree --output ./tests/mocks --outpkg mocks

# Run unit tests only (fast, no artifacts)
test-unit:
	go test -v -short -race ./internal/...

# Run integration tests only (slow, requires Docker)
test-integration:
	go test -v -race ./tests/integration/...

# Run all tests with coverage
test-coverage:
	go test -race -coverprofile=coverage.out ./...

# Check coverage against threshold (99%)
test-coverage-check: test-coverage
	go tool cover -func=coverage.out | grep total | awk '{print ((int($$3) > 99) != 1)}'

# Serve API documentation
docs:
	npx redoc-cli serve docs/openapi.yaml

# Push DB migrations (uses DATABASE_URL from .env/environment)
db-push:
	powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\push_migrations.ps1

# Preview DB migrations without applying
db-push-dry-run:
	powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\push_migrations.ps1 -DryRun

# Show help
help:
	@echo "Available targets:"
	@echo "  docs                 - Serve API documentation"
	@echo "  run                  - Run the application"
	@echo "  dev                  - Run the application with hot reload (requires air)"
	@echo "  build                - Build the application"
	@echo "  test                 - Run all tests"
	@echo "  test-unit            - Run unit tests only"
	@echo "  test-integration     - Run integration tests only"
	@echo "  test-coverage        - Run tests with coverage"
	@echo "  test-coverage-html   - Generate HTML coverage report"
	@echo "  test-race            - Run tests with race detection"
	@echo "  test-specific        - Run specific test (TEST=name PKG=path)"
	@echo "  clean                - Clean build artifacts"
	@echo "  deps                 - Install dependencies"
	@echo "  fmt                  - Format code"
	@echo "  lint                 - Run linter"
	@echo "  sqlc-generate        - Generate sqlc repository code"
	@echo "  sqlc-vet             - Vet sqlc queries"
	@echo "  db-push              - Apply SQL migrations to DATABASE_URL"
	@echo "  db-push-dry-run      - Preview SQL migrations without applying"
	@echo "  help                 - Show this help message"
