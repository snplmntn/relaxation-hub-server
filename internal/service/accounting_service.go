package service

import (
	"context"
	"math"
	"strings"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

// AccountingService manages the daily accounting sheet line items (expenses and
// therapist tips) that back the daily sales report's derived scalars.
type AccountingService struct {
	repo repository.AccountingRepository
}

func NewAccountingService(repo repository.AccountingRepository) *AccountingService {
	return &AccountingService{repo: repo}
}

// ListExpenses returns the expense lines for one branch-day plus their SIGNED
// total (positive = paid out, negative = cash came in).
func (s *AccountingService) ListExpenses(ctx context.Context, businessDate string, branchID int64) ([]model.AccountingExpense, float64, error) {
	filter, err := parseAccountingLineItemFilter(businessDate, branchID)
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.repo.ListExpenses(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	amounts := make([]float64, 0, len(rows))
	for i := range rows {
		rows[i].Amount = roundCurrency(rows[i].Amount)
		amounts = append(amounts, rows[i].Amount)
	}
	return rows, roundCurrency(SumSignedAmounts(amounts)), nil
}

// CreateExpense validates and records one expense line.
func (s *AccountingService) CreateExpense(ctx context.Context, req *model.CreateAccountingExpenseRequest, actorID int64) (*model.AccountingExpense, error) {
	if req == nil {
		return nil, NewValidationError("invalid_payload", "an expense payload is required", nil)
	}
	businessDate, err := ParseAccountingBusinessDate(req.BusinessDate)
	if err != nil {
		return nil, err
	}
	if req.BranchID <= 0 {
		return nil, NewValidationError("invalid_branch", "a valid branch is required", map[string]string{"branch_id": "must be greater than zero"})
	}
	label := strings.TrimSpace(req.Label)
	if label == "" {
		return nil, NewValidationError("invalid_label", "label is required", map[string]string{"label": "required"})
	}
	if len([]rune(label)) > model.AccountingExpenseLabelMaxLength {
		return nil, NewValidationError("invalid_label", "label is too long", map[string]string{"label": "must be at most 200 characters"})
	}
	category, err := ParseAccountingExpenseCategory(req.Category)
	if err != nil {
		return nil, err
	}
	amount, err := ValidateSignedExpenseAmount(req.Amount)
	if err != nil {
		return nil, err
	}

	expense := model.AccountingExpense{
		BusinessDate: businessDate,
		Date:         businessDate.Format(accountingDateLayout),
		BranchID:     req.BranchID,
		Label:        label,
		Category:     category,
		Amount:       amount,
		CreatedBy:    &actorID,
	}
	created, err := s.repo.CreateExpense(ctx, expense)
	if err != nil {
		return nil, err
	}
	created.Amount = roundCurrency(created.Amount)
	return created, nil
}

// DeleteExpense removes one expense line, returning model.ErrNotFound when the
// id does not exist.
func (s *AccountingService) DeleteExpense(ctx context.Context, expenseID int64) error {
	if expenseID <= 0 {
		return model.ErrNotFound
	}
	return s.repo.DeleteExpense(ctx, expenseID)
}

// ListTips returns the tip lines for one branch-day plus their total.
func (s *AccountingService) ListTips(ctx context.Context, businessDate string, branchID int64) ([]model.AccountingTip, float64, error) {
	filter, err := parseAccountingLineItemFilter(businessDate, branchID)
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.repo.ListTips(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	amounts := make([]float64, 0, len(rows))
	for i := range rows {
		rows[i].Amount = roundCurrency(rows[i].Amount)
		amounts = append(amounts, rows[i].Amount)
	}
	return rows, roundCurrency(SumSignedAmounts(amounts)), nil
}

// CreateTip validates and records one therapist tip line.
func (s *AccountingService) CreateTip(ctx context.Context, req *model.CreateAccountingTipRequest, actorID int64) (*model.AccountingTip, error) {
	if req == nil {
		return nil, NewValidationError("invalid_payload", "a tip payload is required", nil)
	}
	businessDate, err := ParseAccountingBusinessDate(req.BusinessDate)
	if err != nil {
		return nil, err
	}
	if req.BranchID <= 0 {
		return nil, NewValidationError("invalid_branch", "a valid branch is required", map[string]string{"branch_id": "must be greater than zero"})
	}
	if req.TherapistID <= 0 {
		return nil, NewValidationError("invalid_therapist", "a valid therapist is required", map[string]string{"therapist_id": "required"})
	}
	amount, err := ValidateTipAmount(req.Amount)
	if err != nil {
		return nil, err
	}
	exists, err := s.repo.TherapistExists(ctx, req.TherapistID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, NewValidationError("invalid_therapist", "therapist not found", map[string]string{"therapist_id": "does not exist"})
	}

	tip := model.AccountingTip{
		BusinessDate: businessDate,
		Date:         businessDate.Format(accountingDateLayout),
		BranchID:     req.BranchID,
		TherapistID:  req.TherapistID,
		Amount:       amount,
		Note:         strings.TrimSpace(req.Note),
		CreatedBy:    &actorID,
	}
	created, err := s.repo.CreateTip(ctx, tip)
	if err != nil {
		return nil, err
	}
	created.Amount = roundCurrency(created.Amount)
	return created, nil
}

// DeleteTip removes one tip line, returning model.ErrNotFound when the id does
// not exist.
func (s *AccountingService) DeleteTip(ctx context.Context, tipID int64) error {
	if tipID <= 0 {
		return model.ErrNotFound
	}
	return s.repo.DeleteTip(ctx, tipID)
}

const accountingDateLayout = "2006-01-02"

// ParseAccountingBusinessDate parses the YYYY-MM-DD business date shared by
// every accounting endpoint.
func ParseAccountingBusinessDate(value string) (time.Time, error) {
	parsed, err := time.Parse(accountingDateLayout, strings.TrimSpace(value))
	if err != nil {
		return time.Time{}, NewValidationError(
			"invalid_business_date",
			"business_date is required in YYYY-MM-DD format",
			map[string]string{"business_date": "must be YYYY-MM-DD"},
		)
	}
	return parsed, nil
}

// ParseAccountingExpenseCategory validates a category against the same list the
// accounting_expenses CHECK constraint enforces.
func ParseAccountingExpenseCategory(value string) (model.AccountingExpenseCategory, error) {
	candidate := model.AccountingExpenseCategory(strings.TrimSpace(value))
	for _, allowed := range model.AccountingExpenseCategories {
		if candidate == allowed {
			return allowed, nil
		}
	}
	return "", NewValidationError(
		"invalid_category",
		"category is not one of the allowed accounting categories",
		map[string]string{"category": "must be one of salary, reliever, transport, utilities, supplies, maintenance, vault_transfer, owner_funds, other"},
	)
}

// ValidateSignedExpenseAmount enforces a non-zero, finite expense amount. Signs
// are meaningful: positive is cash paid out, negative is cash that came in.
func ValidateSignedExpenseAmount(amount float64) (float64, error) {
	if math.IsNaN(amount) || math.IsInf(amount, 0) {
		return 0, NewValidationError("invalid_amount", "amount must be a finite number", map[string]string{"amount": "must be a finite number"})
	}
	rounded := roundCurrency(amount)
	if rounded == 0 {
		return 0, NewValidationError("invalid_amount", "amount must not be zero", map[string]string{"amount": "must not be zero"})
	}
	return rounded, nil
}

// ValidateTipAmount enforces a positive, finite tip amount.
func ValidateTipAmount(amount float64) (float64, error) {
	if math.IsNaN(amount) || math.IsInf(amount, 0) {
		return 0, NewValidationError("invalid_amount", "amount must be a finite number", map[string]string{"amount": "must be a finite number"})
	}
	rounded := roundCurrency(amount)
	if rounded <= 0 {
		return 0, NewValidationError("invalid_amount", "amount must be greater than zero", map[string]string{"amount": "must be greater than zero"})
	}
	return rounded, nil
}

func parseAccountingLineItemFilter(businessDate string, branchID int64) (model.AccountingLineItemFilter, error) {
	parsed, err := ParseAccountingBusinessDate(businessDate)
	if err != nil {
		return model.AccountingLineItemFilter{}, err
	}
	if branchID <= 0 {
		return model.AccountingLineItemFilter{}, NewValidationError(
			"invalid_branch",
			"a valid branch is required",
			map[string]string{"branch_id": "must be greater than zero"},
		)
	}
	return model.AccountingLineItemFilter{BusinessDate: parsed, BranchID: branchID}, nil
}

// SumSignedAmounts adds signed currency amounts, preserving the sign.
func SumSignedAmounts(amounts []float64) float64 {
	total := 0.0
	for _, amount := range amounts {
		total += amount
	}
	return roundCurrency(total)
}

// SplitSignedExpenseAmounts splits signed expense line-item amounts into the two
// buckets the must_be_zero formula consumes: positive amounts are cash paid out
// (others_deducted) and negative amounts are cash that came in from outside the
// day's sales, reported as their absolute value (others_added).
func SplitSignedExpenseAmounts(amounts []float64) (deducted float64, added float64) {
	for _, amount := range amounts {
		switch {
		case amount > 0:
			deducted += amount
		case amount < 0:
			added += -amount
		}
	}
	return roundCurrency(deducted), roundCurrency(added)
}

// DeriveAccountingLineItemTotals reduces one branch-day's raw line items into the
// daily-sales remittance scalars, keeping the counts so callers can distinguish
// "nothing recorded" from "recorded and summing to zero".
func DeriveAccountingLineItemTotals(items model.AccountingDayLineItems) model.AccountingLineItemTotals {
	deducted, added := SplitSignedExpenseAmounts(items.ExpenseAmounts)
	return model.AccountingLineItemTotals{
		ExpenseCount:   len(items.ExpenseAmounts),
		OthersDeducted: deducted,
		OthersAdded:    added,
		TipCount:       len(items.TipAmounts),
		TipsTotal:      SumSignedAmounts(items.TipAmounts),
	}
}

// ApplyAccountingLineItemTotals overwrites a remittance's tips_total,
// others_deducted and others_added with the values derived from the accounting
// sheet line items.
//
// This runs on every read of the daily sales report rather than on write. The
// remittance page and the accounting page both edit the same branch-day, and the
// scalars used to be whatever the last save happened to send, so one page could
// silently clobber the other's numbers and must_be_zero would drift. Deriving
// here keeps must_be_zero (whose formula is deliberately unchanged) consistent
// regardless of write order. The stored scalars are left alone when no line
// items exist, so branch-days captured before the accounting sheet existed keep
// reporting their hand-entered values.
func ApplyAccountingLineItemTotals(remittance *model.DailySalesRemittance, totals model.AccountingLineItemTotals) {
	if remittance == nil {
		return
	}
	if totals.TipCount > 0 {
		remittance.TipsTotal = totals.TipsTotal
	}
	if totals.ExpenseCount > 0 {
		remittance.OthersDeducted = totals.OthersDeducted
		remittance.OthersAdded = totals.OthersAdded
	}
}
