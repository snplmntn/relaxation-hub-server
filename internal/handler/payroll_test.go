package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/snplmntn/relaxation-hub-server/internal/auth"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

type fakePayrollService struct {
	rates            []model.StaffCompensationRate
	adjustments      []model.StaffPayrollAdjustment
	runs             []model.PayrollRun
	runDetail        *model.PayrollRun
	rateFilterUserID *int64
	rateFilterRole   string
	createdRate      model.StaffCompensationRate
	generationFilter model.PayrollGenerationFilter
	createdAdj       model.StaffPayrollAdjustment
	updatedAdj       model.StaffPayrollAdjustment
	voidedAdjID      int64
	voidedActorID    int64
	adjFilter        repository.StaffPayrollAdjustmentFilter
	profileUserID    int64
	profileBranchID  *int64
	profileLocation  string
	approvedRunID    int64
	approvedActorID  int64
	voidedRunID      int64
	voidedRunActorID int64
	voidedRunReason  string
	paidRunID        int64
	paidRowID        int64
	paidActorID      int64
	paidRequest      model.PayrollPaymentRequest
	paidRow          *model.PayrollRow
	stale            bool
	staleReasons     []string
	staleRunID       int64
	workbookRunID    int64
	workbookBytes    []byte
	pdfRunID         int64
	pdfDraft         bool
	pdfBytes         []byte
	err              error
}

func (f *fakePayrollService) CreateCompensationRate(ctx context.Context, rate model.StaffCompensationRate, actorID int64) (*model.StaffCompensationRate, error) {
	f.createdRate = rate
	f.voidedActorID = actorID
	if f.err != nil {
		return nil, f.err
	}
	rate.RateID = 91
	return &rate, nil
}

func (f *fakePayrollService) ListCompensationRates(ctx context.Context, userID *int64, role string) ([]model.StaffCompensationRate, error) {
	f.rateFilterUserID = userID
	f.rateFilterRole = role
	return f.rates, f.err
}

func (f *fakePayrollService) UpsertStaffProfile(ctx context.Context, userID int64, branchID *int64, locationLabel string) error {
	f.profileUserID = userID
	f.profileBranchID = branchID
	f.profileLocation = locationLabel
	return f.err
}

func (f *fakePayrollService) ListStaffPayrollAdjustments(ctx context.Context, filter repository.StaffPayrollAdjustmentFilter) ([]model.StaffPayrollAdjustment, error) {
	f.adjFilter = filter
	return f.adjustments, f.err
}

func (f *fakePayrollService) CreateStaffPayrollAdjustment(ctx context.Context, adjustment model.StaffPayrollAdjustment, actorID int64) (*model.StaffPayrollAdjustment, error) {
	f.createdAdj = adjustment
	f.voidedActorID = actorID
	if f.err != nil {
		return nil, f.err
	}
	adjustment.AdjustmentID = 81
	return &adjustment, nil
}

func (f *fakePayrollService) UpdateStaffPayrollAdjustment(ctx context.Context, adjustment model.StaffPayrollAdjustment, actorID int64) (*model.StaffPayrollAdjustment, error) {
	f.updatedAdj = adjustment
	f.voidedActorID = actorID
	return &adjustment, f.err
}

func (f *fakePayrollService) VoidStaffPayrollAdjustment(ctx context.Context, adjustmentID int64, actorID int64) error {
	f.voidedAdjID = adjustmentID
	f.voidedActorID = actorID
	return f.err
}

func (f *fakePayrollService) GenerateDraftPayrollRun(ctx context.Context, filter model.PayrollGenerationFilter) (*model.PayrollRun, error) {
	f.generationFilter = filter
	f.voidedActorID = filter.GeneratedBy
	if f.err != nil {
		return nil, f.err
	}
	return &model.PayrollRun{
		PayrollRunID: 71,
		PeriodStart:  filter.PeriodStart,
		StartDate:    filter.PeriodStart.Format("2006-01-02"),
		PeriodEnd:    filter.PeriodEnd,
		EndDate:      filter.PeriodEnd.Format("2006-01-02"),
		Status:       model.PayrollRunStatusDraft,
	}, nil
}

