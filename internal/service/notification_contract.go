package service

import "strings"

const notificationSchemaVersion = "2"

func normalizeNotificationType(raw string) string {
	switch strings.TrimSpace(raw) {
	case "":
		return "system.info"
	case "chat_message":
		return "chat.message.new"
	case "booking_status":
		return "booking.status_updated"
	case "booking_assigned":
		return "booking.assigned"
	case "booking_updated":
		return "booking.updated"
	case "booking_offer", "booking_offered":
		return "booking.offer.created"
	case "ride_offer":
		return "ride.offer.created"
	case "ride_status":
		return "ride.status_updated"
	case "ride_completed":
		return "ride.completed"
	case "new_rating":
		return "review.new"
	case "therapist_cancelled":
		return "booking.therapist_cancelled"
	case "therapist_unassignment_warning":
		return "therapist.unassignment_warning"
	case "therapist_suspended":
		return "therapist.suspended"
	case "account_suspended":
		return "account.suspended"
	case "system_ban":
		return "system.ban"
	default:
		return strings.TrimSpace(raw)
	}
}
