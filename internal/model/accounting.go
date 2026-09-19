package model

import "time"

// AccountingExpenseCategory classifies a daily accounting sheet expense line.
type AccountingExpenseCategory string

const (
	AccountingExpenseCategorySalary        AccountingExpenseCategory = "salary"
	AccountingExpenseCategoryReliever      AccountingExpenseCategory = "reliever"
	AccountingExpenseCategoryTransport     AccountingExpenseCategory = "transport"
	AccountingExpenseCategoryUtilities     AccountingExpenseCategory = "utilities"
	AccountingExpenseCategorySupplies      AccountingExpenseCategory = "supplies"
	AccountingExpenseCategoryMaintenance   AccountingExpenseCategory = "maintenance"
	AccountingExpenseCategoryVaultTransfer AccountingExpenseCategory = "vault_transfer"
	AccountingExpenseCategoryOwnerFunds    AccountingExpenseCategory = "owner_funds"
	AccountingExpenseCategoryOther         AccountingExpenseCategory = "other"
)

// AccountingExpenseCategories lists every accepted expense category, in the same
// order as the CHECK constraint on accounting_expenses.category.
var AccountingExpenseCategories = []AccountingExpenseCategory{
	AccountingExpenseCategorySalary,
	AccountingExpenseCategoryReliever,
	AccountingExpenseCategoryTransport,
	AccountingExpenseCategoryUtilities,
	AccountingExpenseCategorySupplies,
	AccountingExpenseCategoryMaintenance,
	AccountingExpenseCategoryVaultTransfer,
	AccountingExpenseCategoryOwnerFunds,
	AccountingExpenseCategoryOther,
}

// AccountingExpenseLabelMaxLength bounds the free-text label of an expense line.
const AccountingExpenseLabelMaxLength = 200

// AccountingExpense is one expense line on a branch's daily accounting sheet.
// Amount is SIGNED: a positive amount is cash paid out of the day's takings, a
// negative amount is cash that came IN from outside the day's sales.
type AccountingExpense struct {
	ExpenseID     int64                     `json:"expense_id"`
	BusinessDate  time.Time                 `json:"-"`
	Date          string                    `json:"business_date"`
	BranchID      int64                     `json:"branch_id"`
	Label         string                    `json:"label"`
	Category      AccountingExpenseCategory `json:"category"`
	Amount        float64                   `json:"amount"`
	CreatedAt     time.Time                 `json:"created_at"`
	CreatedBy     *int64                    `json:"-"`
	CreatedByName string                    `json:"created_by_name"`
}

// AccountingTip is one therapist tip line on a branch's daily accounting sheet.
type AccountingTip struct {
	TipID         int64     `json:"tip_id"`
	BusinessDate  time.Time `json:"-"`
	Date          string    `json:"business_date"`
	BranchID      int64     `json:"branch_id"`
	TherapistID   int64     `json:"therapist_id"`
	TherapistName string    `json:"therapist_name"`
	Amount        float64   `json:"amount"`
	Note          string    `json:"note"`
	CreatedAt     time.Time `json:"created_at"`
	CreatedBy     *int64    `json:"-"`
}

// AccountingLineItemFilter scopes accounting line items to one branch-day.
type AccountingLineItemFilter struct {
	BusinessDate time.Time
	BranchID     int64
}

// CreateAccountingExpenseRequest is the POST /accounting/expenses body.
type CreateAccountingExpenseRequest struct {
	BusinessDate string  `json:"business_date"`
	BranchID     int64   `json:"branch_id"`
	Label        string  `json:"label"`
	Category     string  `json:"category"`
	Amount       float64 `json:"amount"`
}

// CreateAccountingTipRequest is the POST /accounting/tips body.
type CreateAccountingTipRequest struct {
	BusinessDate string  `json:"business_date"`
	BranchID     int64   `json:"branch_id"`
	TherapistID  int64   `json:"therapist_id"`
	Amount       float64 `json:"amount"`
	Note         string  `json:"note"`
}

// AccountingExpenseListResponse is the GET /accounting/expenses envelope. Total
// is the SIGNED sum of every listed amount.
type AccountingExpenseListResponse struct {
	Data  []AccountingExpense `json:"data"`
	Total float64             `json:"total"`
}

// AccountingTipListResponse is the GET /accounting/tips envelope.
type AccountingTipListResponse struct {
	Data  []AccountingTip `json:"data"`
	Total float64         `json:"total"`
}

// AccountingDayLineItems holds the raw line-item amounts recorded for a single
// (business_date, branch_id) pair. Expense amounts are signed.
type AccountingDayLineItems struct {
	ExpenseAmounts []float64
	TipAmounts     []float64
}

// AccountingLineItemTotals are the daily-sales remittance scalars derived from
// AccountingDayLineItems. The counts exist so callers can tell "no line items
// recorded" apart from "line items that happen to sum to zero" and only
// override the stored scalars in the former case.
type AccountingLineItemTotals struct {
	ExpenseCount   int
	OthersDeducted float64
	OthersAdded    float64
	TipCount       int
	TipsTotal      float64
}
