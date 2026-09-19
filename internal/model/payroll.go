package model

import "time"

type PayrollMoneyCents int64
type PayrollRole string
type PayrollRunStatus string
type PayrollRowStatus string
type PayrollPaymentMethod string

const (
	PayrollPaymentMethodCash         PayrollPaymentMethod = "cash"
	PayrollPaymentMethodGCash        PayrollPaymentMethod = "gcash"
	PayrollPaymentMethodBankTransfer PayrollPaymentMethod = "bank_transfer"
	PayrollPaymentMethodOther        PayrollPaymentMethod = "other"
)

type StaffCompensationRate struct {
	RateID             int64             `json:"rate_id"`
	UserID             int64             `json:"user_id"`
	Role               PayrollRole       `json:"role"`
	DailyRateCents     PayrollMoneyCents `json:"daily_rate_cents"`
	OvertimeMultiplier float64           `json:"overtime_multiplier"`
	EffectiveFrom      time.Time         `json:"-"`
	EffectiveFromDate  string            `json:"effective_from"`
	EffectiveTo        *time.Time        `json:"-"`
	EffectiveToDate    *string           `json:"effective_to,omitempty"`
	Notes              string            `json:"notes,omitempty"`
	CreatedBy          *int64            `json:"created_by,omitempty"`
	UpdatedBy          *int64            `json:"updated_by,omitempty"`
	CreatedAt          time.Time         `json:"created_at"`
	UpdatedAt          time.Time         `json:"updated_at"`
}

type StaffPayrollAdjustment struct {
	AdjustmentID      int64                 `json:"adjustment_id"`
	UserID            int64                 `json:"user_id"`
	FullName          string                `json:"full_name,omitempty"`
	Role              PayrollRole           `json:"role"`
	AdjustmentDate    time.Time             `json:"-"`
	Date              string                `json:"adjustment_date"`
	PeriodStart       time.Time             `json:"-"`
	PeriodStartDate   string                `json:"period_start"`
	PeriodEnd         time.Time             `json:"-"`
	PeriodEndDate     string                `json:"period_end"`
	Type              PayrollAdjustmentType `json:"type"`
	Category          string                `json:"category"`
	AmountCents       PayrollMoneyCents     `json:"amount_cents"`
	Reason            string                `json:"reason"`
	CashMovementCents PayrollMoneyCents     `json:"cash_movement_cents"`
	CreatedBy         *int64                `json:"created_by,omitempty"`
	UpdatedBy         *int64                `json:"updated_by,omitempty"`
	VoidedBy          *int64                `json:"voided_by,omitempty"`
	VoidedAt          *time.Time            `json:"voided_at,omitempty"`
	CreatedAt         time.Time             `json:"created_at"`
	UpdatedAt         time.Time             `json:"updated_at"`
}

type PayrollRun struct {
	PayrollRunID int64            `json:"payroll_run_id"`
	PeriodStart  time.Time        `json:"-"`
	StartDate    string           `json:"period_start"`
	PeriodEnd    time.Time        `json:"-"`
	EndDate      string           `json:"period_end"`
	Status       PayrollRunStatus `json:"status"`
	GeneratedBy  *int64           `json:"generated_by,omitempty"`
	GeneratedAt  time.Time        `json:"generated_at"`
	ApprovedBy   *int64           `json:"approved_by,omitempty"`
	ApprovedAt   *time.Time       `json:"approved_at,omitempty"`
	VoidedBy     *int64           `json:"voided_by,omitempty"`
	VoidedAt     *time.Time       `json:"voided_at,omitempty"`
	VoidedReason string           `json:"voided_reason,omitempty"`
	Rows         []PayrollRow     `json:"rows,omitempty"`
}

