package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// --- Mocks ---

type MockDBTX struct {
	mock.Mock
}

func (m *MockDBTX) Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error) {
	args := m.Called(ctx, sql, arguments)
	return args.Get(0).(pgconn.CommandTag), args.Error(1)
}

func (m *MockDBTX) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	callArgs := m.Called(ctx, sql, args)
	return callArgs.Get(0).(pgx.Rows), callArgs.Error(1)
}

func (m *MockDBTX) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	callArgs := m.Called(ctx, sql, args)
	return callArgs.Get(0).(pgx.Row)
}

func (m *MockDBTX) Begin(ctx context.Context) (pgx.Tx, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(pgx.Tx), args.Error(1)
}

func (m *MockDBTX) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	args := m.Called(ctx, tableName, columnNames, rowSrc)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockDBTX) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults {
	args := m.Called(ctx, b)
	return args.Get(0).(pgx.BatchResults)
}

type MockTx struct {
	mock.Mock
}

func (m *MockTx) Begin(ctx context.Context) (pgx.Tx, error) {
	args := m.Called(ctx)
	return args.Get(0).(pgx.Tx), args.Error(1)
}

func (m *MockTx) Commit(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockTx) Rollback(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockTx) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	args := m.Called(ctx, tableName, columnNames, rowSrc)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockTx) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults {
	args := m.Called(ctx, b)
	return args.Get(0).(pgx.BatchResults)
}

func (m *MockTx) Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error) {
	args := m.Called(ctx, sql, arguments)
	return args.Get(0).(pgconn.CommandTag), args.Error(1)
}

func (m *MockTx) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	callArgs := m.Called(ctx, sql, args)
	return callArgs.Get(0).(pgx.Rows), callArgs.Error(1)
}

func (m *MockTx) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	callArgs := m.Called(ctx, sql, args)
	return callArgs.Get(0).(pgx.Row)
}

func (m *MockTx) Conn() *pgx.Conn                { return nil }
func (m *MockTx) LargeObjects() pgx.LargeObjects { return pgx.LargeObjects{} }
func (m *MockTx) Prepare(ctx context.Context, name, sql string) (*pgconn.StatementDescription, error) {
	return nil, nil
}

type MockRow struct {
	mock.Mock
}

func (m *MockRow) Scan(dest ...interface{}) error {
	args := m.Called(dest...)
	return args.Error(0)
}

type MockRows struct {
	mock.Mock
}

func (m *MockRows) Close()                        { m.Called() }
func (m *MockRows) Err() error                    { return m.Called().Error(0) }
func (m *MockRows) CommandTag() pgconn.CommandTag { return m.Called().Get(0).(pgconn.CommandTag) }
func (m *MockRows) FieldDescriptions() []pgconn.FieldDescription {
	return m.Called().Get(0).([]pgconn.FieldDescription)
}
func (m *MockRows) Next() bool { return m.Called().Bool(0) }
func (m *MockRows) Scan(dest ...interface{}) error {
	args := m.Called(dest...)
	return args.Error(0)
}
func (m *MockRows) Values() ([]interface{}, error) {
	args := m.Called()
	return args.Get(0).([]interface{}), args.Error(1)
}
func (m *MockRows) RawValues() [][]byte { return m.Called().Get(0).([][]byte) }
func (m *MockRows) Conn() *pgx.Conn     { return nil }

func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// --- Tests ---

