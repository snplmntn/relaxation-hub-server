package model

import "strings"

const (
	BookingSourceHirayaWeb = "hiraya_web"
	BookingSourceStaffWeb  = "staff_web"
	BookingSourceClientApp = "client_app"
	BookingSourceCustomer  = "customer"
)

func IsValidBookingSource(source string) bool {
	switch strings.TrimSpace(source) {
	case BookingSourceHirayaWeb, BookingSourceStaffWeb, BookingSourceClientApp, BookingSourceCustomer:
		return true
	default:
		return false
	}
}

// Booking Status Constants
const (
	BookingStatusPending    = "pending"
	BookingStatusAssigned   = "assigned"
	BookingStatusOnTheWay   = "on_the_way"
	BookingStatusArrived    = "arrived"
	BookingStatusInProgress = "in_progress"
	BookingStatusCompleted  = "completed"
	BookingStatusCancelled  = "cancelled"
	BookingStatusNoShow     = "no_show"
)

// User Role Constants
const (
	RoleClient     = "client"
	RoleTherapist  = "therapist"
	RoleRider      = "rider"
	RoleAdmin      = "admin"
	RoleSuperAdmin = "super_admin"
)

// Payroll roles
const (
	PayrollRoleTherapist = RoleTherapist
	PayrollRoleRider     = RoleRider
	PayrollRoleAdmin     = RoleAdmin
)

// Payroll run statuses
const (
	PayrollRunStatusDraft    = "draft"
	PayrollRunStatusApproved = "approved"
	PayrollRunStatusPaid     = "paid"
	PayrollRunStatusVoided   = "voided"
)

// Payroll row statuses
const (
	PayrollRowStatusDraft    = "draft"
	PayrollRowStatusApproved = "approved"
	PayrollRowStatusPaid     = "paid"
	PayrollRowStatusBlocked  = "blocked"
	PayrollRowStatusVoided   = "voided"
)

// Staff adjustment types
type PayrollAdjustmentType string

const (
	PayrollAdjustmentTypeAdd   PayrollAdjustmentType = "add"
	PayrollAdjustmentTypeMinus PayrollAdjustmentType = "minus"
)

// Payment Method Constants
const (
	PaymentMethodCash  = "cash"
	PaymentMethodGCash = "gcash"
	PaymentMethodMaya  = "maya"
	PaymentMethodCard  = "card"
	PaymentMethodBDO   = "bdo"
	// PaymentMethodOnline is paid through PayMongo before the booking exists.
	// Distinct from PaymentMethodGCash/PaymentMethodBDO, which mean the customer
	// transferred by hand and a staff member verifies the receipt.
	PaymentMethodOnline = "online"
)

// Event Type Constants
const (
	EventTypeAssigned        = "assigned"
	EventTypeUnassigned      = "unassigned"
	EventTypeSessionPaused   = "session_paused"
	EventTypeSessionResumed  = "session_resumed"
	EventTypeSessionExtended = "session_extended"

	EventTypeReturnRideDestinationUpdated = "return_ride_destination_updated"
	EventTypeReturnRideScheduleUpdated    = "return_ride_schedule_updated"
	EventTypeReturnRideActivated          = "return_ride_activated"
	EventTypeReturnRideActivationFailed   = "return_ride_activation_failed"
)

// Cancellation Reason Constants
const (
	CancellationReasonClientRequest        = "client_request"
	CancellationReasonNoShow               = "no_show"
	CancellationReasonTherapistUnavailable = "therapist_unavailable"
)

// Booking referral source constants
const (
	BookingReferralSourceFacebook        = "Facebook"
	BookingReferralSourceInstagram       = "Instagram"
	BookingReferralSourceTikTok          = "TikTok"
	BookingReferralSourceGoogleSearch    = "Google Search"
	BookingReferralSourceFriendFamily    = "Friend/Family"
	BookingReferralSourceReturningClient = "Returning Client"
	BookingReferralSourceWalkInFlyer     = "Walk-in/Flyer"
	BookingReferralSourcePartnerHotel    = "Partner/Hotel"
	BookingReferralSourcePhone           = "Phone"
	BookingReferralSourceViber           = "Viber"
	BookingReferralSourceWhatsApp        = "WhatsApp"
	BookingReferralSourceTelegram        = "Telegram"
	BookingReferralSourceOthers          = "Others"
)

var allowedBookingReferralSources = map[string]struct{}{
	BookingReferralSourceFacebook:        {},
	BookingReferralSourceInstagram:       {},
	BookingReferralSourceTikTok:          {},
	BookingReferralSourceGoogleSearch:    {},
	BookingReferralSourceFriendFamily:    {},
	BookingReferralSourceReturningClient: {},
	BookingReferralSourceWalkInFlyer:     {},
	BookingReferralSourcePartnerHotel:    {},
	BookingReferralSourcePhone:           {},
	BookingReferralSourceViber:           {},
	BookingReferralSourceWhatsApp:        {},
	BookingReferralSourceTelegram:        {},
	BookingReferralSourceOthers:          {},
}

func IsValidBookingReferralSource(source string) bool {
	_, ok := allowedBookingReferralSources[strings.TrimSpace(source)]
	return ok
}

func IsAdminRole(role string) bool {
	return role == RoleAdmin || role == RoleSuperAdmin
}

// IsValidPaymentMethod reports whether a (already lowercased, trimmed) payment
// method is one the system accepts.
func IsValidPaymentMethod(pm string) bool {
	switch pm {
	case PaymentMethodCash, PaymentMethodGCash, PaymentMethodMaya, PaymentMethodBDO, PaymentMethodCard, PaymentMethodOnline:
		return true
	}
	return false
}
