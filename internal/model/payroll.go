package model

import "time"

type PayrollMoneyCents int64

type StaffCompensationRate struct {
	RateID             int64      `json:"rate_id"`
	UserID             int64      `json:"user_id"`
	Role               string     `json:"role"`
	DailyRateCents     int64      `json:"daily_rate_cents"`
	OvertimeMultiplier float64    `json:"overtime_multiplier"`
	EffectiveFrom      time.Time  `json:"-"`
	EffectiveFromDate  string     `json:"effective_from"`
	EffectiveTo        *time.Time `json:"-"`
	EffectiveToDate    *string    `json:"effective_to,omitempty"`
	Notes              string     `json:"notes,omitempty"`
	CreatedBy          *int64     `json:"created_by,omitempty"`
	UpdatedBy          *int64     `json:"updated_by,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type StaffPayrollAdjustment struct {
	AdjustmentID      int64                 `json:"adjustment_id"`
	UserID            int64                 `json:"user_id"`
	FullName          string                `json:"full_name,omitempty"`
	Role              string                `json:"role"`
	AdjustmentDate    time.Time             `json:"-"`
	Date              string                `json:"adjustment_date"`
	PeriodStart       time.Time             `json:"-"`
	PeriodStartDate   string                `json:"period_start"`
	PeriodEnd         time.Time             `json:"-"`
	PeriodEndDate     string                `json:"period_end"`
	Type              PayrollAdjustmentType `json:"type"`
	Category          string                `json:"category"`
	AmountCents       int64                 `json:"amount_cents"`
	Reason            string                `json:"reason"`
	CashMovementCents int64                 `json:"cash_movement_cents"`
	CreatedAt         time.Time             `json:"created_at"`
	UpdatedAt         time.Time             `json:"updated_at"`
}

type PayrollRun struct {
	PayrollRunID int64        `json:"payroll_run_id"`
	PeriodStart  time.Time    `json:"-"`
	StartDate    string       `json:"period_start"`
	PeriodEnd    time.Time    `json:"-"`
	EndDate      string       `json:"period_end"`
	Status       string       `json:"status"`
	GeneratedBy  *int64       `json:"generated_by,omitempty"`
	GeneratedAt  time.Time    `json:"generated_at"`
	ApprovedBy   *int64       `json:"approved_by,omitempty"`
	ApprovedAt   *time.Time   `json:"approved_at,omitempty"`
	VoidedBy     *int64       `json:"voided_by,omitempty"`
	VoidedAt     *time.Time   `json:"voided_at,omitempty"`
	VoidedReason string       `json:"voided_reason,omitempty"`
	Rows         []PayrollRow `json:"rows,omitempty"`
}

type PayrollRow struct {
	PayrollRowID               int64     `json:"payroll_row_id"`
	PayrollRunID               int64     `json:"payroll_run_id"`
	UserID                     int64     `json:"user_id"`
	Role                       string    `json:"role"`
	FullNameSnapshot           string    `json:"full_name"`
	UsualBranchIDSnapshot      *int64    `json:"usual_branch_id,omitempty"`
	UsualLocationLabelSnapshot string    `json:"usual_location_label,omitempty"`
	Status                     string    `json:"status"`
	RegularMinutes             int       `json:"regular_minutes"`
	OvertimeMinutes            int       `json:"overtime_minutes"`
	DailyRateCents             *int64    `json:"daily_rate_cents,omitempty"`
	OvertimeMultiplier         *float64  `json:"overtime_multiplier,omitempty"`
	GrossCents                 int64     `json:"gross_cents"`
	AddAdjustmentsCents        int64     `json:"add_adjustments_cents"`
	MinusAdjustmentsCents      int64     `json:"minus_adjustments_cents"`
	FinalPayCents              int64     `json:"final_pay_cents"`
	BlockerCodes               []string  `json:"blocker_codes"`
	CreatedAt                  time.Time `json:"created_at"`
	UpdatedAt                  time.Time `json:"updated_at"`
}

type PayrollGenerationFilter struct {
	PeriodStart time.Time
	PeriodEnd   time.Time
	GeneratedBy int64
}

type PayrollPaymentRequest struct {
	PaymentMethod    string `json:"payment_method"`
	PaymentReference string `json:"payment_reference"`
	PaymentNotes     string `json:"payment_notes"`
}
