package app

import (
	"context"
	"errors"
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/snplmntn/relaxation-hub-server/internal/config"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	ws "github.com/snplmntn/relaxation-hub-server/internal/websocket"
)

type App struct {
	cfg     *config.Config
	pool    *pgxpool.Pool
	router  chi.Router
	hub     *ws.Hub
	workers *WorkerManager

	closeOnce sync.Once
}

func New(ctx context.Context, cfg *config.Config) (*App, error) {
	if cfg == nil {
		return nil, errors.New("app config is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	pool, err := db.InitDB(cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}

	workers := NewWorkerManager(ctx)
	hub := ws.NewHub()
	hub.SetPool(pool)

	deps, err := buildDependencies(ctx, cfg, pool, hub, workers)
	if err != nil {
		db.CloseDB(pool)
		return nil, err
	}

	router := chi.NewRouter()
	registerRoutes(router, deps)

	return &App{
		cfg:     cfg,
		pool:    pool,
		router:  router,
		hub:     hub,
		workers: workers,
	}, nil
}

func (a *App) Addr() string {
	return ":" + a.cfg.Port
}

func (a *App) Handler() http.Handler {
	return a.router
}

func (a *App) Start() {
	go a.hub.Run()
	a.workers.Start()
}

func (a *App) Shutdown(ctx context.Context) error {
	var workerErr error
	a.closeOnce.Do(func() {
		workerErr = a.workers.Shutdown(ctx)
		db.CloseDB(a.pool)
	})
	return workerErr
}
