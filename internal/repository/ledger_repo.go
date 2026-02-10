package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/db"
)

// LedgerEntryType represents the type of ledger entry (credit or debit)
type LedgerEntryType string

const (
	LedgerEntryTypeCredit LedgerEntryType = "credit"
	LedgerEntryTypeDebit  LedgerEntryType = "debit"
)

// LedgerCategory represents the category of a ledger entry
type LedgerCategory string

const (
	LedgerCategoryRevenue    LedgerCategory = "revenue"
	LedgerCategoryCommission LedgerCategory = "commission"
	LedgerCategoryPayout     LedgerCategory = "payout"
	LedgerCategoryExpense    LedgerCategory = "expense"
	LedgerCategoryRefund     LedgerCategory = "refund"
	LedgerCategoryAdjustment LedgerCategory = "adjustment"
	LedgerCategorySettlement LedgerCategory = "settlement"
)

// LedgerEntryStatus represents the approval status of a ledger entry
type LedgerEntryStatus string

const (
	LedgerStatusPending  LedgerEntryStatus = "pending"
	LedgerStatusApproved LedgerEntryStatus = "approved"
	LedgerStatusRejected LedgerEntryStatus = "rejected"
)

// LedgerEntry represents a single entry in the financial ledger
type LedgerEntry struct {
	EntryID     int64             `json:"entry_id"`
	BookingID   *int64            `json:"booking_id,omitempty"`
	EntryType   LedgerEntryType   `json:"entry_type"`
	Category    LedgerCategory    `json:"category"`
	Amount      float64           `json:"amount"`
	Description string            `json:"description,omitempty"`
	EntryDate   time.Time         `json:"entry_date"`
	CreatedAt   time.Time         `json:"created_at"`
	CreatedBy   *int64            `json:"created_by,omitempty"`
	ProofURL    *string           `json:"proof_url,omitempty"`
	Status      LedgerEntryStatus `json:"status"`
	ReviewedBy  *int64            `json:"reviewed_by,omitempty"`
	ReviewedAt  *time.Time        `json:"reviewed_at,omitempty"`
	TargetUserID *int64           `json:"target_user_id,omitempty"`
	Voided       bool             `json:"voided"`
	VoidedAt     *time.Time       `json:"voided_at,omitempty"`
	VoidedReason *string          `json:"voided_reason,omitempty"`
}

// LedgerSummary holds aggregated ledger data for reporting
type LedgerSummary struct {
	TotalCredits float64 `json:"total_credits"`
	TotalDebits  float64 `json:"total_debits"`
	NetProfit    float64 `json:"net_profit"`
	EntryCount   int     `json:"entry_count"`
}

// LedgerPeriodSummary holds ledger data grouped by time period
type LedgerPeriodSummary struct {
	PeriodStart  time.Time `json:"period_start"`
	TotalCredits float64   `json:"total_credits"`
	TotalDebits  float64   `json:"total_debits"`
	NetProfit    float64   `json:"net_profit"`
	EntryCount   int       `json:"entry_count"`
}

// LedgerRepository defines data access methods for ledger entries
type LedgerRepository interface {
	// Insert adds a new ledger entry
	Insert(ctx context.Context, entry *LedgerEntry) error
	// InsertBookingEntries creates all standard ledger entries for a completed booking
	InsertBookingEntries(ctx context.Context, bookingID int64, therapistID *int64, revenue, payout, commission float64, entryDate time.Time) error
	// InsertExpense adds a manual expense entry with optional proof URL
	InsertExpense(ctx context.Context, amount float64, description string, category LedgerCategory, entryDate time.Time, createdBy int64, proofURL *string) error
	// GetSummary returns aggregated ledger data for a date range
	GetSummary(ctx context.Context, startDate, endDate time.Time) (*LedgerSummary, error)
	// GetSummaryByPeriod returns ledger data grouped by time period (day, week, month, quarter, year)
	GetSummaryByPeriod(ctx context.Context, startDate, endDate time.Time, granularity string) ([]LedgerPeriodSummary, error)
	// ListByBookingID returns all ledger entries for a specific booking
	ListByBookingID(ctx context.Context, bookingID int64) ([]LedgerEntry, error)
	// ListExpenses returns expense entries within a date range
	ListExpenses(ctx context.Context, startDate, endDate time.Time) ([]LedgerEntry, error)
	// DeleteExpense removes an expense entry by ID (only expenses can be deleted)
	DeleteExpense(ctx context.Context, entryID int64) error
	// VoidEntry marks an entry as voided instead of deleting it
	VoidEntry(ctx context.Context, entryID int64, reason string) error
	// GetTherapistBalance calculates the current balance owed to a therapist
	GetTherapistBalance(ctx context.Context, therapistID int64) (float64, error)
	// RecordSettlement adds a settlement entry (payment to therapist)
	RecordSettlement(ctx context.Context, therapistID int64, amount float64, reference string, recordedBy int64) error
	// GetTherapistBalances returns the current balance for all therapists
	GetTherapistBalances(ctx context.Context) ([]TherapistBalance, error)
	// ListEntries returns all ledger entries within a date range
	ListEntries(ctx context.Context, startDate, endDate time.Time) ([]LedgerEntry, error)
}