func TestLedgerRepo_Insert(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewLedgerRepository(mockDB)

	t.Run("success", func(t *testing.T) {
		ctx := context.Background()
		bookingID := int64(100)
		entry := &LedgerEntry{
			BookingID:   &bookingID,
			EntryType:   LedgerEntryTypeCredit,
			Category:    LedgerCategoryRevenue,
			Amount:      1500.0,
			Description: "Test revenue",
			EntryDate:   time.Now(),
		}

		row := new(MockRow)
		mockDB.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
			return contains(sql, "INSERT INTO ledger_entries")
		}), []interface{}{
			entry.BookingID,
			entry.EntryType,
			entry.Category,
			entry.Amount,
			entry.Description,
			entry.EntryDate,
			entry.CreatedBy,
			LedgerStatusPending,
			entry.ProofURL,
		}).Return(row).Once()

		now := time.Now()
		row.On("Scan", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
			*args.Get(0).(*int64) = 500
			*args.Get(1).(*time.Time) = now
		}).Return(nil).Once()

		err := repo.Insert(ctx, entry)

		assert.NoError(t, err)
		assert.Equal(t, int64(500), entry.EntryID)
		assert.Equal(t, now, entry.CreatedAt)
		mockDB.AssertExpectations(t)
	})
}

func TestLedgerRepo_InsertBookingEntries(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewLedgerRepository(mockDB)

	t.Run("success with revenue, payout, and commission", func(t *testing.T) {
		ctx := context.Background()
		bookingID := int64(200)
		therapistID := int64(10)
		revenue := 2000.0
		payout := 1500.0
		commission := 500.0
		entryDate := time.Now()

		mockDB.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
			return contains(sql, "revenue")
		}), []interface{}{bookingID, revenue, entryDate}).Return(pgconn.NewCommandTag("INSERT 1"), nil).Once()

		mockDB.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
			return contains(sql, "payout")
		}), []interface{}{bookingID, payout, entryDate, &therapistID}).Return(pgconn.NewCommandTag("INSERT 1"), nil).Once()

		mockDB.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
			return contains(sql, "commission")
		}), []interface{}{bookingID, commission, entryDate}).Return(pgconn.NewCommandTag("INSERT 1"), nil).Once()

		err := repo.InsertBookingEntries(ctx, bookingID, &therapistID, revenue, payout, commission, entryDate)

		assert.NoError(t, err)
		mockDB.AssertExpectations(t)
	})
}

func TestLedgerRepo_GetSummary(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewLedgerRepository(mockDB)

	t.Run("success", func(t *testing.T) {
		ctx := context.Background()
		start := time.Now().Add(-24 * time.Hour)
		end := time.Now()

		row := new(MockRow)
		mockDB.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
			return contains(sql, "SELECT") && contains(sql, "SUM") && contains(sql, "FROM ledger_entries")
		}), []interface{}{start, end}).Return(row).Once()

		row.On("Scan", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
			*args.Get(0).(*float64) = 10000.0
			*args.Get(1).(*float64) = 7000.0
			*args.Get(2).(*int) = 50
		}).Return(nil).Once()

		summary, err := repo.GetSummary(ctx, start, end)

		assert.NoError(t, err)
		assert.Equal(t, 10000.0, summary.TotalCredits)
		assert.Equal(t, 7000.0, summary.TotalDebits)
		assert.Equal(t, 3000.0, summary.NetProfit)
		assert.Equal(t, 50, summary.EntryCount)
	})
}

func TestLedgerRepo_GetPayoutBalance(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewLedgerRepository(mockDB)

	t.Run("success", func(t *testing.T) {
		ctx := context.Background()
		therapistID := int64(10)

		row := new(MockRow)
		mockDB.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
			return contains(sql, "SELECT COALESCE") && contains(sql, "FROM ledger_entries") && contains(sql, "target_user_id = $1")
		}), []interface{}{therapistID, string(TargetRoleTherapist)}).Return(row).Once()

		row.On("Scan", mock.Anything).Run(func(args mock.Arguments) {
			*args.Get(0).(*float64) = 5000.0
		}).Return(nil).Once()

		balance, err := repo.GetPayoutBalance(ctx, therapistID, TargetRoleTherapist)

		assert.NoError(t, err)
		assert.Equal(t, 5000.0, balance)
		mockDB.AssertExpectations(t)
	})
}

