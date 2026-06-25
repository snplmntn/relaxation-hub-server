package service

import (
	"context"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

type voucherUserStore interface {
	FindUserByID(ctx context.Context, userID int) (*model.User, error)
}

// validateVoucherClient ensures the supplied client id refers to an actual
// client account before a voucher code is applied. Voucher codes are available
// to all clients (not just VIPs); only the client-role check is enforced.
func validateVoucherClient(ctx context.Context, userRepo voucherUserStore, clientID int64) error {
	if userRepo == nil {
		return nil
	}

	user, err := userRepo.FindUserByID(ctx, int(clientID))
	if err != nil {
		return err
	}
	if user.Role != model.RoleClient {
		return NewValidationError("invalid_client", "selected user is not a client", map[string]string{"client_id": "not a client"})
	}
	return nil
}