type ledgerRepoImpl struct {
	db db.DBTX
}

// NewLedgerRepository creates a new LedgerRepository
func NewLedgerRepository(database db.DBTX) LedgerRepository {
	return &ledgerRepoImpl{db: database}
}

func (r *ledgerRepoImpl) Insert(ctx context.Context, entry *LedgerEntry) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	query := `
		INSERT INTO ledger_entries (booking_id, entry_type, category, amount, description, entry_date, created_by, status, proof_url)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING entry_id, created_at
	`
	// Set default status if not provided (though DB handles it, better to claim control)
	if entry.Status == "" {
		entry.Status = LedgerStatusPending
	}

	return r.db.QueryRow(ctx, query,
		entry.BookingID,
		entry.EntryType,
		entry.Category,
		entry.Amount,
		entry.Description,
		entry.EntryDate,
		entry.CreatedBy,
		entry.Status,
		entry.ProofURL,
	).Scan(&entry.EntryID, &entry.CreatedAt)
}

func (r *ledgerRepoImpl) InsertBookingEntries(ctx context.Context, bookingID int64, therapistID *int64, revenue, payout, commission float64, entryDate time.Time) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	// Insert revenue entry (credit)
	if revenue > 0 {
		_, err := r.db.Exec(ctx, `
			INSERT INTO ledger_entries (booking_id, entry_type, category, amount, description, entry_date, status)
			VALUES ($1, 'credit', 'revenue', $2, 'Client payment', $3, 'approved')
		`, bookingID, revenue, entryDate)
		if err != nil {
			return fmt.Errorf("failed to insert revenue entry: %w", err)
		}
	}

	// Insert payout entry (debit)
	if payout > 0 {
		_, err := r.db.Exec(ctx, `
			INSERT INTO ledger_entries (booking_id, entry_type, category, amount, description, entry_date, status, target_user_id)
			VALUES ($1, 'debit', 'payout', $2, 'Therapist payout', $3, 'approved', $4)
		`, bookingID, payout, entryDate, therapistID)
		if err != nil {
			return fmt.Errorf("failed to insert payout entry: %w", err)
		}
	}
	// Insert commission entry (credit)
	if commission > 0 {
		_, err := r.db.Exec(ctx, `
			INSERT INTO ledger_entries (booking_id, entry_type, category, amount, description, entry_date, status)
			VALUES ($1, 'credit', 'commission', $2, 'Platform commission', $3, 'approved')
		`, bookingID, commission, entryDate)
		if err != nil {
			return fmt.Errorf("failed to insert commission entry: %w", err)
		}
	}

	return nil
}

func (r *ledgerRepoImpl) InsertExpense(ctx context.Context, amount float64, description string, category LedgerCategory, entryDate time.Time, createdBy int64, proofURL *string) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	_, err := r.db.Exec(ctx, `
		INSERT INTO ledger_entries (entry_type, category, amount, description, entry_date, created_by, proof_url, status)
		VALUES ('debit', $1, $2, $3, $4, $5, $6, 'approved')
	`, category, amount, description, entryDate, createdBy, proofURL)
	return err
}

func (r *ledgerRepoImpl) GetSummary(ctx context.Context, startDate, endDate time.Time) (*LedgerSummary, error) {
	ctx, cancel := db.WithLongQueryTimeout(ctx)
	defer cancel()

	var credits, debits float64
	var count int

	err := r.db.QueryRow(ctx, `
		SELECT
			COALESCE(SUM(CASE WHEN entry_type = 'credit' THEN amount ELSE 0.0 END), 0.0) as total_credits,
			COALESCE(SUM(CASE WHEN entry_type = 'debit' THEN amount ELSE 0.0 END), 0.0) as total_debits,
			COUNT(*) as entry_count
		FROM ledger_entries
		WHERE entry_date >= $1 AND entry_date <= $2 AND category != 'commission' AND category != 'settlement' AND voided = FALSE
	`, startDate, endDate).Scan(&credits, &debits, &count)
	if err != nil {
		return nil, fmt.Errorf("failed to get ledger summary: %w", err)
	}

	return &LedgerSummary{
		TotalCredits: credits,
		TotalDebits:  debits,
		NetProfit:    credits - debits,
		EntryCount:   count,
	}, nil
}