func (f *fakePayrollService) ListPayrollRuns(ctx context.Context) ([]model.PayrollRun, error) {
	return f.runs, f.err
}

func (f *fakePayrollService) GetPayrollRun(ctx context.Context, runID int64) (*model.PayrollRun, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.runDetail, nil
}

func (f *fakePayrollService) ApprovePayrollRun(ctx context.Context, runID int64, actorID int64) error {
	f.approvedRunID = runID
	f.approvedActorID = actorID
	return f.err
}

func (f *fakePayrollService) VoidPayrollRun(ctx context.Context, runID int64, actorID int64, reason string) error {
	f.voidedRunID = runID
	f.voidedRunActorID = actorID
	f.voidedRunReason = reason
	return f.err
}

func (f *fakePayrollService) MarkPayrollRowPaid(ctx context.Context, runID, rowID, actorID int64, req model.PayrollPaymentRequest) (*model.PayrollRow, error) {
	f.paidRunID = runID
	f.paidRowID = rowID
	f.paidActorID = actorID
	f.paidRequest = req
	if f.err != nil {
		return nil, f.err
	}
	if f.paidRow != nil {
		return f.paidRow, nil
	}
	return &model.PayrollRow{PayrollRunID: runID, PayrollRowID: rowID, Status: model.PayrollRowStatusPaid}, nil
}

func (f *fakePayrollService) CheckPayrollRunStaleness(ctx context.Context, runID int64) (bool, []string, error) {
	f.staleRunID = runID
	return f.stale, f.staleReasons, f.err
}

func (f *fakePayrollService) BuildPayrollWorkbook(ctx context.Context, runID int64) ([]byte, error) {
	f.workbookRunID = runID
	if f.err != nil {
		return nil, f.err
	}
	return f.workbookBytes, nil
}

func (f *fakePayrollService) BuildPayrollPayslipPDF(ctx context.Context, runID int64, draftWatermark bool) ([]byte, error) {
	f.pdfRunID = runID
	f.pdfDraft = draftWatermark
	if f.err != nil {
		return nil, f.err
	}
	return f.pdfBytes, nil
}

func TestPayrollHandlerCreateRateParsesPayloadAndActor(t *testing.T) {
	service := &fakePayrollService{}
	handler := NewPayrollHandler(service)
	jwtKey := "test-secret-key-32-characters-long"
	token, err := auth.GenerateToken(7, model.RoleSuperAdmin, jwtKey)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	req := httptest.NewRequest("POST", "/payroll/rates", bytes.NewReader([]byte(`{"user_id":12,"role":"rider","daily_rate_cents":120000,"overtime_multiplier":1.25,"effective_from":"2026-05-18","notes":" base "}`)))
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	middleware.AuthMiddleware(http.HandlerFunc(handler.CreateCompensationRate), jwtKey).ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", w.Code, w.Body.String())
	}
	if service.createdRate.UserID != 12 || service.createdRate.Role != model.PayrollRoleRider || service.createdRate.EffectiveFrom.Format("2006-01-02") != "2026-05-18" || service.createdRate.Notes != "base" {
		t.Fatalf("unexpected created rate: %#v", service.createdRate)
	}
}

func TestPayrollHandlerListRatesParsesFilters(t *testing.T) {
	service := &fakePayrollService{}
	handler := NewPayrollHandler(service)
	req := httptest.NewRequest("GET", "/payroll/rates?user_id=12&role=admin", nil)
	w := httptest.NewRecorder()

	handler.ListCompensationRates(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if service.rateFilterUserID == nil || *service.rateFilterUserID != 12 || service.rateFilterRole != model.RoleAdmin {
		t.Fatalf("unexpected filters: user=%v role=%q", service.rateFilterUserID, service.rateFilterRole)
	}
}

func TestPayrollHandlerCreateAdjustmentParsesPayload(t *testing.T) {
	service := &fakePayrollService{}
	handler := NewPayrollHandler(service)
	jwtKey := "test-secret-key-32-characters-long"
	token, err := auth.GenerateToken(7, model.RoleSuperAdmin, jwtKey)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	req := payrollRequest("POST", "/payroll/adjustments", []byte(`{"user_id":22,"role":"therapist","adjustment_date":"2026-05-17","period_start":"2026-05-01","period_end":"2026-05-31","type":"add","category":"bonus","amount_cents":5000,"reason":" Coverage ","cash_movement_cents":0}`))
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	middleware.AuthMiddleware(http.HandlerFunc(handler.CreateStaffPayrollAdjustment), jwtKey).ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", w.Code, w.Body.String())
	}
	if service.createdAdj.UserID != 22 || service.createdAdj.Role != model.PayrollRoleTherapist || service.createdAdj.Reason != "Coverage" {
		t.Fatalf("unexpected adjustment: %#v", service.createdAdj)
	}
	if service.voidedActorID != 7 {
		t.Fatalf("expected actor 7, got %d", service.voidedActorID)
	}
}

