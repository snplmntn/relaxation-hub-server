package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

const recurringWorkerInterval = 1 * time.Hour

// RecurringBookingWorker advances the generation horizon for all active recurring series.
type RecurringBookingWorker struct {
	recurringRepo   repository.RecurringBookingRepository
	recurringService *RecurringBookingService
	pollInterval    time.Duration
	stopCh          chan struct{}
}

// NewRecurringBookingWorker creates a new RecurringBookingWorker.
func NewRecurringBookingWorker(
	recurringRepo repository.RecurringBookingRepository,
	recurringService *RecurringBookingService,
) *RecurringBookingWorker {
	return &RecurringBookingWorker{
		recurringRepo:    recurringRepo,
		recurringService: recurringService,
		pollInterval:     recurringWorkerInterval,
		stopCh:           make(chan struct{}),
	}
}

// Start begins the background generation loop.
func (w *RecurringBookingWorker) Start(ctx context.Context) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("recurring booking worker panic recovered", "error", r)
			}
		}()

		ticker := time.NewTicker(w.pollInterval)
		defer ticker.Stop()

		// Run once immediately on startup
		w.run(ctx)

		for {
			select {
			case <-ticker.C:
				w.run(ctx)
			case <-w.stopCh:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Stop signals the worker to stop.
func (w *RecurringBookingWorker) Stop() {
	select {
	case w.stopCh <- struct{}{}:
	default:
	}
}

func (w *RecurringBookingWorker) run(ctx context.Context) {
	now := time.Now()
	series, err := w.recurringRepo.ListActiveForGeneration(ctx, now)
	if err != nil {
		slog.Error("recurring_worker: failed to list active series", "error", err)
		return
	}

	for i := range series {
		rec := series[i]
		if err := w.recurringService.AdvanceHorizon(ctx, &rec, now); err != nil {
			slog.Error("recurring_worker: failed to advance horizon",
				"recurring_id", rec.RecurringID, "error", err)
		}
	}
}
