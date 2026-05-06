# AGENTS.md (relaxation-hub-server)

## Scope
- Applies to `relaxation-hub-server/`.
- Inherits root `../AGENTS.md`; this file adds server-specific rules.

## Stack
- Go 1.25+
- Chi router
- PostgreSQL (pgx)

## Architecture Rules
- Preserve request flow: `internal/handler -> internal/service -> internal/repository`.
- Keep handlers HTTP-focused (parse, validate, respond); business rules stay in services.
- Keep repositories focused on persistence/query logic only.
- Add/extend interfaces before wiring implementations when introducing new dependencies.

## Data and Migrations
- Put schema changes in `internal/db/migrations/` as additive migration files.
- Do not silently alter historical migrations that may already be applied.
- Keep SQL changes backward-compatible with rolling deploy assumptions unless user requests otherwise.

## API and Security
- Enforce auth/RBAC checks on protected routes.
- Keep rate limiting for public/auth-sensitive endpoints.
- Validate request payloads and return consistent status/error shapes.
- Never hardcode secrets or tokens; use env-driven config.

## Validation
Run commands from `relaxation-hub-server/`:
- Fast baseline: `make test`
- Concurrency-sensitive changes: `make test-race`
- Integration-path changes (db/external behavior): `make test-integration` (Docker required)
- Formatting/lint before handoff when relevant: `make fmt`, `make lint`

## Change Notes in Final Response
- List changed files.
- List commands run and pass/fail status.
- Call out skipped validations and why.