func TestPayrollHandlerUpdateAndDeleteAdjustmentUsePathID(t *testing.T) {
	service := &fakePayrollService{}
	handler := NewPayrollHandler(service)
	jwtKey := "test-secret-key-32-characters-long"
	token, err := auth.GenerateToken(7, model.RoleSuperAdmin, jwtKey)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	updateReq := payrollRequest("PATCH", "/payroll/adjustments/44", []byte(`{"user_id":22,"role":"rider","adjustment_date":"2026-05-17","period_start":"2026-05-01","period_end":"2026-05-31","type":"minus","category":"deduction","amount_cents":5000,"reason":" Deduct ","cash_movement_cents":0}`))
	updateReq.Header.Set("Authorization", "Bearer "+token)
	updateReq.SetPathValue("id", "44")
	updateW := httptest.NewRecorder()
	middleware.AuthMiddleware(http.HandlerFunc(handler.UpdateStaffPayrollAdjustment), jwtKey).ServeHTTP(updateW, updateReq)
	if updateW.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", updateW.Code, updateW.Body.String())
	}
	if service.updatedAdj.AdjustmentID != 44 {
		t.Fatalf("expected path id on update, got %#v", service.updatedAdj)
	}

	deleteReq := payrollRequest("DELETE", "/payroll/adjustments/44", nil)
	deleteReq.Header.Set("Authorization", "Bearer "+token)
	deleteReq.SetPathValue("id", "44")
	deleteW := httptest.NewRecorder()
	middleware.AuthMiddleware(http.HandlerFunc(handler.DeleteStaffPayrollAdjustment), jwtKey).ServeHTTP(deleteW, deleteReq)
	if deleteW.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d body=%s", deleteW.Code, deleteW.Body.String())
	}
	if service.voidedAdjID != 44 {
		t.Fatalf("expected void id 44, got %d", service.voidedAdjID)
	}
}

func TestPayrollHandlerActorSensitiveEndpointsRequireActor(t *testing.T) {
	handler := NewPayrollHandler(&fakePayrollService{})
	cases := []struct {
		name    string
		request *http.Request
		handle  http.HandlerFunc
	}{
		{
			name:    "create rate",
			request: payrollRequest("POST", "/payroll/rates", []byte(`{"user_id":12,"role":"rider","daily_rate_cents":120000,"effective_from":"2026-05-18"}`)),
			handle:  handler.CreateCompensationRate,
		},
		{
			name:    "create adjustment",
			request: payrollRequest("POST", "/payroll/adjustments", []byte(`{"user_id":22,"role":"rider","adjustment_date":"2026-05-17","period_start":"2026-05-01","period_end":"2026-05-31","type":"add","category":"bonus","amount_cents":5000,"reason":"Coverage"}`)),
			handle:  handler.CreateStaffPayrollAdjustment,
		},
		{
			name:    "update adjustment",
			request: payrollRequest("PATCH", "/payroll/adjustments/44", []byte(`{"user_id":22,"role":"rider","adjustment_date":"2026-05-17","period_start":"2026-05-01","period_end":"2026-05-31","type":"add","category":"bonus","amount_cents":5000,"reason":"Coverage"}`)),
			handle:  handler.UpdateStaffPayrollAdjustment,
		},
		{
			name:    "delete adjustment",
			request: payrollRequest("DELETE", "/payroll/adjustments/44", nil),
			handle:  handler.DeleteStaffPayrollAdjustment,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.request.SetPathValue("id", "44")
			w := httptest.NewRecorder()
			tc.handle(w, tc.request)
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401, got %d body=%s", w.Code, w.Body.String())
			}
		})
	}
}

