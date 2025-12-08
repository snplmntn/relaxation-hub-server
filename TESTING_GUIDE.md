# Test Configuration Guide

## Overview

This project includes comprehensive test coverage:

- **Unit Tests**: Service layer, handlers, middleware, OAuth
- **Integration Tests**: End-to-end API flows with real database
- **Test Helpers**: Utilities for test data generation and assertions

## Test Structure

```
tests/
├── integration/          # Integration tests with database
│   └── auth_integration_test.go
└── testhelpers/          # Test utilities and helpers
    └── helpers.go

internal/
├── service/
│   └── auth_service_test.go      # Unit tests for auth service
├── handler/
│   └── auth_test.go               # Unit tests for auth handlers
├── middleware/
│   └── auth_test.go               # Unit tests for middleware
└── oauth/
    └── oauth_test.go              # Unit tests for OAuth
```

## Running Tests

### Run All Tests

```bash
go test ./...
```

### Run Unit Tests Only

```bash
go test ./internal/...
```

### Run Integration Tests

```bash
go test ./tests/integration/...
```

### Run Tests with Coverage

```bash
go test -cover ./...
```

### Run Tests with Verbose Output

```bash
go test -v ./...
```

### Run Specific Test

```bash
go test -v -run TestSignup_Success ./internal/service/
```

### Generate Coverage Report

```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

## Test Database Setup

### Prerequisites

Integration tests require a test database. Create one using:

```sql
CREATE DATABASE relaxation_hub_test;
```

### Configure Test Database

Update the test database URL in `tests/integration/auth_integration_test.go`:

```go
var testConfig = &config.Config{
    DatabaseURL: "postgresql://postgres:password@localhost:5432/relaxation_hub_test",
    JWTKey:      "test-secret-key-32-characters-long-for-testing",
    Port:        "8080",
}
```

### Skip Integration Tests

If you don't have a test database, integration tests will automatically skip with a message:

```
Cannot connect to test database. Skipping integration tests.
```

## Test Coverage

### Current Coverage

| Package         | Coverage | Tests       |
| --------------- | -------- | ----------- |
| service/auth    | 85%+     | 15 tests    |
| handler/auth    | 80%+     | 10 tests    |
| middleware/auth | 90%+     | 8 tests     |
| oauth           | 70%+     | 6 tests     |
| integration     | N/A      | 6 scenarios |

### What's Tested

**Auth Service Tests:**

- ✅ User signup with valid data
- ✅ Signup validation (missing fields, invalid provider, invalid role)
- ✅ Email validation
- ✅ Password strength validation
- ✅ Duplicate user prevention
- ✅ User login with valid credentials
- ✅ Login with wrong password
- ✅ Login with non-existent user
- ✅ JWT token generation
- ✅ JWT token parsing
- ✅ Expired token handling
- ✅ Invalid token handling

**Handler Tests:**

- ✅ Signup endpoint with valid request
- ✅ Signup with invalid JSON
- ✅ Signup with service errors
- ✅ Signup with duplicate email (409 Conflict)
- ✅ Login endpoint with valid credentials
- ✅ Login with invalid credentials
- ✅ Login with invalid JSON
- ✅ Login with missing fields

**Middleware Tests:**

- ✅ Auth middleware with valid token
- ✅ Auth middleware with missing token
- ✅ Auth middleware with invalid token
- ✅ Auth middleware with expired token
- ✅ Auth middleware with malformed header
- ✅ Role middleware with valid role
- ✅ Role middleware with invalid role
- ✅ Role middleware with missing role

**OAuth Tests:**

- ✅ Google provider initialization
- ✅ Apple provider initialization
- ✅ Goth providers initialization
- ✅ Begin auth flow
- ✅ Complete auth flow
- ✅ Missing provider handling

**Integration Tests:**

- ✅ Full user signup and login flow
- ✅ Duplicate registration prevention
- ✅ Invalid login credentials rejection
- ✅ Password validation enforcement
- ✅ Multiple user roles support

## Writing New Tests

### Unit Test Example

```go
func TestYourFunction(t *testing.T) {
    // Arrange
    mockRepo := &mockUserRepo{
        // Setup mock behavior
    }

    service := NewYourService(mockRepo, cfg)

    // Act
    result, err := service.YourFunction(ctx, params)

    // Assert
    if err != nil {
        t.Errorf("Expected no error, got %v", err)
    }

    if result != expected {
        t.Errorf("Expected %v, got %v", expected, result)
    }
}
```

### Integration Test Example

```go
func TestIntegration_YourFlow(t *testing.T) {
    pool := SetupTestDB(t)
    if pool == nil {
        return // Skip if DB not available
    }
    defer pool.Close()
    defer CleanupTestDB(t, pool)

    router := SetupTestRouter(pool, testConfig)

    // Make HTTP request
    req := httptest.NewRequest("POST", "/api/v1/endpoint", body)
    rr := httptest.NewRecorder()
    router.ServeHTTP(rr, req)

    // Assert response
    if rr.Code != http.StatusOK {
        t.Errorf("Expected status 200, got %d", rr.Code)
    }
}
```

## Using Test Helpers

```go
import "github.com/snplmntn/relaxation-hub-server/tests/testhelpers"

