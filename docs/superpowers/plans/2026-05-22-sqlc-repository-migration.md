# sqlc Repository Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (in-place default), or superpowers:subagent-driven-development plus superpowers:using-git-worktrees (isolated workspace), or superpowers:executing-plans (inline execution of the written plan). Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Introduce `sqlc` as a typed query generator behind the existing Go repository layer, then migrate `ServiceRepository` as the first production slice.

**Architecture:** Keep `handler -> service -> repository` unchanged. `sqlc` generated code lives under `internal/db/sqlc` and is called only by repository implementations; handlers and services continue depending on existing repository interfaces. Dynamic SQL and transaction-heavy repositories stay handwritten until their behavior is fully covered.

**Tech Stack:** Go 1.25+, PostgreSQL, pgx/v5, sqlc, existing integration tests under `tests/integration`.

---

## File Structure

- Create `server/sqlc.yaml`: sqlc configuration for PostgreSQL + pgx/v5 code generation.
- Create `server/internal/db/schema.sql`: code-generation schema for the service pilot, curated from `internal/db/migrations/001_init.sql`.
- Create `server/internal/db/queries/services.sql`: typed SQL queries for service catalog reads, create, and soft delete.
- Generate `server/internal/db/sqlc/*.go`: generated code, package `dbsqlc`.
- Modify `server/Makefile`: add `sqlc-generate` and optionally `sqlc-vet` targets.
- Modify `server/internal/repository/service_repo.go`: keep the public `ServiceRepository` interface, add a generated `Queries` field, migrate safe methods to sqlc, leave dynamic `Update` handwritten.
- Modify `server/tests/integration/service_repo_test.go`: add missing coverage for `ListRecentByUser` and `ListPopular` before replacing those methods.
- Do not modify `internal/handler`, `internal/service`, or `internal/app/dependencies.go` for the pilot unless a compile error proves it is necessary.

## Guardrails

- Do not replace `internal/db.DBTX` globally.
- Do not change repository interfaces during the pilot.
- Do not point sqlc directly at `internal/db/migrations/001_init.sql`; it contains extensions, triggers, `DO $$` blocks, seed data, RLS statements, and historical consolidation details that are not a stable code-generation schema.
- Do not migrate `BookingRepository`, wallet, ledger, payroll, assignment, ride dispatch, or worker queries in the first slice.
- Keep `ServiceRepository.Update` handwritten for the pilot because it currently accepts `map[string]interface{}` and builds whitelisted dynamic SQL from service-layer input.

---

### Task 1: Add Missing Service Repository Characterization Tests

**Files:**
- Modify: `server/tests/integration/service_repo_test.go`

- [ ] **Step 1: Add tests for recent and popular service queries**

Append these tests to `server/tests/integration/service_repo_test.go`:

```go
func TestServiceRepo_ListRecentByUser(t *testing.T) {
	pool := SetupTestDB(t)

	testhelpers.WithTransaction(t, pool, func(tx pgx.Tx) {
		repo := repository.NewServiceRepository(tx)
		ctx := context.Background()

		clientID := createServiceRepoUser(t, ctx, tx, "client")

		recent := &model.Service{Name: "Recent Pilot Service", BasePrice: 100, DurationMinutes: 60, Category: "Test", IsActive: true}
		old := &model.Service{Name: "Old Pilot Service", BasePrice: 100, DurationMinutes: 60, Category: "Test", IsActive: true}
		require.NoError(t, repo.Create(ctx, recent))
		require.NoError(t, repo.Create(ctx, old))

		_, err := tx.Exec(ctx, `
			INSERT INTO bookings (client_id, service_id, payment_method, status, scheduled_start, duration_minutes, created_at)
			VALUES
				($1, $2, 'cash', 'completed', NOW(), 60, NOW() - INTERVAL '2 days'),
				($1, $3, 'cash', 'completed', NOW(), 60, NOW() - INTERVAL '45 days')
		`, clientID, recent.ServiceID, old.ServiceID)
		require.NoError(t, err)

		services, err := repo.ListRecentByUser(ctx, clientID)
		require.NoError(t, err)

		require.NotEmpty(t, services)
		foundRecent := false
		for _, svc := range services {
			if svc.ServiceID == recent.ServiceID {
				foundRecent = true
			}
			assert.NotEqual(t, old.ServiceID, svc.ServiceID)
		}
		assert.True(t, foundRecent, "recent service should be returned")
	})
}

func TestServiceRepo_ListPopular(t *testing.T) {
	pool := SetupTestDB(t)

	testhelpers.WithTransaction(t, pool, func(tx pgx.Tx) {
		repo := repository.NewServiceRepository(tx)
		ctx := context.Background()

		clientID := createServiceRepoUser(t, ctx, tx, "client")
		popular := &model.Service{Name: "Popular Pilot Service", BasePrice: 100, DurationMinutes: 60, Category: "Test", IsActive: true}
		inactive := &model.Service{Name: "Inactive Popular Pilot Service", BasePrice: 100, DurationMinutes: 60, Category: "Test", IsActive: false}
		require.NoError(t, repo.Create(ctx, popular))
		require.NoError(t, repo.Create(ctx, inactive))

		_, err := tx.Exec(ctx, `
			INSERT INTO bookings (client_id, service_id, payment_method, status, scheduled_start, duration_minutes, created_at)
			VALUES
				($1, $2, 'cash', 'completed', NOW(), 60, NOW() - INTERVAL '1 day'),
				($1, $2, 'cash', 'completed', NOW(), 60, NOW() - INTERVAL '2 days'),
				($1, $3, 'cash', 'completed', NOW(), 60, NOW() - INTERVAL '1 day')
		`, clientID, popular.ServiceID, inactive.ServiceID)
		require.NoError(t, err)

		services, err := repo.ListPopular(ctx)
		require.NoError(t, err)

		require.NotEmpty(t, services)
		foundPopular := false
		for _, svc := range services {
			if svc.ServiceID == popular.ServiceID {
				foundPopular = true
			}
			assert.NotEqual(t, inactive.ServiceID, svc.ServiceID)
		}
		assert.True(t, foundPopular, "popular active service should be returned")
	})
}

func createServiceRepoUser(t *testing.T, ctx context.Context, tx pgx.Tx, role string) int64 {
	t.Helper()

	var userID int64
	err := tx.QueryRow(ctx, `
		INSERT INTO users (full_name, role, primary_email, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		RETURNING user_id
	`, "Service Repo User", role, "service-repo-"+role+"@example.com").Scan(&userID)
	require.NoError(t, err)

	return userID
}
```

- [ ] **Step 2: Run the new tests against the current handwritten repo**

Run from `server/`:

```powershell
go test -v -race ./tests/integration -run 'TestServiceRepo_(ListRecentByUser|ListPopular)'
```

Expected: PASS. These are characterization tests, so they should pass before the sqlc migration.

- [ ] **Step 3: Run the existing service repo tests**

Run from `server/`:

```powershell
go test -v -race ./tests/integration -run 'TestServiceRepo_'
```

Expected: PASS.

- [ ] **Step 4: Commit**

```powershell
git add tests/integration/service_repo_test.go
git commit -m "test: cover service catalog repository queries"
```

---

### Task 2: Add sqlc Configuration and Service Query Inputs

**Files:**
- Create: `server/sqlc.yaml`
- Create: `server/internal/db/schema.sql`
- Create: `server/internal/db/queries/services.sql`
- Modify: `server/Makefile`

- [ ] **Step 1: Create sqlc configuration**

Create `server/sqlc.yaml`:

```yaml
version: "2"
sql:
  - engine: postgresql
    schema:
      - internal/db/schema.sql
    queries:
      - internal/db/queries
    gen:
      go:
        package: dbsqlc
        out: internal/db/sqlc
        sql_package: pgx/v5
        emit_json_tags: true
        emit_db_tags: true
        emit_interface: true
        emit_empty_slices: true
        emit_pointers_for_null_types: true
        overrides:
          - column: services.service_id
            go_type: int64
          - column: bookings.booking_id
            go_type: int64
          - column: bookings.client_id
            go_type: int64
          - db_type: pg_catalog.numeric
            go_type: float64
          - db_type: pg_catalog.numeric
            nullable: true
            go_type:
              type: float64
              pointer: true
```

- [ ] **Step 2: Create the code-generation schema for the pilot**

Create `server/internal/db/schema.sql`:

```sql
CREATE TABLE services (
    service_id BIGSERIAL PRIMARY KEY,
    name VARCHAR(150) NOT NULL,
    description TEXT,
    category VARCHAR(50),
    preview_image_url TEXT,
    base_price NUMERIC(10,2) NOT NULL,
    therapist_commission NUMERIC(10,2) DEFAULT 0,
    duration_minutes INT NOT NULL DEFAULT 60,
    is_active BOOLEAN DEFAULT TRUE,
    deleted_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE bookings (
    booking_id BIGSERIAL PRIMARY KEY,
    client_id BIGINT NOT NULL,
    service_id BIGINT,
    payment_method VARCHAR(20),
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    scheduled_start TIMESTAMP,
    duration_minutes INT NOT NULL DEFAULT 60,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

- [ ] **Step 3: Create service query definitions**

Create `server/internal/db/queries/services.sql`:

```sql
-- name: CreateService :one
INSERT INTO services (
    name,
    description,
    base_price,
    duration_minutes,
    category,
    is_active,
    preview_image_url,
    therapist_commission
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
RETURNING *;

-- name: GetServiceByID :one
SELECT *
FROM services
WHERE service_id = $1
  AND deleted_at IS NULL;

-- name: GetServicesByIDs :many
SELECT *
FROM services
WHERE service_id = ANY(@ids::bigint[])
  AND deleted_at IS NULL;

-- name: ListActiveServices :many
SELECT *
FROM services
WHERE deleted_at IS NULL
  AND is_active = TRUE
ORDER BY name ASC;

-- name: ListRecentServicesByUser :many
SELECT s.*
FROM services s
INNER JOIN (
    SELECT service_id, MAX(created_at) AS last_booked
    FROM bookings
    WHERE client_id = $1
    GROUP BY service_id
) latest_b ON s.service_id = latest_b.service_id
WHERE s.deleted_at IS NULL
  AND latest_b.last_booked > NOW() - INTERVAL '30 days'
ORDER BY latest_b.last_booked DESC
LIMIT 3;

-- name: ListPopularServices :many
SELECT s.*
FROM services s
INNER JOIN (
    SELECT service_id, COUNT(booking_id) AS booking_count
    FROM bookings
    WHERE status = 'completed'
      AND created_at > NOW() - INTERVAL '30 days'
    GROUP BY service_id
) popular ON popular.service_id = s.service_id
WHERE s.deleted_at IS NULL
  AND s.is_active = TRUE
ORDER BY popular.booking_count DESC
LIMIT 3;

-- name: ListUnavailableServices :many
SELECT *
FROM services
WHERE is_active = FALSE
  AND deleted_at IS NULL
ORDER BY name ASC
LIMIT 3;

-- name: SoftDeleteService :execrows
UPDATE services
SET deleted_at = CURRENT_TIMESTAMP
WHERE service_id = $1
  AND deleted_at IS NULL;
```

- [ ] **Step 4: Add Makefile targets**

Modify `server/Makefile`:

```makefile
.PHONY: run dev build test test-unit test-integration test-coverage clean help docs db-push db-push-dry-run sqlc-generate sqlc-vet

sqlc-generate:
	sqlc generate

sqlc-vet:
	sqlc vet
```

Also add the new targets to the `help` output:

```makefile
	@echo "  sqlc-generate        - Generate typed database query code"
	@echo "  sqlc-vet             - Vet sqlc query definitions"
```

- [ ] **Step 5: Generate sqlc code**

Run from `server/`:

```powershell
sqlc generate
```

Expected: generated Go files appear under `internal/db/sqlc/`.

If `sqlc` is not installed, install it outside the repo and rerun:

```powershell
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
sqlc generate
```

- [ ] **Step 6: Compile generated package**

Run from `server/`:

```powershell
go test ./internal/db/sqlc
```

Expected: PASS.

- [ ] **Step 7: Commit**

```powershell
git add sqlc.yaml Makefile internal/db/schema.sql internal/db/queries/services.sql internal/db/sqlc
git commit -m "build: add sqlc service query generation"
```

---

### Task 3: Migrate ServiceRepository Reads, Create, and Delete to sqlc

**Files:**
- Modify: `server/internal/repository/service_repo.go`

- [ ] **Step 1: Add sqlc imports and repository field**

Modify the imports in `server/internal/repository/service_repo.go`:

```go
import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	dbsqlc "github.com/snplmntn/relaxation-hub-server/internal/db/sqlc"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)
```

Change the struct and constructor:

```go
type serviceRepo struct {
	db      db.DBTX
	queries *dbsqlc.Queries
}

func NewServiceRepository(db db.DBTX) ServiceRepository {
	return &serviceRepo{
		db:      db,
		queries: dbsqlc.New(db),
	}
}
```

- [ ] **Step 2: Add mapping helpers**

Add these helpers near the bottom of `service_repo.go` and remove the old `scanServices` helper after all callers are migrated:

```go
func serviceFromSQLC(s dbsqlc.Service) model.Service {
	return model.Service{
		ServiceID:           s.ServiceID,
		Name:                s.Name,
		Description:         stringValue(s.Description),
		BasePrice:           s.BasePrice,
		DurationMinutes:     int(s.DurationMinutes),
		Category:            stringValue(s.Category),
		PreviewImageURL:     stringValue(s.PreviewImageUrl),
		TherapistCommission: s.TherapistCommission,
		IsActive:            boolValue(s.IsActive),
		DeletedAt:           s.DeletedAt,
		CreatedAt:           timeValue(s.CreatedAt),
	}
}

func servicesFromSQLC(rows []dbsqlc.Service) []model.Service {
	services := make([]model.Service, 0, len(rows))
	for _, row := range rows {
		services = append(services, serviceFromSQLC(row))
	}
	return services
}

func stringValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func boolValue(v *bool) bool {
	return v != nil && *v
}

func timeValue(v *time.Time) time.Time {
	if v == nil {
		return time.Time{}
	}
	return *v
}
```

Add `time` to the import list if generated nullable timestamp fields require pointers:

```go
import "time"
```

If generated `created_at`, `deleted_at`, or `is_active` fields are not pointers after `sqlc generate`, simplify the helper to assign them directly.

- [ ] **Step 3: Migrate `Create`**

Replace `Create` with:

```go
func (r *serviceRepo) Create(ctx context.Context, svc *model.Service) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	row, err := r.queries.CreateService(ctx, dbsqlc.CreateServiceParams{
		Name:                svc.Name,
		Description:         &svc.Description,
		BasePrice:           svc.BasePrice,
		DurationMinutes:     int32(svc.DurationMinutes),
		Category:            &svc.Category,
		IsActive:            &svc.IsActive,
		PreviewImageUrl:     &svc.PreviewImageURL,
		TherapistCommission: svc.TherapistCommission,
	})
	if err != nil {
		return err
	}

	mapped := serviceFromSQLC(row)
	*svc = mapped
	return nil
}
```

- [ ] **Step 4: Migrate `GetByID`, `GetByIDs`, and list methods**

Replace the methods with:

```go
func (r *serviceRepo) GetByID(ctx context.Context, serviceID int64) (*model.Service, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	row, err := r.queries.GetServiceByID(ctx, serviceID)
	if err != nil {
		return nil, err
	}
	svc := serviceFromSQLC(row)
	return &svc, nil
}

