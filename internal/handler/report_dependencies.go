package handler

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

type ReportDependency string

const (
	reportDependencyLedgerRepo          ReportDependency = "ledgerRepo"
	reportDependencyBookingReferralRepo ReportDependency = "bookingReferralRepo"
	reportDependencyRiderWalletService  ReportDependency = "riderWalletService"
	reportDependencyStorageService      ReportDependency = "storageService"
	reportDependencyReportExportService ReportDependency = "reportExportService"
)

type reportOperation string

const (
	reportOperationGetLedgerSummary           reportOperation = "GetLedgerSummary"
	reportOperationGetLedgerTrend             reportOperation = "GetLedgerTrend"
	reportOperationGetReferralSummary         reportOperation = "GetReferralSummary"
	reportOperationListExpenses               reportOperation = "ListExpenses"
	reportOperationCreateExpense              reportOperation = "CreateExpense"
	reportOperationDeleteExpense              reportOperation = "DeleteExpense"
	reportOperationUploadExpenseReceipt       reportOperation = "UploadExpenseReceipt"
	reportOperationListPayoutBalances         reportOperation = "ListPayoutBalances"
	reportOperationRecordSettlement           reportOperation = "RecordSettlement"
	reportOperationListLedgerEntries          reportOperation = "ListLedgerEntries"
	reportOperationListRiderPayoutRequests    reportOperation = "ListRiderPayoutRequests"
	reportOperationResolveRiderPayoutRequest  reportOperation = "ResolveRiderPayoutRequest"
	reportOperationGetDailySalesReport        reportOperation = "GetDailySalesReport"
	reportOperationUpsertDailySalesRemittance reportOperation = "UpsertDailySalesRemittance"
	reportOperationExportDailySalesReport     reportOperation = "ExportDailySalesReport"
	reportOperationGetBookingExportReport     reportOperation = "GetBookingExportReport"
	reportOperationExportBookingReport        reportOperation = "ExportBookingReport"
	reportOperationListPayrollAdjustments     reportOperation = "ListPayrollAdjustments"
	reportOperationCreatePayrollAdjustment    reportOperation = "CreatePayrollAdjustment"
	reportOperationUpdatePayrollAdjustment    reportOperation = "UpdatePayrollAdjustment"
	reportOperationDeletePayrollAdjustment    reportOperation = "DeletePayrollAdjustment"
	reportOperationExportTherapistSalaries    reportOperation = "ExportTherapistSalaries"
)

type ReportDependencyState struct {
	Available bool      `json:"available"`
	Message   string    `json:"message,omitempty"`
	CheckedAt time.Time `json:"checked_at"`
}

type ReportDependencySnapshot struct {
	Status       string                           `json:"status"`
	Degraded     bool                             `json:"degraded"`
	Dependencies map[string]ReportDependencyState `json:"dependencies"`
}

type reportDependencyCheckFunc func(context.Context) ReportDependencyState

type ReportDependencyStatusProvider struct {
	mu                  sync.Mutex
	checks              map[ReportDependency]reportDependencyCheckFunc
	databaseHealthCheck reportDependencyHealthCheckFunc
	states              map[ReportDependency]ReportDependencyState
	lastAlertAt         map[ReportDependency]time.Time
	alertThrottle       time.Duration
}

type reportDependencyHealthCheckFunc func(context.Context) error

type reportDependencyRuntimeHealthChecker interface {
	HealthCheck(context.Context) error
}

type reportDependencyUnavailableEnvelope struct {
	Error reportDependencyUnavailablePayload `json:"error"`
}

type reportDependencyUnavailablePayload struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	Dependency string `json:"dependency"`
	Retryable  bool   `json:"retryable"`
}

var reportDependencyDescriptions = map[ReportDependency]string{
	reportDependencyLedgerRepo:          "ledgerRepo is not configured",
	reportDependencyBookingReferralRepo: "bookingReferralRepo is not configured",
	reportDependencyRiderWalletService:  "riderWalletService is not configured",
	reportDependencyStorageService:      "storageService is not configured",
	reportDependencyReportExportService: "reportExportService is not configured",
}

var orderedReportDependencies = []ReportDependency{
	reportDependencyLedgerRepo,
	reportDependencyBookingReferralRepo,
	reportDependencyRiderWalletService,
	reportDependencyStorageService,
	reportDependencyReportExportService,
}

