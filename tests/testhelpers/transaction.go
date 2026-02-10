package testhelpers

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// WithTransaction wraps a test in a transaction that automatically rolls back.
// This ensures complete test isolation with zero data pollution.
//
// SOTA Practice: Transaction-based test isolation is the gold standard for integration tests.
// Each test gets a clean slate and leaves no trace behind.
func WithTransaction(t *testing.T, pool *pgxpool.Pool, testFunc func(tx pgx.Tx)) {
	ctx := context.Background()
	
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("failed to start transaction: %v", err)
	}

	// Ensure rollback happens even if test panics
	defer func() {
		if err := tx.Rollback(ctx); err != nil && err != pgx.ErrTxClosed {
			t.Logf("Warning: failed to rollback transaction: %v", err)
		}
	}()

	// Run the test
	testFunc(tx)
}
