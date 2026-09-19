package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

type fakeAccountingRepo struct {
	expenses         []model.AccountingExpense
	tips             []model.AccountingTip
	createdExpense   *model.AccountingExpense
	createdTip       *model.AccountingTip
	therapistExists  bool
	deletedExpenseID int64
	deletedTipID     int64
	deleteErr        error
}

func (f *fakeAccountingRepo) ListExpenses(_ context.Context, _ model.AccountingLineItemFilter) ([]model.AccountingExpense, error) {
	return f.expenses, nil
}

func (f *fakeAccountingRepo) CreateExpense(_ context.Context, expense model.AccountingExpense) (*model.AccountingExpense, error) {
	expense.ExpenseID = 1
	f.createdExpense = &expense
	return &expense, nil
}

func (f *fakeAccountingRepo) DeleteExpense(_ context.Context, expenseID int64) error {
	f.deletedExpenseID = expenseID
	return f.deleteErr
}

func (f *fakeAccountingRepo) ListTips(_ context.Context, _ model.AccountingLineItemFilter) ([]model.AccountingTip, error) {
	return f.tips, nil
}

func (f *fakeAccountingRepo) CreateTip(_ context.Context, tip model.AccountingTip) (*model.AccountingTip, error) {
	tip.TipID = 1
	f.createdTip = &tip
	return &tip, nil
}

func (f *fakeAccountingRepo) DeleteTip(_ context.Context, tipID int64) error {
	f.deletedTipID = tipID
	return f.deleteErr
}

func (f *fakeAccountingRepo) TherapistExists(_ context.Context, _ int64) (bool, error) {
	return f.therapistExists, nil
}

func TestSplitSignedExpenseAmountsSeparatesDeductedFromAdded(t *testing.T) {
	deducted, added := SplitSignedExpenseAmounts([]float64{3656, -1000, 250.5, 0, -49.5})

	if deducted != 3906.5 {
		t.Fatalf("expected others_deducted 3906.50, got %.2f", deducted)
	}
	// Negative amounts mean cash came IN, so they are reported as absolute values.
	if added != 1049.5 {
		t.Fatalf("expected others_added 1049.50, got %.2f", added)
	}
}

func TestSplitSignedExpenseAmountsHandlesEmptyAndSingleSignSets(t *testing.T) {
	deducted, added := SplitSignedExpenseAmounts(nil)
	if deducted != 0 || added != 0 {
		t.Fatalf("expected zero buckets for no line items, got %.2f/%.2f", deducted, added)
	}

	deducted, added = SplitSignedExpenseAmounts([]float64{100, 200})
	if deducted != 300 || added != 0 {
		t.Fatalf("expected all-positive split 300/0, got %.2f/%.2f", deducted, added)
	}

	deducted, added = SplitSignedExpenseAmounts([]float64{-100, -200})
	if deducted != 0 || added != 300 {
		t.Fatalf("expected all-negative split 0/300, got %.2f/%.2f", deducted, added)
	}
}

func TestSumSignedAmountsPreservesSign(t *testing.T) {
	if total := SumSignedAmounts([]float64{3656, -4000}); total != -344 {
		t.Fatalf("expected signed total -344, got %.2f", total)
	}
	if total := SumSignedAmounts(nil); total != 0 {
		t.Fatalf("expected zero total for no amounts, got %.2f", total)
	}
}

func TestDeriveAccountingLineItemTotalsCountsLineItems(t *testing.T) {
	totals := DeriveAccountingLineItemTotals(model.AccountingDayLineItems{
		ExpenseAmounts: []float64{3656, -1000},
		TipAmounts:     []float64{100, 50},
	})

	if totals.ExpenseCount != 2 || totals.OthersDeducted != 3656 || totals.OthersAdded != 1000 {
		t.Fatalf("unexpected expense totals: %#v", totals)
	}
	if totals.TipCount != 2 || totals.TipsTotal != 150 {
		t.Fatalf("unexpected tip totals: %#v", totals)
	}
}

func TestApplyAccountingLineItemTotalsOnlyOverridesWhenLineItemsExist(t *testing.T) {
	remittance := model.DailySalesRemittance{TipsTotal: 15, OthersDeducted: 10, OthersAdded: 25}

	// No line items recorded: the hand-entered scalars must survive untouched.
	ApplyAccountingLineItemTotals(&remittance, DeriveAccountingLineItemTotals(model.AccountingDayLineItems{}))
	if remittance.TipsTotal != 15 || remittance.OthersDeducted != 10 || remittance.OthersAdded != 25 {
		t.Fatalf("expected stored scalars preserved, got %#v", remittance)
	}

	// Line items that sum to zero still count as recorded and must override.
	ApplyAccountingLineItemTotals(&remittance, DeriveAccountingLineItemTotals(model.AccountingDayLineItems{
		ExpenseAmounts: []float64{500, -500},
		TipAmounts:     []float64{0},
	}))
	if remittance.TipsTotal != 0 || remittance.OthersDeducted != 500 || remittance.OthersAdded != 500 {
		t.Fatalf("expected derived scalars applied, got %#v", remittance)
	}
}

