package service

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// mockDB implements db.DBTX for testing.
// It returns a mockTx that embeds pgx.Tx to satisfy the interface.
type mockDB struct {
	// Embed Exec/Query/QueryRow from pgx.Tx? No, DBTX has them.
}

func (m *mockDB) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (m *mockDB) Query(ctx context.Context, sql string, arguments ...any) (pgx.Rows, error) {
	return &mockRows{}, nil
}
func (m *mockDB) QueryRow(ctx context.Context, sql string, arguments ...any) pgx.Row {
	return &mockRow{}
}
func (m *mockDB) Begin(ctx context.Context) (pgx.Tx, error) {
	return &mockTx{}, nil
}
func (m *mockDB) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (m *mockDB) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults {
	return &mockBatchResults{}
}
func (m *mockDB) LargeObjects() pgx.LargeObjects { return pgx.LargeObjects{} }
func (m *mockDB) Prepare(ctx context.Context, name, sql string) (*pgconn.StatementDescription, error) { return nil, nil }
func (m *mockDB) Ping(ctx context.Context) error { return nil }

type mockTx struct {
	pgx.Tx // Embed interface to satisfy methods we don't implement explicitely
	// Note: Explicit implementation overrides embedded.
}
func (m *mockTx) Commit(ctx context.Context) error { return nil }
func (m *mockTx) Rollback(ctx context.Context) error { return nil }
func (m *mockTx) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) { return pgconn.CommandTag{}, nil }
func (m *mockTx) Query(ctx context.Context, sql string, arguments ...any) (pgx.Rows, error) { return &mockRows{}, nil }
func (m *mockTx) QueryRow(ctx context.Context, sql string, arguments ...any) pgx.Row { return &mockRow{} }
// We assume we don't call other methods of Tx in our service logic (only Begin, Commit, Rollback, Query/Exec).
// If other methods are called, they will panic due to embedded nil interface.

type mockRow struct {}
func (m *mockRow) Scan(dest ...any) error { return nil }

type mockRows struct { pgx.Rows }
func (m *mockRows) Close() {}
func (m *mockRows) Err() error { return nil }
func (m *mockRows) Next() bool { return false }
func (m *mockRows) Scan(dest ...any) error { return nil }

type mockBatchResults struct { pgx.BatchResults }
func (m *mockBatchResults) Close() error { return nil }