func TestPayrollHandlerStaffProfileParsesPayload(t *testing.T) {
	service := &fakePayrollService{}
	handler := NewPayrollHandler(service)
	req := payrollRequest("PUT", "/payroll/staff-profiles/22", []byte(`{"usual_branch_id":3,"usual_location_label":" Makati "}`))
	req.SetPathValue("userID", "22")
	w := httptest.NewRecorder()

	handler.UpsertStaffProfile(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d body=%s", w.Code, w.Body.String())
	}
	if service.profileUserID != 22 || service.profileBranchID == nil || *service.profileBranchID != 3 || service.profileLocation != "Makati" {
		t.Fatalf("unexpected profile call: user=%d branch=%v location=%q", service.profileUserID, service.profileBranchID, service.profileLocation)
	}
}

func TestPayrollHandlerMapsRateAdjustmentAndStaffProfileErrors(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{name: "invalid role", err: model.ErrInvalidPayrollRole, want: http.StatusBadRequest},
		{name: "invalid adjustment", err: model.ErrInvalidPayrollAdjustment, want: http.StatusBadRequest},
		{name: "rate locked", err: model.ErrPayrollRateLocked, want: http.StatusConflict},
		{name: "adjustment locked", err: model.ErrPayrollAdjustmentLocked, want: http.StatusConflict},
		{name: "not found", err: model.ErrNotFound, want: http.StatusNotFound},
		{name: "unexpected", err: errors.New("db down"), want: http.StatusInternalServerError},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler := NewPayrollHandler(&fakePayrollService{err: tc.err})
			req := payrollRequest("POST", "/payroll/adjustments", []byte(`{"user_id":22,"role":"rider","adjustment_date":"2026-05-17","period_start":"2026-05-01","period_end":"2026-05-31","type":"add","category":"bonus","amount_cents":5000,"reason":"Coverage","cash_movement_cents":0}`))
			jwtKey := "test-secret-key-32-characters-long"
			token, err := auth.GenerateToken(7, model.RoleSuperAdmin, jwtKey)
			if err != nil {
				t.Fatalf("generate token: %v", err)
			}
			req.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()

			middleware.AuthMiddleware(http.HandlerFunc(handler.CreateStaffPayrollAdjustment), jwtKey).ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Fatalf("expected %d, got %d body=%s", tc.want, w.Code, w.Body.String())
			}
		})
	}
}

