package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PoolLogAttrs returns pgx pool pressure details when source is a pool.
func PoolLogAttrs(source any) []any {
	pool, ok := source.(interface{ Stat() *pgxpool.Stat })
	if !ok {
		return nil
	}
	stats := pool.Stat()
	return []any{
		"pool_max_conns", stats.MaxConns(),
		"pool_total_conns", stats.TotalConns(),
		"pool_acquired_conns", stats.AcquiredConns(),
		"pool_idle_conns", stats.IdleConns(),
		"pool_constructing_conns", stats.ConstructingConns(),
		"pool_empty_acquires", stats.EmptyAcquireCount(),
		"pool_canceled_acquires", stats.CanceledAcquireCount(),
	}
}

// Query timeout constants for different operation types
const (
	DefaultQueryTimeout = 5 * time.Second
	LongQueryTimeout    = 30 * time.Second
	TransactionTimeout  = 60 * time.Second
)

// WithQueryTimeout wraps a context with a default query timeout
func WithQueryTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, DefaultQueryTimeout)
}

// WithLongQueryTimeout wraps a context with a longer timeout for complex queries
func WithLongQueryTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, LongQueryTimeout)
}

// WithTransactionTimeout wraps a context with timeout for transaction operations
func WithTransactionTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, TransactionTimeout)
}
