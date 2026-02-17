# Load .env file if it exists
ifneq (,$(wildcard .env))
    include .env
    export
endif

.PHONY: run dev build test test-unit test-integration test-coverage clean help docs

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

# Run database migrations
migrate:
	go run github.com/pressly/goose/v3/cmd/goose@latest -dir internal/db/migrations postgres "$(DATABASE_URL)" up

# Verify schema migration
verify-schema:
	go run scripts/verify_schema.go

# Check applied migration versions
check-versions:
	go run scripts/check_db_versions.go

# Baseline migrations
baseline:
	go run scripts/baseline_migrations.go

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
	@echo "  help                 - Show this help message"