func (r *serviceRepo) GetByIDs(ctx context.Context, ids []int64) ([]model.Service, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	if len(ids) == 0 {
		return []model.Service{}, nil
	}

	rows, err := r.queries.GetServicesByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	return servicesFromSQLC(rows), nil
}

func (r *serviceRepo) ListActive(ctx context.Context) ([]model.Service, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	rows, err := r.queries.ListActiveServices(ctx)
	if err != nil {
		return nil, err
	}
	return servicesFromSQLC(rows), nil
}

func (r *serviceRepo) ListRecentByUser(ctx context.Context, userID int64) ([]model.Service, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	rows, err := r.queries.ListRecentServicesByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	return servicesFromSQLC(rows), nil
}

func (r *serviceRepo) ListPopular(ctx context.Context) ([]model.Service, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	rows, err := r.queries.ListPopularServices(ctx)
	if err != nil {
		return nil, err
	}
	return servicesFromSQLC(rows), nil
}

func (r *serviceRepo) ListUnavailable(ctx context.Context) ([]model.Service, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	rows, err := r.queries.ListUnavailableServices(ctx)
	if err != nil {
		return nil, err
	}
	return servicesFromSQLC(rows), nil
}
```

- [ ] **Step 5: Migrate `Delete`**

Replace `Delete` with:

```go
func (r *serviceRepo) Delete(ctx context.Context, serviceID int64) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	rowsAffected, err := r.queries.SoftDeleteService(ctx, serviceID)
	if err != nil {
		return fmt.Errorf("failed to delete service: %w", err)
	}
	if rowsAffected == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