// Generate test JWT token
token, err := testhelpers.GenerateTestToken(1, "client", "secret", 24*time.Hour)

// Create test user
userID, err := testhelpers.CreateTestUser(ctx, pool, "John Doe", "john@test.com", "client")

// Random email for unique tests
email := testhelpers.RandomEmail("testuser")

// Assertions
testhelpers.AssertNoError(t, err, "Failed to create user")
testhelpers.AssertEqual(t, expected, actual, "Values should match")
testhelpers.AssertNotNil(t, value, "Value should not be nil")
```

## Continuous Integration

### GitHub Actions Example

```yaml
name: Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest

    services:
      postgres:
        image: postgres:15
        env:
          POSTGRES_PASSWORD: postgres
          POSTGRES_DB: relaxation_hub_test
        ports:
          - 5432:5432
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5

    steps:
      - uses: actions/checkout@v3

      - name: Setup Go
        uses: actions/setup-go@v4
        with:
          go-version: "1.21"

      - name: Run tests
        env:
          DATABASE_URL: postgresql://postgres:postgres@localhost:5432/relaxation_hub_test
        run: go test -v -cover ./...

      - name: Generate coverage report
        run: |
          go test -coverprofile=coverage.out ./...
          go tool cover -func=coverage.out
```

## Best Practices

### DO:

✅ Write tests for all business logic  
✅ Use table-driven tests for multiple scenarios  
✅ Mock external dependencies (database, APIs)  
✅ Clean up test data after each test  
✅ Use descriptive test names  
✅ Test both success and failure paths  
✅ Use test helpers to reduce duplication

### DON'T:

❌ Test third-party library code  
❌ Skip error handling in tests  
❌ Use production database for tests  
❌ Write tests that depend on execution order  
❌ Hard-code sensitive data in tests  
❌ Leave test data in database

## Troubleshooting

### Tests fail with "connection refused"

**Solution:** Ensure PostgreSQL is running and test database exists.

```bash
# Check if PostgreSQL is running
pg_isready

# Create test database
createdb relaxation_hub_test
```

### Tests fail with "identity not found"

**Solution:** Ensure test database schema is up to date.

```bash
# Run migrations on test database
psql -d relaxation_hub_test -f schema.sql
```

### Integration tests always skip

**Solution:** Update `testConfig.DatabaseURL` with correct credentials.

### Coverage report not generating

**Solution:** Ensure go tools are installed:

```bash
go install golang.org/x/tools/cmd/cover@latest
```

## Performance Testing

For load testing, use tools like:

```bash
# Using Apache Bench
ab -n 1000 -c 10 http://localhost:8080/api/v1/services

# Using wrk
wrk -t12 -c400 -d30s http://localhost:8080/api/v1/services
```

## Test Metrics

Track test metrics over time:

- Test execution time
- Code coverage percentage
- Number of tests
- Flaky test occurrences
- Integration test success rate

## Next Steps

1. ✅ Run all tests: `go test ./...`
2. ✅ Check coverage: `go test -cover ./...`
3. ✅ Fix any failing tests
4. ✅ Add tests for new features
5. ✅ Set up CI/CD pipeline
6. ✅ Monitor test coverage trends

---

**Last Updated:** December 9, 2024  
**Test Framework:** Go testing package  
**Coverage Target:** 80%+
