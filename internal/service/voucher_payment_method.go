package service

import (
	"strings"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

// ErrVoucherNotAllowedOnline is returned when a voucher code is attached to a
// booking paid through PayMongo.
//
// Online payments are always settled at the un-vouchered price: the processing
// fee is absorbed rather than passed to the customer, so the margin does not
// also give up a stacked promotion. The VIP discount is deliberately NOT part
// of this rule — it is a standing customer benefit rather than a campaign, and
// it applies to online bookings exactly as it does to cash ones.
//
// This lives apart from the booking services because every path that can attach
// a voucher must go through it: customer single booking, customer group
// booking, the group voucher preview, and a staff member editing a booking in
// the admin. Guarding only the customer-facing path would leave the rule
// enforceable by the UI alone.
func validateVoucherPaymentMethod(paymentMethod, voucherCode string) error {
	if strings.TrimSpace(voucherCode) == "" {
		return nil
	}
	if strings.TrimSpace(strings.ToLower(paymentMethod)) != model.PaymentMethodOnline {
		return nil
	}
	return NewValidationError(
		"voucher_not_allowed_online",
		"vouchers cannot be used with online payment",
		map[string]string{"voucher_code": "not valid when payment_method is online"},
	)
}
