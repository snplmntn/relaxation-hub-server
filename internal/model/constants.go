package model

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
	RoleClient    = "client"
	RoleTherapist = "therapist"
	RoleRider     = "rider"
	RoleAdmin     = "admin"
)

// Payment Method Constants
const (
	PaymentMethodCash  = "cash"
	PaymentMethodGCash = "gcash"
	PaymentMethodMaya  = "maya"
	PaymentMethodCard  = "card"
	PaymentMethodBDO   = "bdo"
)

// Event Type Constants
const (
	EventTypeAssigned        = "assigned"
	EventTypeUnassigned      = "unassigned"
	EventTypeSessionPaused   = "session_paused"
	EventTypeSessionResumed  = "session_resumed"
	EventTypeSessionExtended = "session_extended"
)

// Cancellation Reason Constants
const (
	CancellationReasonClientRequest        = "client_request"
	CancellationReasonNoShow               = "no_show"
	CancellationReasonTherapistUnavailable = "therapist_unavailable"
)