var reportDependencyMatrix = map[reportOperation][]ReportDependency{
	reportOperationGetLedgerSummary:           {reportDependencyLedgerRepo},
	reportOperationGetLedgerTrend:             {reportDependencyLedgerRepo},
	reportOperationGetReferralSummary:         {reportDependencyBookingReferralRepo},
	reportOperationListExpenses:               {reportDependencyLedgerRepo},
	reportOperationCreateExpense:              {reportDependencyLedgerRepo},
	reportOperationDeleteExpense:              {reportDependencyLedgerRepo},
	reportOperationUploadExpenseReceipt:       {reportDependencyStorageService},
	reportOperationListPayoutBalances:         {reportDependencyLedgerRepo},
	reportOperationRecordSettlement:           {reportDependencyLedgerRepo},
	reportOperationListLedgerEntries:          {reportDependencyLedgerRepo},
	reportOperationListRiderPayoutRequests:    {reportDependencyRiderWalletService},
	reportOperationResolveRiderPayoutRequest:  {reportDependencyRiderWalletService},
	reportOperationGetDailySalesReport:        {reportDependencyReportExportService},
	reportOperationUpsertDailySalesRemittance: {reportDependencyReportExportService},
	reportOperationExportDailySalesReport:     {reportDependencyReportExportService},
	reportOperationGetBookingExportReport:     {reportDependencyReportExportService},
	reportOperationExportBookingReport:        {reportDependencyReportExportService},
	reportOperationListPayrollAdjustments:     {reportDependencyReportExportService},
	reportOperationCreatePayrollAdjustment:    {reportDependencyReportExportService},
	reportOperationUpdatePayrollAdjustment:    {reportDependencyReportExportService},
	reportOperationDeletePayrollAdjustment:    {reportDependencyReportExportService},
	reportOperationExportTherapistSalaries:    {reportDependencyReportExportService},
}

func NewReportDependencyStatusProvider(h *ReportHandler, databaseHealthCheck reportDependencyHealthCheckFunc) *ReportDependencyStatusProvider {
	return &ReportDependencyStatusProvider{
		checks: map[ReportDependency]reportDependencyCheckFunc{
			reportDependencyLedgerRepo:          h.newDatabaseBackedDependencyCheck(reportDependencyLedgerRepo),
			reportDependencyBookingReferralRepo: h.newDatabaseBackedDependencyCheck(reportDependencyBookingReferralRepo),
			reportDependencyRiderWalletService:  h.newDatabaseBackedDependencyCheck(reportDependencyRiderWalletService),
			reportDependencyReportExportService: h.newDatabaseBackedDependencyCheck(reportDependencyReportExportService),
			reportDependencyStorageService:      h.newStorageDependencyCheck(),
		},
		databaseHealthCheck: databaseHealthCheck,
		states:              make(map[ReportDependency]ReportDependencyState),
		lastAlertAt:         make(map[ReportDependency]time.Time),
		alertThrottle:       30 * time.Second,
	}
}

func (h *ReportHandler) newDatabaseBackedDependencyCheck(dep ReportDependency) reportDependencyCheckFunc {
	return func(ctx context.Context) ReportDependencyState {
		return h.evaluateReportDependency(dep)
	}
}

func isDatabaseBackedReportDependency(dep ReportDependency) bool {
	return dep != reportDependencyStorageService
}

func (h *ReportHandler) newStorageDependencyCheck() reportDependencyCheckFunc {
	return func(ctx context.Context) ReportDependencyState {
		state := h.evaluateReportDependency(reportDependencyStorageService)
		if !state.Available {
			return state
		}

		runtimeChecker, ok := h.storageService.(reportDependencyRuntimeHealthChecker)
		if !ok {
			return state
		}

		if err := runtimeChecker.HealthCheck(ctx); err != nil {
			state.Available = false
			state.Message = "storage unavailable: " + err.Error()
		}
		return state
	}
}

func reportDependenciesFor(op reportOperation) []ReportDependency {
	deps := reportDependencyMatrix[op]
	out := make([]ReportDependency, len(deps))
	copy(out, deps)
	return out
}

func (p *ReportDependencyStatusProvider) Snapshot(ctx context.Context) ReportDependencySnapshot {
	states := p.Check(ctx, orderedReportDependencies...)
	dependencies := make(map[string]ReportDependencyState, len(states))
	degraded := false
	for _, dep := range orderedReportDependencies {
		state := states[dep]
		if !state.Available {
			degraded = true
		}
		dependencies[string(dep)] = state
	}

	status := "ok"
	if degraded {
		status = "degraded"
	}

	return ReportDependencySnapshot{
		Status:       status,
		Degraded:     degraded,
		Dependencies: dependencies,
	}
}