func TestApplyAccountingLineItemTotalsOverridesTipsIndependentlyOfExpenses(t *testing.T) {
	remittance := model.DailySalesRemittance{TipsTotal: 15, OthersDeducted: 10, OthersAdded: 25}

	ApplyAccountingLineItemTotals(&remittance, DeriveAccountingLineItemTotals(model.AccountingDayLineItems{
		TipAmounts: []float64{100},
	}))

	if remittance.TipsTotal != 100 {
		t.Fatalf("expected tips_total derived from tip rows, got %.2f", remittance.TipsTotal)
	}
	if remittance.OthersDeducted != 10 || remittance.OthersAdded != 25 {
		t.Fatalf("expected expense scalars untouched with no expense rows, got %#v", remittance)
	}
}

func TestBuildDailySalesReportDerivesRemittanceScalarsFromLineItems(t *testing.T) {
	businessDate, _ := time.Parse("2006-01-02", "2026-08-04")
	repo := &fakeReportExportRepository{
		roster: []model.ReportTherapistRosterRow{{BranchID: 1, BranchName: "Main", TherapistID: 10, TherapistName: "Dimple"}},
		dailySales: []model.ReportDailySalesBookingRow{
			{BranchID: 1, BranchName: "Main", TherapistID: 10, TherapistName: "Dimple", PaymentMethod: "cash", TotalSales: 5000, TotalHours: 2, BookingCount: 2},
		},
		remittances: []model.DailySalesRemittance{{
			BusinessDate: businessDate, BranchID: 1,
			ActualRemitted: 5000,
			TipsTotal:      999, OthersDeducted: 999, OthersAdded: 999,
		}},
		accountingLineItems: map[int64]model.AccountingDayLineItems{
			1: {ExpenseAmounts: []float64{3656, -1000}, TipAmounts: []float64{100}},
		},
	}
	svc := NewReportExportService(repo)

	report, err := svc.BuildDailySalesReport(context.Background(), businessDate)
	if err != nil {
		t.Fatalf("BuildDailySalesReport returned error: %v", err)
	}
	if len(report.Branches) != 1 {
		t.Fatalf("expected one branch, got %#v", report.Branches)
	}
	remittance := report.Branches[0].Remittance
	if remittance.TipsTotal != 100 || remittance.OthersDeducted != 3656 || remittance.OthersAdded != 1000 {
		t.Fatalf("expected line-item derived scalars to replace stored ones, got %#v", remittance)
	}
	// must_be_zero must be computed from the derived scalars, not the stored ones.
	expected := CalculateDailySalesMustBeZero(5000, remittance)
	if remittance.MustBeZero != expected {
		t.Fatalf("expected must_be_zero %.2f, got %.2f", expected, remittance.MustBeZero)
	}
}

func TestBuildDailySalesReportKeepsStoredScalarsWithoutLineItems(t *testing.T) {
	businessDate, _ := time.Parse("2006-01-02", "2026-08-04")
	repo := &fakeReportExportRepository{
		roster: []model.ReportTherapistRosterRow{{BranchID: 1, BranchName: "Main", TherapistID: 10, TherapistName: "Dimple"}},
		remittances: []model.DailySalesRemittance{{
			BusinessDate: businessDate, BranchID: 1,
			TipsTotal: 15, OthersDeducted: 10, OthersAdded: 25,
		}},
	}
	svc := NewReportExportService(repo)

	report, err := svc.BuildDailySalesReport(context.Background(), businessDate)
	if err != nil {
		t.Fatalf("BuildDailySalesReport returned error: %v", err)
	}
	remittance := report.Branches[0].Remittance
	if remittance.TipsTotal != 15 || remittance.OthersDeducted != 10 || remittance.OthersAdded != 25 {
		t.Fatalf("expected legacy hand-entered scalars preserved, got %#v", remittance)
	}
}

func TestParseAccountingBusinessDateRejectsBadFormats(t *testing.T) {
	if _, err := ParseAccountingBusinessDate("2026-08-04"); err != nil {
		t.Fatalf("expected 2026-08-04 to parse, got %v", err)
	}
	for _, value := range []string{"", "04-08-2026", "2026/08/04", "2026-08-04T00:00:00Z", "not-a-date"} {
		_, err := ParseAccountingBusinessDate(value)
		assertValidationCode(t, err, "invalid_business_date")
	}
}

