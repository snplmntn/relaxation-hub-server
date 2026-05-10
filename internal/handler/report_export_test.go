package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

type fakeReportExportService struct {
	dailyReport model.DailySalesReport
	workbook    []byte
	updateErr   error
	voidErr     error
}

func (f *fakeReportExportService) BuildDailySalesReport(ctx context.Context, businessDate time.Time) (*model.DailySalesReport, error) {
	f.dailyReport.BusinessDate = businessDate
	return &f.dailyReport, nil
}

func (f *fakeReportExportService) UpsertDailySalesRemittance(ctx context.Context, remittance model.DailySalesRemittance) (*model.DailySalesRemittance, error) {
	return &remittance, nil
}

func (f *fakeReportExportService) BuildDailySalesWorkbook(report model.DailySalesReport) ([]byte, error) {
	return f.workbook, nil
}

func (f *fakeReportExportService) ListPayrollAdjustments(ctx context.Context, filter model.PayrollAdjustmentFilter) ([]model.PayrollAdjustment, error) {
	return nil, nil
}

func (f *fakeReportExportService) CreatePayrollAdjustment(ctx context.Context, adjustment model.PayrollAdjustment) (*model.PayrollAdjustment, error) {
	adjustment.AdjustmentID = 1
	return &adjustment, nil
}

func (f *fakeReportExportService) UpdatePayrollAdjustment(ctx context.Context, adjustment model.PayrollAdjustment) (*model.PayrollAdjustment, error) {
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	return &adjustment, nil
}

func (f *fakeReportExportService) VoidPayrollAdjustment(ctx context.Context, adjustmentID int64, actorID int64) error {
	return f.voidErr
}

func (f *fakeReportExportService) BuildSalaryReport(ctx context.Context, filter model.SalaryReportFilter) (*model.SalaryReport, error) {
	return &model.SalaryReport{StartDate: filter.StartDate, EndDate: filter.EndDate}, nil
}

func (f *fakeReportExportService) BuildSalaryWorkbook(report model.SalaryReport) ([]byte, error) {
	return f.workbook, nil
}

func TestGetDailySalesReportRequiresValidBusinessDate(t *testing.T) {
	h := NewReportHandler(nil, nil, nil, nil)
	h.SetReportExportService(&fakeReportExportService{})

	req := httptest.NewRequest("GET", "/reports/daily-sales?business_date=bad", nil)
	w := httptest.NewRecorder()

	h.GetDailySalesReport(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestGetDailySalesReportReturnsBranchSectionsAndMustBeZero(t *testing.T) {
	businessDate, _ := time.Parse("2006-01-02", "2026-02-10")
	h := NewReportHandler(nil, nil, nil, nil)
	h.SetReportExportService(&fakeReportExportService{dailyReport: model.DailySalesReport{
		BusinessDate: businessDate,
		Branches: []model.DailySalesBranchSection{{
			BranchID:   1,
			BranchName: "Main",
			Remittance: model.DailySalesRemittance{BranchID: 1, BusinessDate: businessDate, OtherRemittedAmount: 25, MustBeZero: -25},
			Therapists: []model.DailySalesTherapistRow{{TherapistID: 10, TherapistName: "Ada"}},
		}},
	}})

	req := httptest.NewRequest("GET", "/reports/daily-sales?business_date=2026-02-10", nil)
	w := httptest.NewRecorder()

	h.GetDailySalesReport(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var resp model.DailySalesReport
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Branches) != 1 || len(resp.Branches[0].Therapists) != 1 {
		t.Fatalf("unexpected response shape: %#v", resp)
	}
	if resp.Branches[0].Remittance.OtherRemittedAmount != 25 || resp.Branches[0].Remittance.MustBeZero != -25 {
		t.Fatalf("expected remittance fields with must_be_zero, got %#v", resp.Branches[0].Remittance)
	}
}

func TestExportDailySalesWorkbookReturnsExcelAttachment(t *testing.T) {
	h := NewReportHandler(nil, nil, nil, nil)
	h.SetReportExportService(&fakeReportExportService{workbook: []byte("xlsx")})

	req := httptest.NewRequest("GET", "/reports/daily-sales/export?business_date=2026-02-10", nil)
	w := httptest.NewRecorder()

	h.ExportDailySalesReport(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Type"); got != model.ExcelContentType {
		t.Fatalf("expected Excel content type, got %q", got)
	}
	if got := w.Header().Get("Content-Disposition"); got == "" {
		t.Fatal("expected attachment content disposition")
	}
}

func TestCreatePayrollAdjustmentValidatesPayload(t *testing.T) {
	h := NewReportHandler(nil, nil, nil, nil)
	h.SetReportExportService(&fakeReportExportService{})

	body := []byte(`{"therapist_id":10,"adjustment_date":"2026-02-10","period_start":"2026-02-01","period_end":"2026-02-15","type":"add","category":"benefits","amount":50,"reason":"Benefit"}`)
	req := httptest.NewRequest("POST", "/reports/payroll-adjustments", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.CreatePayrollAdjustment(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestUpsertDailySalesRemittanceRejectsNegativePayload(t *testing.T) {
	h := NewReportHandler(nil, nil, nil, nil)
	h.SetReportExportService(&fakeReportExportService{})

	body := []byte(`{"business_date":"2026-02-10","branch_id":1,"bill_1000":-1,"actual_remitted":10}`)
	req := httptest.NewRequest("PUT", "/reports/daily-sales/remittances", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.UpsertDailySalesRemittance(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestExportTherapistSalariesRejectsInvalidRanges(t *testing.T) {
	h := NewReportHandler(nil, nil, nil, nil)
	h.SetReportExportService(&fakeReportExportService{workbook: []byte("xlsx")})

	cases := []string{
		"/reports/therapist-salaries/export?start_date=2026-02-15&end_date=2026-02-01",
		"/reports/therapist-salaries/export?start_date=2026-01-01&end_date=2026-04-01",
	}
	for _, target := range cases {
		req := httptest.NewRequest("GET", target, nil)
		w := httptest.NewRecorder()
		h.ExportTherapistSalaries(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s: expected 400, got %d body=%s", target, w.Code, w.Body.String())
		}
	}
}

func TestPayrollAdjustmentNotFoundMapsTo404(t *testing.T) {
	h := NewReportHandler(nil, nil, nil, nil)
	h.SetReportExportService(&fakeReportExportService{updateErr: model.ErrNotFound, voidErr: model.ErrNotFound})

	body := []byte(`{"therapist_id":10,"adjustment_date":"2026-02-10","period_start":"2026-02-01","period_end":"2026-02-15","type":"add","category":"benefits","amount":50,"reason":"Benefit"}`)
	req := httptest.NewRequest("PATCH", "/reports/payroll-adjustments/99", bytes.NewReader(body))
	req.SetPathValue("id", "99")
	w := httptest.NewRecorder()
	h.UpdatePayrollAdjustment(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected update 404, got %d body=%s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest("DELETE", "/reports/payroll-adjustments/99", nil)
	req.SetPathValue("id", "99")
	w = httptest.NewRecorder()
	h.DeletePayrollAdjustment(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected delete 404, got %d body=%s", w.Code, w.Body.String())
	}
}
