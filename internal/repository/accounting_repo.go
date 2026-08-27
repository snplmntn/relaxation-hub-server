package repository

import (
	"context"
	"fmt"

	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

// AccountingRepository persists the line items behind the daily accounting
// sheet: per-branch-day expenses (signed amounts) and per-therapist tips.
type AccountingRepository interface {
	ListExpenses(ctx context.Context, filter model.AccountingLineItemFilter) ([]model.AccountingExpense, error)
	CreateExpense(ctx context.Context, expense model.AccountingExpense) (*model.AccountingExpense, error)
	DeleteExpense(ctx context.Context, expenseID int64) error
	ListTips(ctx context.Context, filter model.AccountingLineItemFilter) ([]model.AccountingTip, error)
	CreateTip(ctx context.Context, tip model.AccountingTip) (*model.AccountingTip, error)
	DeleteTip(ctx context.Context, tipID int64) error
	// TherapistExists reports whether a therapist profile exists for the id so
	// the service can reject bad input before it hits the foreign key.
	TherapistExists(ctx context.Context, therapistID int64) (bool, error)
}

type accountingRepo struct {
	db db.DBTX
}

// NewAccountingRepository creates a new AccountingRepository.
func NewAccountingRepository(database db.DBTX) AccountingRepository {
	return &accountingRepo{db: database}
}

// expenseSelectColumns expects the expense rows aliased as "e" and the creating
// user LEFT JOINed as "u".
const expenseSelectColumns = `e.expense_id, e.business_date, e.branch_id, e.label, e.category, e.amount,
	e.created_at, e.created_by, COALESCE(u.full_name, '')`

// tipSelectColumns expects the tip rows aliased as "t" and the therapist's user
// row LEFT JOINed as "therapist".
const tipSelectColumns = `t.tip_id, t.business_date, t.branch_id, t.therapist_id,
	COALESCE(therapist.full_name, ''), t.amount, COALESCE(t.note, ''), t.created_at, t.created_by`

func expenseScanTargets(e *model.AccountingExpense) []any {
	return []any{
		&e.ExpenseID, &e.BusinessDate, &e.BranchID, &e.Label, &e.Category,
		&e.Amount, &e.CreatedAt, &e.CreatedBy, &e.CreatedByName,
	}
}

func tipScanTargets(t *model.AccountingTip) []any {
	return []any{
		&t.TipID, &t.BusinessDate, &t.BranchID, &t.TherapistID, &t.TherapistName,
		&t.Amount, &t.Note, &t.CreatedAt, &t.CreatedBy,
	}
}

func (r *accountingRepo) ListExpenses(ctx context.Context, filter model.AccountingLineItemFilter) ([]model.AccountingExpense, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	rows, err := r.db.Query(ctx, `
		SELECT `+expenseSelectColumns+`
		FROM accounting_expenses e
		LEFT JOIN users u ON u.user_id = e.created_by
		WHERE e.business_date = $1 AND e.branch_id = $2
		ORDER BY e.created_at, e.expense_id`,
		filter.BusinessDate.Format("2006-01-02"), filter.BranchID)
	if err != nil {
		return nil, fmt.Errorf("query accounting expenses: %w", err)
	}
	defer rows.Close()

	items := make([]model.AccountingExpense, 0)
	for rows.Next() {
		var item model.AccountingExpense
		if err := rows.Scan(expenseScanTargets(&item)...); err != nil {
			return nil, fmt.Errorf("scan accounting expense: %w", err)
		}
		item.Date = item.BusinessDate.Format("2006-01-02")
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate accounting expenses: %w", err)
	}
	return items, nil
}

func (r *accountingRepo) CreateExpense(ctx context.Context, expense model.AccountingExpense) (*model.AccountingExpense, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	// The insert is wrapped in a CTE so created_by_name resolves from users in
	// the same round trip as the RETURNING clause.
	out := model.AccountingExpense{}
	err := r.db.QueryRow(ctx, `
		WITH inserted AS (
			INSERT INTO accounting_expenses (business_date, branch_id, label, category, amount, created_by)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING expense_id, business_date, branch_id, label, category, amount, created_at, created_by
		)
		SELECT `+expenseSelectColumns+`
		FROM inserted e
		LEFT JOIN users u ON u.user_id = e.created_by`,
		expense.BusinessDate.Format("2006-01-02"), expense.BranchID, expense.Label,
		string(expense.Category), expense.Amount, expense.CreatedBy,
	).Scan(expenseScanTargets(&out)...)
	if err != nil {
		return nil, fmt.Errorf("create accounting expense: %w", err)
	}
	out.Date = out.BusinessDate.Format("2006-01-02")
	return &out, nil
}

func (r *accountingRepo) DeleteExpense(ctx context.Context, expenseID int64) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	tag, err := r.db.Exec(ctx, `DELETE FROM accounting_expenses WHERE expense_id = $1`, expenseID)
	if err != nil {
		return fmt.Errorf("delete accounting expense: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrNotFound
	}
	return nil
}

func (r *accountingRepo) ListTips(ctx context.Context, filter model.AccountingLineItemFilter) ([]model.AccountingTip, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	rows, err := r.db.Query(ctx, `
		SELECT `+tipSelectColumns+`
		FROM accounting_tips t
		LEFT JOIN users therapist ON therapist.user_id = t.therapist_id
		WHERE t.business_date = $1 AND t.branch_id = $2
		ORDER BY t.created_at, t.tip_id`,
		filter.BusinessDate.Format("2006-01-02"), filter.BranchID)
	if err != nil {
		return nil, fmt.Errorf("query accounting tips: %w", err)
	}
	defer rows.Close()

	items := make([]model.AccountingTip, 0)
	for rows.Next() {
		var item model.AccountingTip
		if err := rows.Scan(tipScanTargets(&item)...); err != nil {
			return nil, fmt.Errorf("scan accounting tip: %w", err)
		}
		item.Date = item.BusinessDate.Format("2006-01-02")
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate accounting tips: %w", err)
	}
	return items, nil
}

func (r *accountingRepo) CreateTip(ctx context.Context, tip model.AccountingTip) (*model.AccountingTip, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	out := model.AccountingTip{}
	err := r.db.QueryRow(ctx, `
		WITH inserted AS (
			INSERT INTO accounting_tips (business_date, branch_id, therapist_id, amount, note, created_by)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING tip_id, business_date, branch_id, therapist_id, amount, note, created_at, created_by
		)
		SELECT `+tipSelectColumns+`
		FROM inserted t
		LEFT JOIN users therapist ON therapist.user_id = t.therapist_id`,
		tip.BusinessDate.Format("2006-01-02"), tip.BranchID, tip.TherapistID,
		tip.Amount, tip.Note, tip.CreatedBy,
	).Scan(tipScanTargets(&out)...)
	if err != nil {
		return nil, fmt.Errorf("create accounting tip: %w", err)
	}
	out.Date = out.BusinessDate.Format("2006-01-02")
	return &out, nil
}

func (r *accountingRepo) DeleteTip(ctx context.Context, tipID int64) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	tag, err := r.db.Exec(ctx, `DELETE FROM accounting_tips WHERE tip_id = $1`, tipID)
	if err != nil {
		return fmt.Errorf("delete accounting tip: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrNotFound
	}
	return nil
}

func (r *accountingRepo) TherapistExists(ctx context.Context, therapistID int64) (bool, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	var exists bool
	if err := r.db.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM therapist_profiles WHERE therapist_id = $1)`,
		therapistID,
	).Scan(&exists); err != nil {
		return false, fmt.Errorf("check therapist exists: %w", err)
	}
	return exists, nil
}