func TestParseAccountingExpenseCategoryAcceptsOnlyAllowedValues(t *testing.T) {
	for _, category := range model.AccountingExpenseCategories {
		parsed, err := ParseAccountingExpenseCategory(string(category))
		if err != nil || parsed != category {
			t.Fatalf("expected %q to be accepted, got %v / %v", category, parsed, err)
		}
	}
	for _, value := range []string{"", "Salary", "rent", "tips"} {
		_, err := ParseAccountingExpenseCategory(value)
		assertValidationCode(t, err, "invalid_category")
	}
}

func TestValidateSignedExpenseAmountRejectsZeroAndKeepsSign(t *testing.T) {
	amount, err := ValidateSignedExpenseAmount(-1000.004)
	if err != nil {
		t.Fatalf("expected negative expense to be accepted, got %v", err)
	}
	if amount != -1000 {
		t.Fatalf("expected rounded -1000, got %.4f", amount)
	}
	for _, value := range []float64{0, 0.001, -0.004} {
		_, err := ValidateSignedExpenseAmount(value)
		assertValidationCode(t, err, "invalid_amount")
	}
}

func TestValidateTipAmountRequiresPositive(t *testing.T) {
	if _, err := ValidateTipAmount(100); err != nil {
		t.Fatalf("expected positive tip to be accepted, got %v", err)
	}
	for _, value := range []float64{0, -100} {
		_, err := ValidateTipAmount(value)
		assertValidationCode(t, err, "invalid_amount")
	}
}

func TestCreateExpenseValidatesPayload(t *testing.T) {
	base := model.CreateAccountingExpenseRequest{BusinessDate: "2026-08-04", BranchID: 1, Label: "Cio salary", Category: "salary", Amount: 3656}
	cases := []struct {
		name    string
		mutate  func(*model.CreateAccountingExpenseRequest)
		code    string
		details string
	}{
		{"missing date", func(r *model.CreateAccountingExpenseRequest) { r.BusinessDate = "" }, "invalid_business_date", "business_date"},
		{"zero branch", func(r *model.CreateAccountingExpenseRequest) { r.BranchID = 0 }, "invalid_branch", "branch_id"},
		{"blank label", func(r *model.CreateAccountingExpenseRequest) { r.Label = "   " }, "invalid_label", "label"},
		{"long label", func(r *model.CreateAccountingExpenseRequest) { r.Label = strings.Repeat("a", 201) }, "invalid_label", "label"},
		{"bad category", func(r *model.CreateAccountingExpenseRequest) { r.Category = "rent" }, "invalid_category", "category"},
		{"zero amount", func(r *model.CreateAccountingExpenseRequest) { r.Amount = 0 }, "invalid_amount", "amount"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			req := base
			testCase.mutate(&req)
			svc := NewAccountingService(&fakeAccountingRepo{})

			_, err := svc.CreateExpense(context.Background(), &req, 7)

			assertValidationCode(t, err, testCase.code)
			var validationErr *ValidationError
			if errors.As(err, &validationErr) {
				if _, ok := validationErr.Details[testCase.details]; !ok {
					t.Fatalf("expected details for %q, got %#v", testCase.details, validationErr.Details)
				}
			}
		})
	}
}

