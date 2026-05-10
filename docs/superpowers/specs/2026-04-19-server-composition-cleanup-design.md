# Server Composition Cleanup Design

Date: 2026-04-19
Scope: `server/`

## Goal

Refactor the Go server startup and low-risk hygiene surface so the codebase is easier to maintain, read, and test without changing API behavior, database schema, or booking-domain business rules.

## Current Problems

- `cmd/server/main.go` mixes configuration loading, database setup, dependency wiring, worker startup, middleware setup, route registration, HTTP server startup, and shutdown handling.
- Route and worker setup are difficult to inspect independently because they are embedded in one large entrypoint.
- Several tracked backup files with `.go.<number>` suffixes remain in `internal/`.
- Handler response helpers duplicate the shared `internal/response` package through a compatibility wrapper.
- `internal/service/booking_service.go` and `internal/repository/booking_repo.go` are large and need future decomposition, but changing them in this wave would increase regression risk.

## Non-Goals

- No booking service or booking repository decomposition in this wave.
- No API contract changes.
- No route removals, including existing compatibility shims.
- No database schema changes or migration files unless a compile or test failure proves one is required.
- No auth, RBAC, rate-limit, or request-size-limit weakening.

## Recommended Approach

Create an application composition layer and keep `cmd/server/main.go` as a thin executable entrypoint.

The new composition layer should own:

- dependency construction
- handler construction
- middleware registration
- route registration
- background worker construction and lifecycle
- shutdown cleanup hooks

`cmd/server/main.go` should own:

- config loading
- logger setup
- app construction
- HTTP server startup
- signal handling
- graceful shutdown

This preserves the existing request flow:

`handler -> service -> repository`

## Proposed Structure

- `internal/app/app.go`
  - Defines `App`, `Config`, and constructor functions.
  - Exposes the root `http.Handler`.
  - Exposes lifecycle methods such as `StartWorkers` and `Shutdown`.

- `internal/app/dependencies.go`
  - Builds repositories, services, handlers, providers, and shared adapters.
  - Groups related dependencies into typed structs instead of passing loose values through large functions.

- `internal/app/routes.go`
  - Registers global middleware, health routes, public API routes, authenticated routes, and admin routes.
  - Keeps existing route paths and compatibility shims.

- `internal/app/workers.go`
  - Starts assignment, completion, upcoming-booking, and rider-dispatch workers.
  - Centralizes worker context cancellation and wait-group management.

- `cmd/server/main.go`
  - Reduced to executable orchestration only.

Exact file names can change during implementation if the existing code shape suggests a cleaner split.

## Hygiene Cleanup

- Remove tracked backup files:
  - `internal/handler/location.go.7300643708241555806`
  - `internal/handler/websocket.go.8963988042758427283`
  - `internal/service/location_service.go.1840936057351746845`
- Keep `internal/handler/response.go` initially as a compatibility facade over `internal/response`.
- Only replace direct `json.NewEncoder` or `http.Error` usage in a narrow, safe slice if it is required by the composition refactor. Broad handler response standardization belongs in a later cleanup wave.

## Error Handling And Security

- Startup errors should return errors from constructors and be logged/fatal only in `cmd/server/main.go`.
- Middleware order should remain equivalent:
  - CORS
  - request logging
  - global rate limiting
  - body size limiting
  - public routes
  - auth-protected groups
  - role-protected groups
- Existing auth middleware, optional auth middleware, RBAC checks, ticket rate limiting, and public WebSocket token validation behavior must remain intact.

## Testing Plan

- Add focused tests for any newly exported route or app construction behavior where practical.
- Preserve existing handler, service, and repository tests.
- Run targeted package tests during the refactor.
- Before handoff, run:
  - `go test ./cmd/server ./internal/...`
  - `go test ./...` with a longer timeout if the package-level run succeeds
  - `gofmt` on changed Go files
- Run `make lint` only if `golangci-lint` is installed.

## Acceptance Criteria

- `cmd/server/main.go` is substantially smaller and focused on executable startup/shutdown.
- Application composition code is grouped by purpose and easier to inspect independently.
- All existing routes remain registered.
- Background workers still start and stop through a single lifecycle boundary.
- Tracked backup files are removed.
- No schema migration is added unless proven necessary.
- Relevant tests pass or any pre-existing/slow-test blockers are reported explicitly.

## Future Cleanup Waves

After this wave, split the booking domain in smaller reviewed steps:

- booking lifecycle and status transitions
- therapist assignment and offers
- session extension flow
- booking notifications and messaging helpers
- booking read models and repository scanners
- report/accounting query separation