func TestLedgerRepo_RecordSettlement(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewLedgerRepository(mockDB)

	t.Run("success", func(t *testing.T) {
		ctx := context.Background()
		therapistID := int64(10)
		amount := 1500.0
		ref := "SETTLE-001"
		adminID := int64(7)

		mockDB.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
			return contains(sql, "INSERT INTO ledger_entries") && contains(sql, "settlement")
		}), []interface{}{amount, ref, adminID, therapistID, string(TargetRoleTherapist)}).Return(pgconn.NewCommandTag("INSERT 1"), nil).Once()

		err := repo.RecordSettlement(ctx, therapistID, TargetRoleTherapist, amount, ref, adminID)

		assert.NoError(t, err)
		mockDB.AssertExpectations(t)
	})
}

func TestLedgerRepo_RecordPayrollSettlement(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewLedgerRepository(mockDB).(interface {
		RecordPayrollSettlement(ctx context.Context, payrollRunID, payrollRowID, userID int64, role TargetRole, amount float64, method, reference string, recordedBy int64) (int64, error)
	})
	ctx := context.Background()
	row := new(MockRow)

	mockDB.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return contains(sql, "INSERT INTO ledger_entries") &&
			contains(sql, "payroll_run_id") &&
			contains(sql, "payroll_row_id") &&
			contains(sql, "target_role") &&
			contains(sql, "RETURNING entry_id")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) == 8 &&
			args[0] == int64(71) &&
			args[1] == int64(81) &&
			args[2] == int64(22) &&
			args[3] == string(TargetRoleAdmin) &&
			args[4] == 123.45 &&
			args[5] == "cash" &&
			args[6] == "CASH-1" &&
			args[7] == int64(7)
	})).Return(row).Once()
	row.On("Scan", mock.AnythingOfType("*int64")).Run(func(args mock.Arguments) {
		*args.Get(0).(*int64) = 909
	}).Return(nil).Once()

	entryID, err := repo.RecordPayrollSettlement(ctx, 71, 81, 22, TargetRoleAdmin, 123.45, "cash", "CASH-1", 7)

	assert.NoError(t, err)
	assert.Equal(t, int64(909), entryID)
	mockDB.AssertExpectations(t)
	row.AssertExpectations(t)
}

func TestLedgerRepo_GetPayoutBalances(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewLedgerRepository(mockDB)

	t.Run("success", func(t *testing.T) {
		ctx := context.Background()

		rows := new(MockRows)
		mockDB.On("Query", mock.Anything, mock.MatchedBy(func(sql string) bool {
			return contains(sql, "SELECT") && contains(sql, "users u") && contains(sql, "UNION ALL")
		}), []interface{}(nil)).Return(rows, nil).Once()

		rows.On("Next").Return(true).Once()
		rows.On("Scan", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
			*args.Get(0).(*int64) = 1
			*args.Get(1).(*string) = "therapist"
			*args.Get(2).(*string) = "John Doe"
			*args.Get(3).(*float64) = 5000.0
			*args.Get(4).(*float64) = 3000.0
		}).Return(nil).Once()
		rows.On("Next").Return(false).Once()
		rows.On("Close").Return(nil).Once()
		rows.On("Err").Return(nil).Once()

		balances, err := repo.GetPayoutBalances(ctx)

		assert.NoError(t, err)
		assert.Len(t, balances, 1)
		assert.Equal(t, 2000.0, balances[0].BalanceOwed)
		assert.Equal(t, TargetRoleTherapist, balances[0].Role)
		mockDB.AssertExpectations(t)
	})
}