```

- [ ] **Step 6: Keep `Update` handwritten and validate imports**

Leave `Update` using dynamic SQL. After the old scanning helper is removed, keep only imports used by the file.

- [ ] **Step 7: Run focused compile checks**

Run from `server/`:

```powershell
go test ./internal/repository
go test ./internal/service -run ServiceCatalog
```

Expected: PASS.

- [ ] **Step 8: Run service integration tests**

Run from `server/`:

```powershell
go test -v -race ./tests/integration -run 'TestServiceRepo_'
go test -v -race ./tests/integration -run 'TestIntegration_(ListServices|CreateService_AdminOnly|CreateService_ClientForbidden)'
```

Expected: PASS.

- [ ] **Step 9: Commit**

```powershell
git add internal/repository/service_repo.go
git commit -m "refactor: use sqlc for service repository queries"
```

---

### Task 4: Add sqlc to the Normal Validation Path

**Files:**
- Modify: `server/Makefile`
- Optionally modify: `.github/workflows/*` if CI exists and already runs backend checks

- [ ] **Step 1: Include sqlc generation in the backend validation command**

Modify the `test` target in `server/Makefile`:

```makefile
test: sqlc-generate
	go test -v ./...
```

If this is too broad for local iteration, add a separate target instead:

```makefile
verify: sqlc-generate
	go test -v ./...
```

- [ ] **Step 2: Verify generated code is stable**

Run from `server/`:

```powershell
sqlc generate
git diff -- internal/db/sqlc
```

Expected: no diff after generation.

- [ ] **Step 3: Run backend unit baseline**

Run from `server/`:

```powershell
make test-unit
```

Expected: PASS.

- [ ] **Step 4: Run service integration baseline**

Run from `server/`:

```powershell
go test -v -race ./tests/integration -run 'TestServiceRepo_|TestIntegration_(ListServices|CreateService_AdminOnly|CreateService_ClientForbidden)'
```

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add Makefile
git commit -m "build: include sqlc in backend verification"
```

---

### Task 5: Decide Whether to Expand Beyond the Pilot

**Files:**
- No code changes required unless expanding.
- Create follow-up plan files for each accepted repository cluster.

- [ ] **Step 1: Review pilot outcome**

Use these criteria:

```text
Keep expanding if:
- service_repo.go is smaller or clearer after migration
- sqlc type conversion is localized to repository helpers
- integration tests caught no behavioral drift
- generated code is stable and easy to regenerate

Stop or pause if:
- schema.sql becomes hard to keep aligned with real migrations
- nullable/numeric conversion creates noisy repository code
- dynamic queries are being forced into awkward query duplication
- generated code introduces more churn than handwritten pgx scans remove
```

- [ ] **Step 2: Pick the next repository cluster**

Recommended expansion order:

```text
1. BlogPostRepository or LegalDocumentRepository
2. AddressRepository, after adding transaction coverage around default promotion
3. BranchRepository read/create/delete paths, leaving map-based Update handwritten or refactoring it separately
4. Notification single-row/list paths
5. Payment proof/read-only paths
6. Booking, assignment queue, wallet, ledger, payroll, and ride dispatch only after separate plans
```

- [ ] **Step 3: Document the pilot decision**

Create `server/docs/sqlc-pilot.md`:

```markdown
# sqlc Pilot Result

## Scope
- Migrated: ServiceRepository create/read/list/delete methods.
- Left handwritten: ServiceRepository.Update because it uses dynamic map-based update SQL.

## Decision
- Status before validation: pending.
- Continue expanding when all evidence lines below say `PASS` and repository code is clearer after review.
- Pause expansion when any evidence line says `FAIL` or generated/schema maintenance is more complex than the handwritten repository it replaced.

## Evidence
- `sqlc generate`: not run when document is created.
- `go test -v -race ./tests/integration -run 'TestServiceRepo_'`: not run when document is created.
- `make test-unit`: not run when document is created.

## Notes
- Numeric handling: `NUMERIC` maps to `float64` for this pilot to preserve current `model.Service` behavior.
- Nullable handling: nullable strings are converted to empty strings at the repository boundary to preserve existing `COALESCE` behavior.
- Schema maintenance: `internal/db/schema.sql` is a code-generation schema and must be updated when migrated query areas change.
```

- [ ] **Step 4: Commit**

```powershell
git add docs/sqlc-pilot.md
git commit -m "docs: record sqlc pilot decision"
```

---

## Final Verification

Run from `server/` after all pilot tasks:

```powershell
sqlc generate
go test ./internal/db/sqlc
go test ./internal/repository
go test ./internal/service -run ServiceCatalog
go test -v -race ./tests/integration -run 'TestServiceRepo_'
go test -v -race ./tests/integration -run 'TestIntegration_(ListServices|CreateService_AdminOnly|CreateService_ClientForbidden)'
make test-unit
make lint
```

Expected:

```text
All commands pass.
Generated sqlc files have no diff after rerunning sqlc generate.
No handler/service interface changes were required.
```

## Known Risks

- `sqlc` is not currently installed on this machine, so the first implementation task may need to install it before generation can be verified.
- The curated `internal/db/schema.sql` must stay aligned with real migrations for migrated query areas.
- `NUMERIC` fields currently map to `float64` to preserve existing model behavior; this is compatible with the current code but should be revisited separately if money handling is hardened.
- `ServiceRepository.Update` remains handwritten until the service update API is converted from `map[string]interface{}` to a typed update struct.
- Full integration tests require Docker-backed PostgreSQL in this repo.
