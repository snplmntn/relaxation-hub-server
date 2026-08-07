package model

import "time"

const ExcelContentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"

type ReportWarningCounts struct {
	CompletedBookingsMissingActualEnd int `json:"completed_bookings_missing_actual_end"`
}

type BookingExportFilter struct {
	StartDate   time.Time
	EndDate     time.Time
	TherapistID *int64
}

type ReportBookingExportRow struct {
	BookingID              int64     `json:"booking_id"`
	BusinessDate           time.Time `json:"-"`
	Date                   string    `json:"business_date"`
	TherapistID            int64     `json:"therapist_id"`
	TherapistName          string    `json:"therapist_name"`
	ClientName             string    `json:"client_name"`
	ServiceName            string    `json:"service_name"`
	DurationMinutes        int       `json:"duration_minutes"`
	BookingDurationMinutes int       `json:"-"`
	ServiceDurationWeight  float64   `json:"-"`
	DurationAllocated      bool      `json:"-"`
	ServicePriceWeight     float64   `json:"-"`
	ServiceCommissionRate  float64   `json:"-"`
	AdditionalService      bool      `json:"-"`
	PaymentMethod          string    `json:"payment_method"`
	PaymentBucket          string    `json:"payment_bucket"`
	FinalTotal             float64   `json:"final_total"`
	TherapistEarnings      float64   `json:"therapist_earnings"`
}

type BookingExportSummary struct {
	TherapistID       int64   `json:"therapist_id,omitempty"`
	TherapistName     string  `json:"therapist_name,omitempty"`
	CashCollected     float64 `json:"cash_collected"`
	GCashSales        float64 `json:"gcash_sales"`
	SpaRemitSales     float64 `json:"spa_remit_sales"`
	OtherSales        float64 `json:"other_sales"`
	NonCashSales      float64 `json:"non_cash_sales"`
	TotalSales        float64 `json:"total_sales"`
	TherapistEarnings float64 `json:"therapist_earnings"`
	NetCashToRemit    float64 `json:"net_cash_to_remit"`
	TotalHours        float64 `json:"total_hours"`
	BookingCount      int     `json:"booking_count"`
}

type BookingExportDailySummary struct {
	Date string `json:"business_date"`
	BookingExportSummary
}

type BookingExportReport struct {
	StartDate  time.Time                   `json:"-"`
	EndDate    time.Time                   `json:"-"`
	Start      string                      `json:"start_date"`
	End        string                      `json:"end_date"`
	Therapists []BookingExportSummary      `json:"therapists"`
	Daily      []BookingExportDailySummary `json:"daily"`
	Totals     BookingExportSummary        `json:"totals"`
	Bookings   []ReportBookingExportRow    `json:"bookings"`
	Warnings   ReportWarningCounts         `json:"warnings"`
}

type ReportTherapistRosterRow struct {
	BranchID      int64  `json:"branch_id"`
	BranchName    string `json:"branch_name"`
	TherapistID   int64  `json:"therapist_id"`
	TherapistName string `json:"therapist_name"`
}

type ReportDailySalesBookingRow struct {
	BranchID      int64   `json:"branch_id"`
	BranchName    string  `json:"branch_name"`
	TherapistID   int64   `json:"therapist_id"`
	TherapistName string  `json:"therapist_name"`
	PaymentMethod string  `json:"payment_method"`
	TotalSales    float64 `json:"total_sales"`
	TotalHours    float64 `json:"total_hours"`
	BookingCount  int     `json:"booking_count"`
}

type DailySalesTherapistRow struct {
	TherapistID   int64   `json:"therapist_id"`
	TherapistName string  `json:"therapist_name"`
	CashSales     float64 `json:"cash_sales"`
	GCashSales    float64 `json:"gcash_sales"`
	SpaRemitSales float64 `json:"spa_remit_sales"`
	OtherSales    float64 `json:"other_sales"`
	TotalSales    float64 `json:"total_sales"`
	TotalHours    float64 `json:"total_hours"`
	BookingCount  int     `json:"booking_count"`
}

type DailySalesBranchSection struct {
	BranchID   int64                    `json:"branch_id"`
	BranchName string                   `json:"branch_name"`
	Therapists []DailySalesTherapistRow `json:"therapists"`
	Totals     DailySalesTherapistRow   `json:"totals"`
	Remittance DailySalesRemittance     `json:"remittance"`
}

type DailySalesReport struct {
	BusinessDate time.Time                 `json:"-"`
	Date         string                    `json:"business_date"`
	Branches     []DailySalesBranchSection `json:"branches"`
	Warnings     ReportWarningCounts       `json:"warnings"`
}

