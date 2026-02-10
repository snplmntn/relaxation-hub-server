# PLAN: 100% Test Coverage for Relaxation Hub Server (SOTA 2026)

> **Objective**: Achieve 100% code coverage using State-of-the-Art (SOTA) practices for the `relaxation-hub-server` backend.

## 1. Audit & Gap Analysis (Updated)
- **Critical Gap**: `internal/handler/booking.go` is 43KB but `booking_test.go` is only 2KB. This indicates **severe lack of coverage** in the most complex business logic.
- **Micro-Tests**: Many handlers have `<1KB` test files, suggesting only "happy path" or placebo tests.
- **Dependency Graph**: Handlers depend on Services, which depend on Repositories. Tests must respect these boundaries.

## 2. SOTA Strategy (2026 Standards)

### A. Testing Pyramid & Isolation
1.  **Repository Layer (Integration)**:
    -   **Tool**: **Testcontainers for Go** (Ephemeral Postgres).
    -   **Why**: Ensure SQL queries, constraints, and cascading deletes work on a REAL DB.
    -   **Strict Rule**: No mocks allowed here.
2.  **Service Layer (Pure Unit)**:
    -   **Tool**: `mockery` (for Repository interfaces) + `testify/table`.
    -   **Why**: Prove business logic (calculations, state transitions) works 100% without DB slowness.
    -   **Target**: 100% Branch Coverage.
3.  **Handler/API Layer (Integration/Unit)**:
    -   **Tool**: `httptest` + `mockery` (for Service interfaces).
    -   **Why**: Verify HTTP status codes, JSON serialization, and Middleware (Auth/Role).

### B. Critical Path Prioritization
1.  **Tier 1 (High Risk)**: `booking`, `payment`, `auth`, `rider` (Earnings/Safety).
2.  **Tier 2 (Core)**: `service`, `therapist`, `user`, `location`.
3.  **Tier 3 (Support)**: `notification`, `review`, `report`.

## 3. Implementation Phases

### Phase 1: Foundation (Agent: `backend-specialist` + `devops-engineer`)
- [ ] **Infrastructure**: Create `tests/testhelpers` with `Testcontainers` setup (Postgres + Redis).
- [ ] **Tooling**: Install `mockery`. Add `make mocks` target.
- [ ] **CI/CD**: Add `make test-coverage-check` that fails if coverage < 99%.

### Phase 2: Repository Layer (Agent: `db-specialist`)
- [ ] **Objective**: Verify all SQL interactions.
- [ ] **Focus**: `booking_repository`, `rider_repository`, `payment_repository`.
- [ ] **Pattern**: `RunTestDB(t)` helper that spins up container per package.

### Phase 3: Service Layer (Agent: `backend-specialist`)
- [ ] **Objective**: The heavy lifting. 100% logic coverage.
- [ ] **Focus**: `BookingService` (Complex state machine), `RideMatchingService` (Algorithm).
- [ ] **Technique**: Table-driven tests covering every `if/else` branch.

### Phase 4: Handler Layer (Agent: `backend-specialist`)
- [ ] **Objective**: API Contract verification.
- [ ] **Focus**: Auth flows, Error handling (400 vs 500), Role access control.

### Phase 5: Advanced Stabilization (Agent: `test-engineer`)
- [ ] **Fuzzing**: Native Go Fuzzing for `PriceCalculation` and `InputValidation`.
- [ ] **Race Detection**: Run full suite with `-race`.

## 4. Verification Plan
- **Pre-Flight**: Run `make dev` to ensure no regression.
- **Execution**:
    1.  `make mocks` (Generate all mocks)
    2.  `make test-integration` (Slow, deep)
    3.  `make test-unit` (Fast, logical)
    4.  `make test-coverage-html` -> Check report.
- **Success Metrics**:
    -   Global Coverage > 99%.
    -   Booking/Payment Coverage = 100%.

## 5. Workflows & Agents
| Stage | Agents | Responsibilities |
|-------|--------|------------------|
| **1. Foundation** | `backend-specialist` | Setup Testcontainers, Mockery. |
| **2. Coverage Sprint** | `backend-specialist` (x2), `test-engineer` | Parallel execution on Tier 1 & 2 modules. |
| **3. Verification** | `security-auditor` | Security audit of Auth tests. |

## 6. User Review Required
> [!IMPORTANT]
> **Docker Required**: Testcontainers needs a local Docker daemon.
> **Mockery Generation**: We will modify interfaces if needed to be more mockable (SOL.I.D principles).