func TestLedgerRepo_ListByBookingID(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewLedgerRepository(mockDB)

	t.Run("success", func(t *testing.T) {
		ctx := context.Background()
		bookingID := int64(200)

		rows := new(MockRows)
		mockDB.On("Query", mock.Anything, mock.MatchedBy(func(sql string) bool {
			return contains(sql, "FROM ledger_entries") && contains(sql, "booking_id = $1")
		}), []interface{}{bookingID}).Return(rows, nil).Once()

		rows.On("Next").Return(true).Once()
		rows.On("Scan", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil).Once()
		rows.On("Next").Return(false).Once()
		rows.On("Close").Return(nil).Once()
		rows.On("Err").Return(nil).Once()

		entries, err := repo.ListByBookingID(ctx, bookingID)

		assert.NoError(t, err)
		assert.Len(t, entries, 1)
		mockDB.AssertExpectations(t)
	})
}

func TestLedgerRepo_ListExpenses(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewLedgerRepository(mockDB)

	t.Run("success", func(t *testing.T) {
		ctx := context.Background()
		start := time.Now().Add(-24 * time.Hour)
		end := time.Now()

		rows := new(MockRows)
		mockDB.On("Query", mock.Anything, mock.MatchedBy(func(sql string) bool {
			return contains(sql, "category = 'expense'")
		}), []interface{}{start, end}).Return(rows, nil).Once()

		rows.On("Next").Return(true).Once()
		rows.On("Scan", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil).Once()
		rows.On("Next").Return(false).Once()
		rows.On("Close").Return(nil).Once()
		rows.On("Err").Return(nil).Once()

		entries, err := repo.ListExpenses(ctx, start, end)

		assert.NoError(t, err)
		assert.Len(t, entries, 1)
		mockDB.AssertExpectations(t)
	})
}

func TestLedgerRepo_DeleteExpense(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewLedgerRepository(mockDB)

	t.Run("success", func(t *testing.T) {
		ctx := context.Background()
		entryID := int64(500)

		mockDB.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
			return contains(sql, "DELETE FROM ledger_entries") && contains(sql, "category = 'expense'")
		}), []interface{}{entryID}).Return(pgconn.NewCommandTag("DELETE 1"), nil).Once()

		err := repo.DeleteExpense(ctx, entryID)

		assert.NoError(t, err)
		mockDB.AssertExpectations(t)
	})

	t.Run("not found", func(t *testing.T) {
		ctx := context.Background()
		entryID := int64(999)

		mockDB.On("Exec", mock.Anything, mock.Anything, []interface{}{entryID}).Return(pgconn.NewCommandTag("DELETE 0"), nil).Once()

		err := repo.DeleteExpense(ctx, entryID)

		assert.Error(t, err)
		mockDB.AssertExpectations(t)
	})
}

func TestLedgerRepo_VoidEntry(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewLedgerRepository(mockDB)

	t.Run("success", func(t *testing.T) {
		ctx := context.Background()
		entryID := int64(500)
		reason := "Voiding test"

		mockDB.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
			return contains(sql, "UPDATE ledger_entries") && contains(sql, "SET voided = TRUE")
		}), []interface{}{entryID, reason}).Return(pgconn.NewCommandTag("UPDATE 1"), nil).Once()

		err := repo.VoidEntry(ctx, entryID, reason)

		assert.NoError(t, err)
		mockDB.AssertExpectations(t)
	})
}

func TestLedgerRepo_ListEntries(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewLedgerRepository(mockDB)

	t.Run("success", func(t *testing.T) {
		ctx := context.Background()
		start := time.Now().Add(-24 * time.Hour)
		end := time.Now()

		rows := new(MockRows)
		mockDB.On("Query", mock.Anything, mock.MatchedBy(func(sql string) bool {
			return contains(sql, "SELECT") && contains(sql, "FROM ledger_entries") && contains(sql, "entry_date >= $1")
		}), []interface{}{start, end}).Return(rows, nil).Once()

		rows.On("Next").Return(true).Once()
		rows.On("Scan", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil).Once()
		rows.On("Next").Return(false).Once()
		rows.On("Close").Return(nil).Once()

		entries, err := repo.ListEntries(ctx, start, end)

		assert.NoError(t, err)
		assert.Len(t, entries, 1)
		mockDB.AssertExpectations(t)
	})
}