func TestPayrollHandlerListAdjustmentsParsesFilters(t *testing.T) {
	service := &fakePayrollService{}
	handler := NewPayrollHandler(service)
	req := httptest.NewRequest("GET", "/payroll/adjustments?period_start=2026-05-01&period_end=2026-05-31&user_id=22&role=rider", nil)
	w := httptest.NewRecorder()

	handler.ListStaffPayrollAdjustments(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if service.adjFilter.UserID == nil || *service.adjFilter.UserID != 22 || service.adjFilter.Role != model.RoleRider || service.adjFilter.PeriodStart.Format("2006-01-02") != "2026-05-01" {
		t.Fatalf("unexpected filter: %#v", service.adjFilter)
	}
	var body struct {
		Data []model.StaffPayrollAdjustment `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
}

func TestPayrollHandlerCreateRunParsesPayloadAndActor(t *testing.T) {
	service := &fakePayrollService{}
	handler := NewPayrollHandler(service)
	jwtKey := "test-secret-key-32-characters-long"
	token, err := auth.GenerateToken(7, model.RoleSuperAdmin, jwtKey)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	req := payrollRequest("POST", "/payroll/runs", []byte(`{"period_start":"2026-05-01","period_end":"2026-05-31"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	middleware.AuthMiddleware(http.HandlerFunc(handler.CreatePayrollRun), jwtKey).ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", w.Code, w.Body.String())
	}
	if service.generationFilter.GeneratedBy != 7 ||
		service.generationFilter.PeriodStart.Format("2006-01-02") != "2026-05-01" ||
		service.generationFilter.PeriodEnd.Format("2006-01-02") != "2026-05-31" {
		t.Fatalf("unexpected generation filter: %#v", service.generationFilter)
	}
}

func TestPayrollHandlerListRunsWritesData(t *testing.T) {
	service := &fakePayrollService{runs: []model.PayrollRun{{PayrollRunID: 71, Status: model.PayrollRunStatusDraft}}}
	handler := NewPayrollHandler(service)
	req := payrollRequest("GET", "/payroll/runs", nil)
	w := httptest.NewRecorder()

	handler.ListPayrollRuns(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Data []model.PayrollRun `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(body.Data) != 1 || body.Data[0].PayrollRunID != 71 {
		t.Fatalf("unexpected runs response: %#v", body)
	}
}

func TestPayrollHandlerGetRunUsesPathID(t *testing.T) {
	service := &fakePayrollService{runDetail: &model.PayrollRun{PayrollRunID: 72, Status: model.PayrollRunStatusDraft}}
	handler := NewPayrollHandler(service)
	req := payrollRequest("GET", "/payroll/runs/72", nil)
	req.SetPathValue("id", "72")
	w := httptest.NewRecorder()

	handler.GetPayrollRun(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var body model.PayrollRun
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.PayrollRunID != 72 {
		t.Fatalf("unexpected run response: %#v", body)
	}
}

func TestPayrollHandlerApproveRunUsesPathIDAndActor(t *testing.T) {
	service := &fakePayrollService{}
	handler := NewPayrollHandler(service)
	jwtKey := "test-secret-key-32-characters-long"
	token, err := auth.GenerateToken(7, model.RoleSuperAdmin, jwtKey)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	req := payrollRequest("POST", "/payroll/runs/72/approve", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.SetPathValue("id", "72")
	w := httptest.NewRecorder()

	middleware.AuthMiddleware(http.HandlerFunc(handler.ApprovePayrollRun), jwtKey).ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d body=%s", w.Code, w.Body.String())
	}
	if service.approvedRunID != 72 || service.approvedActorID != 7 {
		t.Fatalf("unexpected approve call: run=%d actor=%d", service.approvedRunID, service.approvedActorID)
	}
}

func TestPayrollHandlerVoidRunParsesReasonAndActor(t *testing.T) {
	service := &fakePayrollService{}
	handler := NewPayrollHandler(service)
	jwtKey := "test-secret-key-32-characters-long"
	token, err := auth.GenerateToken(7, model.RoleSuperAdmin, jwtKey)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	req := payrollRequest("POST", "/payroll/runs/72/void", []byte(`{"reason":" duplicate "}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.SetPathValue("id", "72")
	w := httptest.NewRecorder()

	middleware.AuthMiddleware(http.HandlerFunc(handler.VoidPayrollRun), jwtKey).ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d body=%s", w.Code, w.Body.String())
	}
	if service.voidedRunID != 72 || service.voidedRunActorID != 7 || service.voidedRunReason != "duplicate" {
		t.Fatalf("unexpected void call: run=%d actor=%d reason=%q", service.voidedRunID, service.voidedRunActorID, service.voidedRunReason)
	}
}

func TestPayrollHandlerMarkRowPaidParsesPayloadAndActor(t *testing.T) {
	service := &fakePayrollService{paidRow: &model.PayrollRow{PayrollRunID: 72, PayrollRowID: 81, Status: model.PayrollRowStatusPaid}}
	handler := NewPayrollHandler(service)
	jwtKey := "test-secret-key-32-characters-long"
	token, err := auth.GenerateToken(7, model.RoleSuperAdmin, jwtKey)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	req := payrollRequest("POST", "/payroll/runs/72/rows/81/mark-paid", []byte(`{"payment_method":"cash","payment_reference":" CASH-1 ","payment_notes":" done "}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.SetPathValue("id", "72")
	req.SetPathValue("rowID", "81")
	w := httptest.NewRecorder()

	middleware.AuthMiddleware(http.HandlerFunc(handler.MarkPayrollRowPaid), jwtKey).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if service.paidRunID != 72 || service.paidRowID != 81 || service.paidActorID != 7 {
		t.Fatalf("unexpected paid target: run=%d row=%d actor=%d", service.paidRunID, service.paidRowID, service.paidActorID)
	}
	if service.paidRequest.PaymentMethod != model.PayrollPaymentMethodCash || service.paidRequest.PaymentReference != "CASH-1" || service.paidRequest.PaymentNotes != "done" {
		t.Fatalf("unexpected paid request: %#v", service.paidRequest)
	}
}

func TestPayrollHandlerMarkRowPaidMapsInvalidPaymentMethod(t *testing.T) {
	service := &fakePayrollService{err: model.ErrInvalidPayrollPaymentMethod}
	handler := NewPayrollHandler(service)
	jwtKey := "test-secret-key-32-characters-long"
	token, err := auth.GenerateToken(7, model.RoleSuperAdmin, jwtKey)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	req := payrollRequest("POST", "/payroll/runs/72/rows/81/mark-paid", []byte(`{"payment_method":"cheque"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.SetPathValue("id", "72")
	req.SetPathValue("rowID", "81")
	w := httptest.NewRecorder()

	middleware.AuthMiddleware(http.HandlerFunc(handler.MarkPayrollRowPaid), jwtKey).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestPayrollHandlerStaleRunReturnsReasons(t *testing.T) {
	service := &fakePayrollService{stale: true, staleReasons: []string{"attendance_source_updated"}}
	handler := NewPayrollHandler(service)
	req := payrollRequest("GET", "/payroll/runs/72/staleness", nil)
	req.SetPathValue("id", "72")
	w := httptest.NewRecorder()

	handler.CheckPayrollRunStaleness(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if service.staleRunID != 72 {
		t.Fatalf("expected staleness run 72, got %d", service.staleRunID)
	}
	var body struct {
		Stale   bool     `json:"stale"`
		Reasons []string `json:"reasons"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if !body.Stale || len(body.Reasons) != 1 || body.Reasons[0] != "attendance_source_updated" {
		t.Fatalf("unexpected staleness body: %#v", body)
	}
}

func TestPayrollHandlerExportsWorkbookWithXLSXContentType(t *testing.T) {
	service := &fakePayrollService{workbookBytes: []byte("xlsx bytes")}
	handler := NewPayrollHandler(service)
	req := payrollRequest("GET", "/payroll/runs/72/export.xlsx", nil)
	req.SetPathValue("id", "72")
	w := httptest.NewRecorder()

	handler.ExportPayrollWorkbook(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if service.workbookRunID != 72 {
		t.Fatalf("expected workbook run 72, got %d", service.workbookRunID)
	}
	if got := w.Header().Get("Content-Type"); got != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
		t.Fatalf("unexpected content type %q", got)
	}
	if got := w.Header().Get("Content-Disposition"); !strings.Contains(got, `filename="payroll-run-72.xlsx"`) {
		t.Fatalf("unexpected content disposition %q", got)
	}
	if w.Body.String() != "xlsx bytes" {
		t.Fatalf("unexpected workbook body %q", w.Body.String())
	}
}

func TestPayrollHandlerExportsPDFWithPDFContentType(t *testing.T) {
	service := &fakePayrollService{pdfBytes: []byte("%PDF bytes")}
	handler := NewPayrollHandler(service)
	req := payrollRequest("GET", "/payroll/runs/72/export.pdf", nil)
	req.SetPathValue("id", "72")
	w := httptest.NewRecorder()

	handler.ExportPayrollPayslipPDF(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if service.pdfRunID != 72 || service.pdfDraft {
		t.Fatalf("expected PDF run 72 without forced draft watermark, got run=%d draft=%v", service.pdfRunID, service.pdfDraft)
	}
	if got := w.Header().Get("Content-Type"); got != "application/pdf" {
		t.Fatalf("unexpected content type %q", got)
	}
	if got := w.Header().Get("Content-Disposition"); !strings.Contains(got, `filename="payroll-run-72-payslips.pdf"`) {
		t.Fatalf("unexpected content disposition %q", got)
	}
	if w.Body.String() != "%PDF bytes" {
		t.Fatalf("unexpected PDF body %q", w.Body.String())
	}
}

func payrollRequest(method string, target string, body []byte) *http.Request {
	return httptest.NewRequest(method, target, bytes.NewReader(body))
}
