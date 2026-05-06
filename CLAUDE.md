# CLAUDE.md (relaxation-hub-server)

## Scope
- Applies to `relaxation-hub-server/`.
- Builds on the global `../CLAUDE.md` and adds Go-specific reminders.

## Persona & Workflow
- Role: Senior Go backend engineer.
- Keep the same concise, no-frills tone from the global file.
- Follow the global "Plan -> Think -> Act" mindset for anything non-trivial.

## Go-Specific Best Practices
- **Data-layer-first**: define models/interfaces in `internal/model`, then repository contracts, services, and finally handlers.
- **Handler discipline**: keep handlers focused on parsing/validation/response; business rules belong to `internal/service` and persistence to `internal/repository`.
- **Strong typing**: prefer domain-specific structs with validation tags rather than anonymous maps; return typed errors.
- **DB/migrations**: add additive SQL files inside `internal/db/migrations`; avoid touching historical migrations unless explicitly requested.
- **Security**: keep auth/RBAC/rate limiting in place for public endpoints; never widen defaults without explicit approval.

## Development Notes
- Favor explicit dependency injection over global variables when wiring services/repositories.
- Keep API responses consistent and annotated; share response DTO definitions with the web team when possible.
