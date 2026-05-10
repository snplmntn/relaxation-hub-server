package db

import (
	"context"
	"time"
)

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
