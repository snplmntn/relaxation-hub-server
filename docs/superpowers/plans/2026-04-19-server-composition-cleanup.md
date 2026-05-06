# Server Composition Cleanup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (in-place default), or superpowers:subagent-driven-development plus superpowers:using-git-worktrees (isolated workspace), or superpowers:executing-plans (inline execution of the written plan). Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move server startup composition out of `cmd/server` into a focused `internal/app` package while preserving routes, workers, auth, rate limiting, and API behavior.

**Architecture:** `cmd/server/main.go` becomes the executable shell for config, logger, HTTP server startup, signal handling, and graceful shutdown. `internal/app` owns database/bootstrap, dependency wiring, route registration, health handling, and worker lifecycle. The existing `handler -> service -> repository` request flow remains unchanged.

**Tech Stack:** Go 1.25+, Chi, pgx, existing internal handler/service/repository packages.

---

### Task 1: Package Boundary Tests

**Files:**
- Create: `internal/app/health_test.go`
- Create: `internal/app/workers_test.go`
- Move later: `cmd/server/health.go` -> `internal/app/health.go`
- Move later: `cmd/server/health_test.go` -> `internal/app/health_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestHealthHandlerReportsDependencySnapshot(t *testing.T) {
	h := NewHealthHandler(stubDependencyHealthProvider{snapshot: handler.ReportDependencySnapshot{Status: "degraded", Degraded: true}})
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	h(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
}

func TestWorkerManagerStartAndShutdown(t *testing.T) {
	worker := &recordingWorker{stopped: make(chan struct{})}
	manager := NewWorkerManager(context.Background())
	manager.Add("recording", worker, worker)
	manager.Start()
	requireEventuallyStarted(t, worker)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := manager.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}
	select {
	case <-worker.stopped:
	default:
		t.Fatal("expected worker Stop to be called")
	}
}
```

- [ ] **Step 2: Verify red**

Run: `go test ./internal/app`

Expected: FAIL because package `internal/app` and its exported functions do not exist.

### Task 2: App Package

**Files:**
- Create: `internal/app/app.go`
- Create: `internal/app/dependencies.go`
- Create: `internal/app/routes.go`
- Create: `internal/app/workers.go`
- Create: `internal/app/health.go`
- Modify: `cmd/server/main.go`
- Remove: `cmd/server/health.go`
- Remove: `cmd/server/health_test.go`

- [ ] **Step 1: Implement package API**

```go
type App struct {
	cfg     *config.Config
	pool    *pgxpool.Pool
	router  chi.Router
	hub     *ws.Hub
	workers *WorkerManager
	closeOnce sync.Once
}

func New(ctx context.Context, cfg *config.Config) (*App, error)
func (a *App) Addr() string
func (a *App) Handler() http.Handler
func (a *App) Start()
func (a *App) Shutdown(ctx context.Context) error
```

- [ ] **Step 2: Move dependency construction**

Create `buildDependencies(ctx context.Context, cfg *config.Config, pool *pgxpool.Pool, hub *ws.Hub, workers *WorkerManager) (*dependencies, error)` and move the existing repository, service, handler, OAuth, provider, report dependency, and worker construction code into it without changing constructor arguments.

- [ ] **Step 3: Move route registration**

Create `func registerRoutes(r chi.Router, deps *dependencies)` and move existing middleware and route registrations into it without removing compatibility shims.

- [ ] **Step 4: Replace executable shell**

Replace `cmd/server/main.go` with a thin entrypoint that loads config, configures `slog`, constructs `app.New`, starts workers/hub, creates `http.Server`, handles signals, shuts down HTTP, then calls `App.Shutdown`.

- [ ] **Step 5: Verify green**

Run: `go test ./internal/app ./cmd/server`

Expected: PASS or compile errors only from moved imports, which must be corrected before continuing.

### Task 3: Hygiene Cleanup

**Files:**
- Delete: `internal/handler/location.go.7300643708241555806`
- Delete: `internal/handler/websocket.go.8963988042758427283`
- Delete: `internal/service/location_service.go.1840936057351746845`

- [ ] **Step 1: Delete tracked backup files**

Use Git-tracked deletion so the files can be restored from the branch history if needed.

- [ ] **Step 2: Verify no backup files remain**

Run: `Get-ChildItem internal -Recurse -Filter '*.go.*'`

Expected: no tracked backup source files remain.

### Task 4: Formatting And Validation

**Files:**
- Modify only changed Go files.

- [ ] **Step 1: Format**

Run: `gofmt -w cmd/server/main.go internal/app/*.go`

Expected: exit 0.

- [ ] **Step 2: Focused tests**

Run: `go test ./internal/app ./cmd/server`

Expected: exit 0.

- [ ] **Step 3: Server package tests**

Run: `go test ./cmd/server ./internal/...`

Expected: exit 0 or report existing slow/flaky blockers with exact failures.

- [ ] **Step 4: Full tests**

Run: `go test ./...` with an extended timeout.

Expected: exit 0 or report timeout/failing packages precisely.
