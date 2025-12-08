.PHONY: run build test test-unit test-integration test-coverage clean help

# Run the application
run:
	go run ./cmd/server/main.go

# Build the application
build:
	go build -o server.exe ./cmd/server/main.go

# Run all tests
test:
	go test -v ./...

# Run unit tests only
test-unit:
	go test -v ./internal/...

# Run integration tests only
test-integration:
	go test -v ./tests/integration/...

# Run tests with coverage
test-coverage:
	go test -cover ./...

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

# Show help
help:
	@echo "Available targets:"
	@echo "  run                  - Run the application"
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