type DailySalesRemittance struct {
	RemittanceID        int64     `json:"remittance_id"`
	BusinessDate        time.Time `json:"-"`
	Date                string    `json:"business_date"`
	BranchID            int64     `json:"branch_id"`
	Bill1000            int       `json:"bill_1000"`
	Bill500             int       `json:"bill_500"`
	Bill200             int       `json:"bill_200"`
	Bill100             int       `json:"bill_100"`
	Bill50              int       `json:"bill_50"`
	Bill20              int       `json:"bill_20"`
	Bill10              int       `json:"bill_10"`
	Bill5               int       `json:"bill_5"`
	Bill1               int       `json:"bill_1"`
	ActualRemitted      float64   `json:"actual_remitted"`
	TipsTotal           float64   `json:"tips_total"`
	ClientFundsUsed     float64   `json:"client_funds_used"`
	ClientFundsAdded    float64   `json:"client_funds_added"`
	RemittedToMark      float64   `json:"remitted_to_mark"`
	OtherRemittedAmount float64   `json:"other_remitted_amount"`
	RemittedTo          string    `json:"remitted_to"`
	OthersDeducted      float64   `json:"others_deducted"`
	OthersAdded         float64   `json:"others_added"`
	Notes               string    `json:"notes"`
	MustBeZero          float64   `json:"must_be_zero"`
	// GCash/Maya wallet balances physically on hand at closing. They are
	// reconciled against the wallet apps, not derived from sales.
	GCashOnHand float64 `json:"gcash_on_hand"`
	MayaOnHand  float64 `json:"maya_on_hand"`
	// VaultClaimed is ticked by the owner once the vault cash has physically
	// been taken; the _at/_by pair is the audit trail for that hand-off and is
	// maintained by the server, never by the client.
	VaultClaimed       bool       `json:"vault_claimed"`
	VaultClaimedAt     *time.Time `json:"vault_claimed_at"`
	VaultClaimedBy     *int64     `json:"vault_claimed_by"`
	VaultClaimedByName string     `json:"vault_claimed_by_name,omitempty"`
	// Second denomination count: the cash the closing staff took, tracked
	// separately from the bill_* counts of the cash that was remitted.
	ClosingBill1000 int       `json:"closing_bill_1000"`
	ClosingBill500  int       `json:"closing_bill_500"`
	ClosingBill200  int       `json:"closing_bill_200"`
	ClosingBill100  int       `json:"closing_bill_100"`
	ClosingBill50   int       `json:"closing_bill_50"`
	ClosingBill20   int       `json:"closing_bill_20"`
	ClosingBill10   int       `json:"closing_bill_10"`
	ClosingBill5    int       `json:"closing_bill_5"`
	ClosingBill1    int       `json:"closing_bill_1"`
	CreatedBy       *int64    `json:"created_by,omitempty"`
	UpdatedBy       *int64    `json:"updated_by,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type UpsertDailySalesRemittanceRequest struct {
	BusinessDate        string  `json:"business_date"`
	BranchID            int64   `json:"branch_id"`
	Bill1000            int     `json:"bill_1000"`
	Bill500             int     `json:"bill_500"`
	Bill200             int     `json:"bill_200"`
	Bill100             int     `json:"bill_100"`
	Bill50              int     `json:"bill_50"`
	Bill20              int     `json:"bill_20"`
	Bill10              int     `json:"bill_10"`
	Bill5               int     `json:"bill_5"`
	Bill1               int     `json:"bill_1"`
	ActualRemitted      float64 `json:"actual_remitted"`
	TipsTotal           float64 `json:"tips_total"`
	ClientFundsUsed     float64 `json:"client_funds_used"`
	ClientFundsAdded    float64 `json:"client_funds_added"`
	RemittedToMark      float64 `json:"remitted_to_mark"`
	OtherRemittedAmount float64 `json:"other_remitted_amount"`
	RemittedTo          string  `json:"remitted_to"`
	OthersDeducted      float64 `json:"others_deducted"`
	OthersAdded         float64 `json:"others_added"`
	Notes               string  `json:"notes"`
	GCashOnHand         float64 `json:"gcash_on_hand"`
	MayaOnHand          float64 `json:"maya_on_hand"`
	// VaultClaimed is the only vault field a client may set; vault_claimed_at
	// and vault_claimed_by are stamped by the server on the false -> true
	// transition and cleared on true -> false.
	VaultClaimed    bool `json:"vault_claimed"`
	ClosingBill1000 int  `json:"closing_bill_1000"`
	ClosingBill500  int  `json:"closing_bill_500"`
	ClosingBill200  int  `json:"closing_bill_200"`
	ClosingBill100  int  `json:"closing_bill_100"`
	ClosingBill50   int  `json:"closing_bill_50"`
	ClosingBill20   int  `json:"closing_bill_20"`
	ClosingBill10   int  `json:"closing_bill_10"`
	ClosingBill5    int  `json:"closing_bill_5"`
	ClosingBill1    int  `json:"closing_bill_1"`
}

type PayrollAdjustmentCategory string

const (
	PayrollAdjustmentCategoryBenefits    PayrollAdjustmentCategory = "benefits"
	PayrollAdjustmentCategoryCashAdvance PayrollAdjustmentCategory = "cash_advance"
	PayrollAdjustmentCategorySalary      PayrollAdjustmentCategory = "salary"
	PayrollAdjustmentCategoryCorrection  PayrollAdjustmentCategory = "correction"
	PayrollAdjustmentCategoryParcel      PayrollAdjustmentCategory = "parcel"
	PayrollAdjustmentCategoryAbsence     PayrollAdjustmentCategory = "absence"
	PayrollAdjustmentCategoryOther       PayrollAdjustmentCategory = "other"
)

type PayrollAdjustmentFilter struct {
	StartDate   time.Time
	EndDate     time.Time
	TherapistID *int64
}

type PayrollAdjustment struct {
	AdjustmentID    int64                     `json:"adjustment_id"`
	TherapistID     int64                     `json:"therapist_id"`
	TherapistName   string                    `json:"therapist_name,omitempty"`
	AdjustmentDate  time.Time                 `json:"-"`
	Date            string                    `json:"adjustment_date"`
	PeriodStart     time.Time                 `json:"-"`
	PeriodStartDate string                    `json:"period_start"`
	PeriodEnd       time.Time                 `json:"-"`
	PeriodEndDate   string                    `json:"period_end"`
	Type            PayrollAdjustmentType     `json:"type"`
	Category        PayrollAdjustmentCategory `json:"category"`
	Amount          float64                   `json:"amount"`
	Reason          string                    `json:"reason"`
	CashMovement    float64                   `json:"cash_movement"`
	CreatedBy       *int64                    `json:"created_by,omitempty"`
	UpdatedBy       *int64                    `json:"updated_by,omitempty"`
	VoidedBy        *int64                    `json:"voided_by,omitempty"`
	VoidedAt        *time.Time                `json:"voided_at,omitempty"`
	CreatedAt       time.Time                 `json:"created_at"`
	UpdatedAt       time.Time                 `json:"updated_at"`
}

type PayrollAdjustmentRequest struct {
	TherapistID    int64   `json:"therapist_id"`
	AdjustmentDate string  `json:"adjustment_date"`
	PeriodStart    string  `json:"period_start"`
	PeriodEnd      string  `json:"period_end"`
	Type           string  `json:"type"`
	Category       string  `json:"category"`
	Amount         float64 `json:"amount"`
	Reason         string  `json:"reason"`
	CashMovement   float64 `json:"cash_movement"`
}

type SalaryReportFilter struct {
	StartDate   time.Time
	EndDate     time.Time
	TherapistID *int64
}

type ReportSalaryBookingRow struct {
	TherapistID       int64     `json:"therapist_id"`
	TherapistName     string    `json:"therapist_name"`
	BusinessDate      time.Time `json:"-"`
	ServiceName       string    `json:"service_name"`
	BookingID         int64     `json:"booking_id"`
	DurationMinutes   int       `json:"duration_minutes"`
	FinalTotal        float64   `json:"final_total"`
	TherapistEarnings float64   `json:"therapist_earnings"`
}

type SalaryTherapistSummary struct {
	TherapistID      int64                    `json:"therapist_id"`
	TherapistName    string                   `json:"therapist_name"`
	Bookings         []ReportSalaryBookingRow `json:"bookings"`
	Adjustments      []PayrollAdjustment      `json:"adjustments"`
	TotalHours       float64                  `json:"total_hours"`
	GrossSales       float64                  `json:"gross_sales"`
	BookingEarnings  float64                  `json:"booking_earnings"`
	AddAdjustments   float64                  `json:"add_adjustments"`
	MinusAdjustments float64                  `json:"minus_adjustments"`
	FinalSalary      float64                  `json:"final_salary"`
}

type SalaryReport struct {
	StartDate  time.Time                `json:"-"`
	EndDate    time.Time                `json:"-"`
	Start      string                   `json:"start_date"`
	End        string                   `json:"end_date"`
	Therapists []SalaryTherapistSummary `json:"therapists"`
	Warnings   ReportWarningCounts      `json:"warnings"`
}