func TestCreateExpenseTrimsLabelAndStampsActor(t *testing.T) {
	repo := &fakeAccountingRepo{}
	svc := NewAccountingService(repo)

	created, err := svc.CreateExpense(context.Background(), &model.CreateAccountingExpenseRequest{
		BusinessDate: "2026-08-04", BranchID: 1, Label: "  Cio salary  ", Category: "salary", Amount: 3656.004,
	}, 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.Label != "Cio salary" {
		t.Fatalf("expected trimmed label, got %q", created.Label)
	}
	if created.Amount != 3656 {
		t.Fatalf("expected rounded amount 3656, got %.4f", created.Amount)
	}
	if created.Date != "2026-08-04" {
		t.Fatalf("expected business_date 2026-08-04, got %q", created.Date)
	}
	if repo.createdExpense == nil || repo.createdExpense.CreatedBy == nil || *repo.createdExpense.CreatedBy != 7 {
		t.Fatalf("expected created_by stamped with the acting user, got %#v", repo.createdExpense)
	}
}

func TestCreateExpenseAcceptsNegativeAmountAsCashIn(t *testing.T) {
	svc := NewAccountingService(&fakeAccountingRepo{})

	created, err := svc.CreateExpense(context.Background(), &model.CreateAccountingExpenseRequest{
		BusinessDate: "2026-08-04", BranchID: 1, Label: "Owner funds drop", Category: "owner_funds", Amount: -5000,
	}, 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.Amount != -5000 {
		t.Fatalf("expected the negative amount to be preserved, got %.2f", created.Amount)
	}
}

func TestCreateTipValidatesTherapistAndAmount(t *testing.T) {
	base := model.CreateAccountingTipRequest{BusinessDate: "2026-08-04", BranchID: 1, TherapistID: 10, Amount: 100}

	svc := NewAccountingService(&fakeAccountingRepo{therapistExists: false})
	_, err := svc.CreateTip(context.Background(), &base, 7)
	assertValidationCode(t, err, "invalid_therapist")

	missingTherapist := base
	missingTherapist.TherapistID = 0
	svc = NewAccountingService(&fakeAccountingRepo{therapistExists: true})
	_, err = svc.CreateTip(context.Background(), &missingTherapist, 7)
	assertValidationCode(t, err, "invalid_therapist")

	zeroAmount := base
	zeroAmount.Amount = 0
	_, err = svc.CreateTip(context.Background(), &zeroAmount, 7)
	assertValidationCode(t, err, "invalid_amount")

	badDate := base
	badDate.BusinessDate = "08/04/2026"
	_, err = svc.CreateTip(context.Background(), &badDate, 7)
	assertValidationCode(t, err, "invalid_business_date")

	zeroBranch := base
	zeroBranch.BranchID = 0
	_, err = svc.CreateTip(context.Background(), &zeroBranch, 7)
	assertValidationCode(t, err, "invalid_branch")
}

func TestCreateTipTrimsNoteAndStampsActor(t *testing.T) {
	repo := &fakeAccountingRepo{therapistExists: true}
	svc := NewAccountingService(repo)

	created, err := svc.CreateTip(context.Background(), &model.CreateAccountingTipRequest{
		BusinessDate: "2026-08-04", BranchID: 1, TherapistID: 10, Amount: 100, Note: "  walk-in  ",
	}, 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.Note != "walk-in" {
		t.Fatalf("expected trimmed note, got %q", created.Note)
	}
	if repo.createdTip == nil || repo.createdTip.CreatedBy == nil || *repo.createdTip.CreatedBy != 7 {
		t.Fatalf("expected created_by stamped with the acting user, got %#v", repo.createdTip)
	}
}

func TestListExpensesReturnsSignedTotal(t *testing.T) {
	repo := &fakeAccountingRepo{expenses: []model.AccountingExpense{
		{ExpenseID: 1, Amount: 3656},
		{ExpenseID: 2, Amount: -1000},
	}}
	svc := NewAccountingService(repo)

	rows, total, err := svc.ListExpenses(context.Background(), "2026-08-04", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected two rows, got %d", len(rows))
	}
	if total != 2656 {
		t.Fatalf("expected signed total 2656, got %.2f", total)
	}
}

func TestListLineItemsRejectsBadFilters(t *testing.T) {
	svc := NewAccountingService(&fakeAccountingRepo{})

	_, _, err := svc.ListExpenses(context.Background(), "", 1)
	assertValidationCode(t, err, "invalid_business_date")

	_, _, err = svc.ListExpenses(context.Background(), "2026-08-04", 0)
	assertValidationCode(t, err, "invalid_branch")

	_, _, err = svc.ListTips(context.Background(), "2026-08-04", -1)
	assertValidationCode(t, err, "invalid_branch")
}

func TestDeleteLineItemsRejectNonPositiveIDsAsNotFound(t *testing.T) {
	repo := &fakeAccountingRepo{}
	svc := NewAccountingService(repo)

	if err := svc.DeleteExpense(context.Background(), 0); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for id 0, got %v", err)
	}
	if err := svc.DeleteTip(context.Background(), -5); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for negative id, got %v", err)
	}
	if repo.deletedExpenseID != 0 || repo.deletedTipID != 0 {
		t.Fatal("expected invalid ids to never reach the repository")
	}

	if err := svc.DeleteExpense(context.Background(), 12); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.deletedExpenseID != 12 {
		t.Fatalf("expected delete to forward id 12, got %d", repo.deletedExpenseID)
	}
}

func assertValidationCode(t *testing.T, err error, code string) {
	t.Helper()
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected *ValidationError, got %v", err)
	}
	if validationErr.Code != code {
		t.Fatalf("expected validation code %q, got %q", code, validationErr.Code)
	}
}
