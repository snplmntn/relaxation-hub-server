package app

import (
	"context"
	"testing"
	"time"
)

type recordingWorker struct {
	started chan struct{}
	stopped chan struct{}
}

func (w *recordingWorker) Start(ctx context.Context) {
	close(w.started)
	<-ctx.Done()
}

func (w *recordingWorker) Stop() {
	close(w.stopped)
}

func TestWorkerManagerStartAndShutdown(t *testing.T) {
	worker := &recordingWorker{
		started: make(chan struct{}),
		stopped: make(chan struct{}),
	}
	manager := NewWorkerManager(context.Background())
	manager.Add("recording", worker, worker)

	manager.Start()

	select {
	case <-worker.started:
	case <-time.After(time.Second):
		t.Fatal("expected worker to start")
	}

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
