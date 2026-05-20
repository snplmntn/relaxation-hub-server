package service

import (
	"context"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

const vipVoucherRequiredMessage = "Client must be VIP to use voucher codes"

type voucherUserStore interface {
	FindUserByID(ctx context.Context, userID int) (*model.User, error)
}

func requireVIPForVoucher(ctx context.Context, userRepo voucherUserStore, clientID int64) error {
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
	if !user.IsVIP {
		return NewValidationError("vip_required", vipVoucherRequiredMessage, map[string]string{"voucher_code": "client must be VIP"})
	}
	return nil
}