type PayrollRow struct {
	PayrollRowID               int64                     `json:"payroll_row_id"`
	PayrollRunID               int64                     `json:"payroll_run_id"`
	UserID                     int64                     `json:"user_id"`
	Role                       PayrollRole               `json:"role"`
	FullNameSnapshot           string                    `json:"full_name"`
	UsualBranchIDSnapshot      *int64                    `json:"usual_branch_id,omitempty"`
	UsualLocationLabelSnapshot string                    `json:"usual_location_label,omitempty"`
	Status                     PayrollRowStatus          `json:"status"`
	RegularMinutes             int                       `json:"regular_minutes"`
	OvertimeMinutes            int                       `json:"overtime_minutes"`
	DailyRateCents             *PayrollMoneyCents        `json:"daily_rate_cents,omitempty"`
	OvertimeMultiplier         *float64                  `json:"overtime_multiplier,omitempty"`
	GrossCents                 PayrollMoneyCents         `json:"gross_cents"`
	AddAdjustmentsCents        PayrollMoneyCents         `json:"add_adjustments_cents"`
	MinusAdjustmentsCents      PayrollMoneyCents         `json:"minus_adjustments_cents"`
	FinalPayCents              PayrollMoneyCents         `json:"final_pay_cents"`
	BlockerCodes               []string                  `json:"blocker_codes"`
	PaidAt                     *time.Time                `json:"paid_at,omitempty"`
	PaidBy                     *int64                    `json:"paid_by,omitempty"`
	PaymentMethod              PayrollPaymentMethod      `json:"payment_method,omitempty"`
	PaymentReference           string                    `json:"payment_reference,omitempty"`
	PaymentNotes               string                    `json:"payment_notes,omitempty"`
	LedgerEntryID              *int64                    `json:"ledger_entry_id,omitempty"`
	CreatedAt                  time.Time                 `json:"created_at"`
	UpdatedAt                  time.Time                 `json:"updated_at"`
	AttendanceDetails          []PayrollAttendanceDetail `json:"attendance_details,omitempty"`
	BookingDetails             []PayrollBookingDetail    `json:"booking_details,omitempty"`
	AdjustmentDetails          []PayrollAdjustmentDetail `json:"adjustment_details,omitempty"`
}

type PayrollAttendanceDetail struct {
	DetailID           int64              `json:"detail_id"`
	PayrollRowID       int64              `json:"payroll_row_id"`
	AttendanceID       int64              `json:"attendance_id"`
	WorkDate           time.Time          `json:"-"`
	Date               string             `json:"work_date"`
	TimeInAt           *time.Time         `json:"time_in_at,omitempty"`
	TimeOutAt          *time.Time         `json:"time_out_at,omitempty"`
	WorkedMinutes      int                `json:"worked_minutes"`
	RegularMinutes     int                `json:"regular_minutes"`
	OvertimeMinutes    int                `json:"overtime_minutes"`
	DailyRateCents     *PayrollMoneyCents `json:"daily_rate_cents,omitempty"`
	OvertimeMultiplier *float64           `json:"overtime_multiplier,omitempty"`
	GrossCents         PayrollMoneyCents  `json:"gross_cents"`
	SourceUpdatedAt    time.Time          `json:"source_updated_at"`
	CreatedAt          time.Time          `json:"created_at"`
}

type PayrollBookingDetail struct {
	DetailID               int64             `json:"detail_id"`
	PayrollRowID           int64             `json:"payroll_row_id"`
	BookingID              int64             `json:"booking_id"`
	BusinessDate           time.Time         `json:"-"`
	Date                   string            `json:"business_date"`
	ServiceName            string            `json:"service_name"`
	DurationMinutes        int               `json:"duration_minutes"`
	FinalTotalCents        PayrollMoneyCents `json:"final_total_cents"`
	TherapistEarningsCents PayrollMoneyCents `json:"therapist_earnings_cents"`
	SourceUpdatedAt        time.Time         `json:"source_updated_at"`
	CreatedAt              time.Time         `json:"created_at"`
}

type PayrollAdjustmentDetail struct {
	DetailID        int64                 `json:"detail_id"`
	PayrollRowID    int64                 `json:"payroll_row_id"`
	AdjustmentID    int64                 `json:"adjustment_id"`
	AdjustmentDate  time.Time             `json:"-"`
	Date            string                `json:"adjustment_date"`
	Type            PayrollAdjustmentType `json:"type"`
	Category        string                `json:"category"`
	AmountCents     PayrollMoneyCents     `json:"amount_cents"`
	Reason          string                `json:"reason"`
	SourceUpdatedAt time.Time             `json:"source_updated_at"`
	CreatedAt       time.Time             `json:"created_at"`
}

type PayrollGenerationFilter struct {
	PeriodStart time.Time
	PeriodEnd   time.Time
	GeneratedBy int64
}

type PayrollPaymentRequest struct {
	PaymentMethod    PayrollPaymentMethod `json:"payment_method"`
	PaymentReference string               `json:"payment_reference"`
	PaymentNotes     string               `json:"payment_notes"`
}