func (p *ReportDependencyStatusProvider) Check(ctx context.Context, deps ...ReportDependency) map[ReportDependency]ReportDependencyState {
	p.mu.Lock()
	defer p.mu.Unlock()

	out := make(map[ReportDependency]ReportDependencyState, len(deps))
	var databaseErr error
	databaseChecked := false
	for _, dep := range deps {
		check := p.checks[dep]
		state := ReportDependencyState{
			Available: false,
			Message:   "no dependency check configured",
			CheckedAt: time.Now().UTC(),
		}
		if check != nil {
			state = check(ctx)
		}
		if state.Available && isDatabaseBackedReportDependency(dep) && p.databaseHealthCheck != nil {
			if !databaseChecked {
				databaseErr = p.databaseHealthCheck(ctx)
				databaseChecked = true
			}
			if databaseErr != nil {
				state.Available = false
				state.Message = "database unavailable: " + databaseErr.Error()
			}
		}
		if state.CheckedAt.IsZero() {
			state.CheckedAt = time.Now().UTC()
		}
		p.logDependencyTransition(dep, state)
		p.states[dep] = state
		out[dep] = state
	}

	return out
}

func (p *ReportDependencyStatusProvider) logDependencyTransition(dep ReportDependency, state ReportDependencyState) {
	now := time.Now().UTC()
	prev, ok := p.states[dep]
	if !ok {
		if !state.Available {
			slog.Warn("report dependency unavailable", "dependency", string(dep), "message", state.Message)
			p.lastAlertAt[dep] = now
		}
		return
	}

	if prev.Available == state.Available {
		return
	}

	if lastAlertAt, ok := p.lastAlertAt[dep]; ok && now.Sub(lastAlertAt) < p.alertThrottle {
		return
	}

	if state.Available {
		slog.Info("report dependency recovered", "dependency", string(dep), "message", state.Message)
	} else {
		slog.Warn("report dependency unavailable", "dependency", string(dep), "message", state.Message)
	}
	p.lastAlertAt[dep] = now
}

func (h *ReportHandler) SetDependencyStatusProvider(provider *ReportDependencyStatusProvider) {
	h.dependencyStatusProvider = provider
}

func (h *ReportHandler) requireReportDependencies(w http.ResponseWriter, r *http.Request, op reportOperation) bool {
	return h.requireSpecificReportDependencies(w, r, op, reportDependenciesFor(op)...)
}

func (h *ReportHandler) requireSpecificReportDependencies(w http.ResponseWriter, r *http.Request, op reportOperation, deps ...ReportDependency) bool {
	if len(deps) == 0 {
		return true
	}

	states := h.reportDependencyStates(r.Context(), deps)
	for _, dep := range deps {
		state := states[dep]
		if state.Available {
			continue
		}

		slog.Warn(
			"report dependency unavailable response",
			"dependency", string(dep),
			"endpoint", r.URL.Path,
			"operation", string(op),
			"status_code", http.StatusServiceUnavailable,
		)
		respondJSON(w, http.StatusServiceUnavailable, reportDependencyUnavailableEnvelope{
			Error: reportDependencyUnavailablePayload{
				Code:       "dependency_unavailable",
				Message:    state.Message,
				Dependency: string(dep),
				Retryable:  true,
			},
		})
		return false
	}

	return true
}

func (h *ReportHandler) reportDependencyStates(ctx context.Context, deps []ReportDependency) map[ReportDependency]ReportDependencyState {
	if h.dependencyStatusProvider != nil {
		return h.dependencyStatusProvider.Check(ctx, deps...)
	}

	out := make(map[ReportDependency]ReportDependencyState, len(deps))
	for _, dep := range deps {
		out[dep] = h.evaluateReportDependency(dep)
	}
	return out
}

func (h *ReportHandler) evaluateReportDependency(dep ReportDependency) ReportDependencyState {
	state := ReportDependencyState{
		Available: true,
		Message:   "available",
		CheckedAt: time.Now().UTC(),
	}

	switch dep {
	case reportDependencyLedgerRepo:
		if h.ledgerRepo == nil {
			state.Available = false
		}
	case reportDependencyBookingReferralRepo:
		if h.bookingReferralRepo == nil {
			state.Available = false
		}
	case reportDependencyRiderWalletService:
		if h.riderWalletService == nil {
			state.Available = false
		}
	case reportDependencyStorageService:
		if h.storageService == nil || !h.storageService.IsConfigured() {
			state.Available = false
		}
	case reportDependencyReportExportService:
		if h.reportExportService == nil {
			state.Available = false
		}
	default:
		state.Available = false
		state.Message = "unknown dependency"
		return state
	}

	if !state.Available {
		state.Message = reportDependencyDescriptions[dep]
	}
	return state
}
