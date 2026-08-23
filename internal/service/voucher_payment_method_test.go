package service

import (
	"testing"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

func TestValidateVoucherPaymentMethod(t *testing.T) {
	t.Run("rejects a voucher on an online booking", func(t *testing.T) {
		err := validateVoucherPaymentMethod(model.PaymentMethodOnline, "WELCOME200")
		if err == nil {
			t.Fatal("expected a voucher on an online booking to be rejected")
		}
		ve, ok := err.(*ValidationError)
		if !ok {
			t.Fatalf("expected a ValidationError, got %T", err)
		}
		if ve.Code != "voucher_not_allowed_online" {
			t.Errorf("code = %q, want voucher_not_allowed_online", ve.Code)
		}
	})

	t.Run("allows a voucher on every other method", func(t *testing.T) {
		for _, pm := range []string{
			model.PaymentMethodCash,
			model.PaymentMethodGCash, // manual transfer keeps its vouchers
			model.PaymentMethodBDO,
			model.PaymentMethodMaya,
			model.PaymentMethodCard,
			"",
		} {
			if err := validateVoucherPaymentMethod(pm, "WELCOME200"); err != nil {
				t.Errorf("payment method %q rejected a voucher: %v", pm, err)
			}
		}
	})

	t.Run("allows an online booking with no voucher", func(t *testing.T) {
		for _, code := range []string{"", "   "} {
			if err := validateVoucherPaymentMethod(model.PaymentMethodOnline, code); err != nil {
				t.Errorf("online booking with blank code %q rejected: %v", code, err)
			}
		}
	})

	t.Run("is not fooled by casing or padding", func(t *testing.T) {
		for _, pm := range []string{"ONLINE", " Online ", "OnLiNe"} {
			if err := validateVoucherPaymentMethod(pm, "WELCOME200"); err == nil {
				t.Errorf("payment method %q slipped a voucher through", pm)
			}
		}
	})
}

func TestIsValidPaymentMethodAcceptsOnline(t *testing.T) {
	if !model.IsValidPaymentMethod(model.PaymentMethodOnline) {
		t.Fatal("online must be an accepted payment method")
	}
	if model.IsValidPaymentMethod("paymongo") {
		t.Fatal("paymongo is a gateway, not a payment method")
	}
}
