# Run Tests Script
# Execute comprehensive test suite for Relaxation Hub Server

Write-Host "================================" -ForegroundColor Cyan
Write-Host "Relaxation Hub - Test Suite" -ForegroundColor Cyan
Write-Host "================================" -ForegroundColor Cyan
Write-Host ""

# Check if Go is installed
Write-Host "[1/6] Checking Go installation..." -ForegroundColor Yellow
$goVersion = go version 2>$null
if ($LASTEXITCODE -ne 0) {
    Write-Host "❌ Go is not installed or not in PATH" -ForegroundColor Red
    exit 1
}
Write-Host "✓ $goVersion" -ForegroundColor Green
Write-Host ""

# Check dependencies
Write-Host "[2/6] Checking dependencies..." -ForegroundColor Yellow
go mod download
if ($LASTEXITCODE -ne 0) {
    Write-Host "❌ Failed to download dependencies" -ForegroundColor Red
    exit 1
}
Write-Host "✓ Dependencies OK" -ForegroundColor Green
Write-Host ""

# Run unit tests
Write-Host "[3/6] Running unit tests..." -ForegroundColor Yellow
Write-Host "Testing: internal/service, internal/handler, internal/middleware, internal/oauth" -ForegroundColor Gray
go test -v ./internal/...
$unitTestResult = $LASTEXITCODE
Write-Host ""

if ($unitTestResult -eq 0) {
    Write-Host "✓ Unit tests PASSED" -ForegroundColor Green
} else {
    Write-Host "❌ Unit tests FAILED" -ForegroundColor Red
}
Write-Host ""

# Run integration tests
Write-Host "[4/6] Running integration tests..." -ForegroundColor Yellow
Write-Host "Note: These tests will skip if test database is not available" -ForegroundColor Gray
go test -v ./tests/integration/...
$integrationTestResult = $LASTEXITCODE
Write-Host ""

if ($integrationTestResult -eq 0) {
    Write-Host "✓ Integration tests PASSED (or skipped)" -ForegroundColor Green
} else {
    Write-Host "⚠ Integration tests had issues (check if test DB is configured)" -ForegroundColor Yellow
}
Write-Host ""

# Generate coverage report
Write-Host "[5/6] Generating coverage report..." -ForegroundColor Yellow
go test -coverprofile=coverage.out ./... 2>$null
if ($LASTEXITCODE -eq 0) {
    $coverage = go tool cover -func=coverage.out | Select-String "total:" | ForEach-Object { $_.ToString().Split()[2] }
    Write-Host "✓ Coverage: $coverage" -ForegroundColor Green
    
    # Generate HTML report
    go tool cover -html=coverage.out -o coverage.html
    Write-Host "✓ HTML report generated: coverage.html" -ForegroundColor Green
} else {
    Write-Host "⚠ Coverage report generation had issues" -ForegroundColor Yellow
}
Write-Host ""

# Summary
Write-Host "[6/6] Test Summary" -ForegroundColor Yellow
Write-Host "================================" -ForegroundColor Cyan

$totalTests = 0
$passedTests = 0
$failedTests = 0

if ($unitTestResult -eq 0) {
    Write-Host "✓ Unit Tests:        PASSED" -ForegroundColor Green
    $passedTests++
} else {
    Write-Host "❌ Unit Tests:        FAILED" -ForegroundColor Red
    $failedTests++
}
$totalTests++

if ($integrationTestResult -eq 0) {
    Write-Host "✓ Integration Tests: PASSED/SKIPPED" -ForegroundColor Green
    $passedTests++
} else {
    Write-Host "⚠ Integration Tests: ISSUES" -ForegroundColor Yellow
    $failedTests++
}
$totalTests++

Write-Host ""
Write-Host "Total: $totalTests | Passed: $passedTests | Failed: $failedTests" -ForegroundColor Cyan
Write-Host "================================" -ForegroundColor Cyan
Write-Host ""

# Exit with appropriate code
if ($failedTests -eq 0) {
    Write-Host "🎉 All tests passed!" -ForegroundColor Green
    Write-Host ""
    Write-Host "Next steps:" -ForegroundColor Cyan
    Write-Host "  1. Open coverage.html to view detailed coverage report" -ForegroundColor Gray
    Write-Host "  2. Run 'make test' anytime to run tests again" -ForegroundColor Gray
    Write-Host "  3. See QA_TESTING_GUIDE.md for manual testing scenarios" -ForegroundColor Gray
    exit 0
} else {
    Write-Host "⚠ Some tests failed. Please review the output above." -ForegroundColor Yellow
    exit 1
}