func (r *ledgerRepoImpl) GetSummaryByPeriod(ctx context.Context, startDate, endDate time.Time, granularity string) ([]LedgerPeriodSummary, error) {
	ctx, cancel := db.WithLongQueryTimeout(ctx)
	defer cancel()

	// Validate granularity and map to PostgreSQL DATE_TRUNC interval
	var truncInterval string
	switch granularity {
	case "day":
		truncInterval = "day"
	case "week":
		truncInterval = "week"
	case "month":
		truncInterval = "month"
	case "quarter":
		truncInterval = "quarter"
	case "year":
		truncInterval = "year"
	default:
		truncInterval = "day"
	}

	query := fmt.Sprintf(`
		SELECT
			DATE_TRUNC('%s', entry_date) as period_start,
			COALESCE(SUM(CASE WHEN entry_type = 'credit' THEN amount ELSE 0.0 END), 0.0) as total_credits,
			COALESCE(SUM(CASE WHEN entry_type = 'debit' THEN amount ELSE 0.0 END), 0.0) as total_debits,
			COUNT(*) as entry_count
		FROM ledger_entries
		WHERE entry_date >= $1 AND entry_date <= $2 AND category != 'commission' AND category != 'settlement' AND voided = FALSE
		GROUP BY DATE_TRUNC('%s', entry_date)
		ORDER BY period_start ASC
	`, truncInterval, truncInterval)

	rows, err := r.db.Query(ctx, query, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get ledger summary by period: %w", err)
	}
	defer rows.Close()

	var summaries []LedgerPeriodSummary
	for rows.Next() {
		var s LedgerPeriodSummary
		if err := rows.Scan(&s.PeriodStart, &s.TotalCredits, &s.TotalDebits, &s.EntryCount); err != nil {
			return nil, err
		}
		s.NetProfit = s.TotalCredits - s.TotalDebits
		summaries = append(summaries, s)
	}

	return summaries, rows.Err()
}

func (r *ledgerRepoImpl) ListByBookingID(ctx context.Context, bookingID int64) ([]LedgerEntry, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	rows, err := r.db.Query(ctx, `
		SELECT entry_id, booking_id, entry_type, category, amount, description, entry_date, created_at, created_by, proof_url, status, voided, voided_at, voided_reason
		FROM ledger_entries
		WHERE booking_id = $1 AND voided = FALSE
		ORDER BY created_at ASC
	`, bookingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []LedgerEntry
	for rows.Next() {
		var e LedgerEntry
		if err := rows.Scan(
			&e.EntryID, &e.BookingID, &e.EntryType, &e.Category, &e.Amount, &e.Description, &e.EntryDate, &e.CreatedAt, &e.CreatedBy,
			&e.ProofURL, &e.Status, &e.Voided, &e.VoidedAt, &e.VoidedReason,
		); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (r *ledgerRepoImpl) ListExpenses(ctx context.Context, startDate, endDate time.Time) ([]LedgerEntry, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	rows, err := r.db.Query(ctx, `
		SELECT entry_id, booking_id, entry_type, category, amount, description, entry_date, created_at, created_by, proof_url, status, voided, voided_at, voided_reason
		FROM ledger_entries
		WHERE category = 'expense' AND entry_date >= $1 AND entry_date <= $2 AND voided = FALSE
		ORDER BY entry_date DESC
	`, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []LedgerEntry
	for rows.Next() {
		var e LedgerEntry
		if err := rows.Scan(
			&e.EntryID, &e.BookingID, &e.EntryType, &e.Category, &e.Amount, &e.Description, &e.EntryDate, &e.CreatedAt, &e.CreatedBy,
			&e.ProofURL, &e.Status, &e.Voided, &e.VoidedAt, &e.VoidedReason,
		); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (r *ledgerRepoImpl) DeleteExpense(ctx context.Context, entryID int64) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	// Only allow deleting expense entries
	cmd, err := r.db.Exec(ctx, `
		DELETE FROM ledger_entries
		WHERE entry_id = $1 AND category = 'expense'
	`, entryID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("expense entry not found or not deletable")
	}
	return nil
}

func (r *ledgerRepoImpl) VoidEntry(ctx context.Context, entryID int64, reason string) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	cmd, err := r.db.Exec(ctx, `
		UPDATE ledger_entries
		SET voided = TRUE, voided_at = NOW(), voided_reason = $2
		WHERE entry_id = $1
	`, entryID, reason)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("entry not found")
	}
	return nil
}
