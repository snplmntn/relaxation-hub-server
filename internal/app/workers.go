package app

import (
	"context"
	"sync"
)

type workerStarter interface {
	Start(context.Context)
}

type workerStopper interface {
	Stop()
}

type workerRegistration struct {
	name    string
	starter workerStarter
	stopper workerStopper
}

// WorkerManager centralizes background worker startup and shutdown.
type WorkerManager struct {
	ctx     context.Context
	cancel  context.CancelFunc
	workers []workerRegistration

	mu      sync.Mutex
	started bool
	wg      sync.WaitGroup
}

func NewWorkerManager(parent context.Context) *WorkerManager {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	return &WorkerManager{
		ctx:    ctx,
		cancel: cancel,
	}
}

func (m *WorkerManager) Context() context.Context {
	return m.ctx
}

func (m *WorkerManager) Add(name string, starter workerStarter, stopper workerStopper) {
	if starter == nil {
		return
	}
	m.workers = append(m.workers, workerRegistration{
		name:    name,
		starter: starter,
		stopper: stopper,
	})
}

func (m *WorkerManager) Start() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.started {
		return
	}
	m.started = true

	for _, worker := range m.workers {
		worker := worker
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			worker.starter.Start(m.ctx)
		}()
	}
}

func (m *WorkerManager) Shutdown(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	m.cancel()

	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()

	var shutdownErr error
	select {
	case <-done:
	case <-ctx.Done():
		shutdownErr = ctx.Err()
	}

	for _, worker := range m.workers {
		if worker.stopper != nil {
			worker.stopper.Stop()
		}
	}

	return shutdownErr
}
