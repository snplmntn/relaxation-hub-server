package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

// WalletService handles wallet operations for therapists.
type WalletService struct {
	db          db.DBTX
	walletRepo  repository.WalletRepository
	bookingRepo repository.BookingRepository
}

// NewWalletService creates a new wallet service.
func NewWalletService(db db.DBTX, walletRepo repository.WalletRepository, bookingRepo repository.BookingRepository) *WalletService {
	return &WalletService{
		db:          db,
		walletRepo:  walletRepo,
		bookingRepo: bookingRepo,
	}
}

// GetWalletSummary returns the wallet with summary data for dashboard.
func (s *WalletService) GetWalletSummary(ctx context.Context, therapistID int64) (*model.WalletSummary, error) {
	wallet, err := s.walletRepo.GetByTherapistID(ctx, therapistID)
	if err != nil {
		slog.Error("failed to get wallet", "therapist_id", therapistID, "error", err)
		return nil, fmt.Errorf("wallet not found")
	}

	activeAdvance, _ := s.walletRepo.GetActiveAdvanceByTherapist(ctx, therapistID)

	txns, _, _ := s.walletRepo.ListTransactions(ctx, wallet.WalletID, 5, 0)

	payouts, _ := s.walletRepo.ListPayoutRequestsByTherapist(ctx, therapistID)
	pendingCount := 0
	for _, p := range payouts {
		if p.Status == "pending" {
			pendingCount++
		}
	}

	return &model.WalletSummary{
		Wallet:             wallet,
		ActiveAdvance:      activeAdvance,
		PendingPayouts:     pendingCount,
		RecentTransactions: txns,
	}, nil
}

// CreditEarning adds earnings to pending balance (called when booking completes).
// After 24h, a worker should call ReleaseEarning to move to available.
func (s *WalletService) CreditEarning(ctx context.Context, therapistID, bookingID int64, amount float64, ledgerEntryID *int64) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	txRepo := repository.NewWalletRepository(tx)

	wallet, err := txRepo.GetByTherapistID(ctx, therapistID)
	if err != nil {
		// Auto-create wallet if not exists
		wallet, err = txRepo.CreateWallet(ctx, therapistID)
		if err != nil {
			slog.Error("failed to create wallet", "therapist_id", therapistID, "error", err)
			return fmt.Errorf("failed to create wallet: %w", err)
		}
	}

	// Check for active cash advance and calculate repayment
	repayment := 0.0
	netEarning := amount
	advance, _ := txRepo.GetActiveAdvanceByTherapist(ctx, therapistID)
	if advance != nil && advance.RemainingBalance > 0 {
		repayment = amount * (advance.RepaymentRate / 100)
		if repayment > advance.RemainingBalance {
			repayment = advance.RemainingBalance
		}
		netEarning = amount - repayment
	}

	// Update pending balance
	if err := txRepo.UpdateBalances(ctx, wallet.WalletID, 0, netEarning); err != nil {
		slog.Error("failed to update pending balance", "wallet_id", wallet.WalletID, "error", err)
		return err
	}

	// Create earning transaction
	desc := fmt.Sprintf("Earnings from booking #%d", bookingID)
	bID := bookingID
	txn := &model.WalletTransaction{
		WalletID:      wallet.WalletID,
		BookingID:     &bID,
		LedgerEntryID: ledgerEntryID,
		Type:          "earning",
		Amount:        netEarning,
		BalanceAfter:  wallet.AvailableBalance,
		PendingAfter:  wallet.PendingBalance + netEarning,
		Description:   &desc,
	}
	if err := txRepo.CreateTransaction(ctx, txn); err != nil {
		slog.Error("failed to create earning transaction", "error", err)
		return err
	}

	// Handle advance repayment if applicable
	if repayment > 0 && advance != nil {
		if err := txRepo.UpdateAdvanceBalance(ctx, advance.AdvanceID, repayment); err != nil {
			slog.Error("failed to update advance balance", "error", err)
			return err
		}

		repayDesc := fmt.Sprintf("Advance repayment from booking #%d", bookingID)
		repayTxn := &model.WalletTransaction{
			WalletID:     wallet.WalletID,
			BookingID:    &bID,
			Type:         "advance_repayment",
			Amount:       -repayment,
			BalanceAfter: wallet.AvailableBalance,
			PendingAfter: wallet.PendingBalance + netEarning,
			Description:  &repayDesc,
		}
		if err := txRepo.CreateTransaction(ctx, repayTxn); err != nil {
			slog.Error("failed to create repayment transaction", "error", err)
			return err
		}

		// Check if advance is fully repaid
		if advance.RemainingBalance-repayment <= 0 {
			if err := txRepo.MarkAdvancePaidOff(ctx, advance.AdvanceID); err != nil {
				slog.Error("failed to mark advance paid off", "error", err)
				return err
			}
			slog.Info("cash advance paid off", "advance_id", advance.AdvanceID, "therapist_id", therapistID)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	slog.Info("credited earning to wallet",
		"therapist_id", therapistID,
		"booking_id", bookingID,
		"amount", amount,
		"net_earning", netEarning,
		"repayment", repayment,
	)
	return nil
}

// ReleaseEarning moves funds from pending to available (called by worker after 24h).
func (s *WalletService) ReleaseEarning(ctx context.Context, therapistID int64, amount float64) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	txRepo := repository.NewWalletRepository(tx)

	wallet, err := txRepo.GetByTherapistID(ctx, therapistID)
	if err != nil {
		return fmt.Errorf("wallet not found: %w", err)
	}

	if wallet.PendingBalance < amount {
		amount = wallet.PendingBalance // Release whatever is available
	}

	if amount <= 0 {
		return nil
	}

	// Move from pending to available
	if err := txRepo.UpdateBalances(ctx, wallet.WalletID, amount, -amount); err != nil {
		return err
	}

	desc := "Pending earnings released to available balance"
	txn := &model.WalletTransaction{
		WalletID:     wallet.WalletID,
		Type:         "earning_released",
		Amount:       amount,
		BalanceAfter: wallet.AvailableBalance + amount,
		PendingAfter: wallet.PendingBalance - amount,
		Description:  &desc,
	}
	if err := txRepo.CreateTransaction(ctx, txn); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	slog.Info("released pending earnings", "therapist_id", therapistID, "amount", amount)
	return nil
}

// RequestPayout creates a payout request from therapist.
func (s *WalletService) RequestPayout(ctx context.Context, therapistID int64, amount float64, method string, accountDetails []byte) (*model.PayoutRequest, error) {
	wallet, err := s.walletRepo.GetByTherapistID(ctx, therapistID)
	if err != nil {
		return nil, &ValidationError{Code: "therapist_id", Message: "Wallet not found"}
	}

	if amount < wallet.MinimumPayout {
		return nil, &ValidationError{Code: "amount", Message: fmt.Sprintf("Minimum payout is ₱%.2f", wallet.MinimumPayout)}
	}

	if amount > wallet.AvailableBalance {
		return nil, &ValidationError{Code: "amount", Message: fmt.Sprintf("Insufficient balance. Available: ₱%.2f", wallet.AvailableBalance)}
	}

	req := &model.PayoutRequest{
		WalletID:       wallet.WalletID,
		TherapistID:    therapistID,
		Amount:         amount,
		PayoutMethod:   method,
		AccountDetails: accountDetails,
	}

	if err := s.walletRepo.CreatePayoutRequest(ctx, req); err != nil {
		slog.Error("failed to create payout request", "therapist_id", therapistID, "error", err)
		return nil, fmt.Errorf("failed to create payout request")
	}

	slog.Info("payout request created", "request_id", req.RequestID, "therapist_id", therapistID, "amount", amount)
	return req, nil
}

// ApprovePayout approves and processes a payout request.
func (s *WalletService) ApprovePayout(ctx context.Context, requestID, adminID int64, txnRef *string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	txRepo := repository.NewWalletRepository(tx)

	req, err := txRepo.GetPayoutRequest(ctx, requestID)
	if err != nil {
		return fmt.Errorf("payout request not found")
	}

	if req.Status != "pending" {
		return &ValidationError{Code: "status", Message: "Request is not pending"}
	}

	wallet, err := txRepo.GetByTherapistID(ctx, req.TherapistID)
	if err != nil {
		return fmt.Errorf("wallet not found")
	}

	if wallet.AvailableBalance < req.Amount {
		return &ValidationError{Code: "amount", Message: "Insufficient balance for payout"}
	}

	// Debit from available balance
	if err := txRepo.UpdateBalances(ctx, wallet.WalletID, -req.Amount, 0); err != nil {
		return err
	}

	// Update request status
	if err := txRepo.UpdatePayoutRequestStatus(ctx, requestID, "completed", adminID, nil, txnRef); err != nil {
		return err
	}

	// Create transaction record
	desc := fmt.Sprintf("Payout #%d via %s", requestID, req.PayoutMethod)
	txn := &model.WalletTransaction{
		WalletID:     wallet.WalletID,
		Type:         "payout",
		Amount:       -req.Amount,
		BalanceAfter: wallet.AvailableBalance - req.Amount,
		PendingAfter: wallet.PendingBalance,
		Description:  &desc,
		ProcessedBy:  &adminID,
	}
	if err := txRepo.CreateTransaction(ctx, txn); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	slog.Info("payout approved", "request_id", requestID, "therapist_id", req.TherapistID, "amount", req.Amount, "admin_id", adminID)
	return nil
}

// RejectPayout rejects a payout request.
func (s *WalletService) RejectPayout(ctx context.Context, requestID, adminID int64, reason string) error {
	req, err := s.walletRepo.GetPayoutRequest(ctx, requestID)
	if err != nil {
		return fmt.Errorf("payout request not found")
	}

	if req.Status != "pending" {
		return &ValidationError{Code: "status", Message: "Request is not pending"}
	}

	if err := s.walletRepo.UpdatePayoutRequestStatus(ctx, requestID, "rejected", adminID, &reason, nil); err != nil {
		return err
	}

	slog.Info("payout rejected", "request_id", requestID, "therapist_id", req.TherapistID, "reason", reason, "admin_id", adminID)
	return nil
}

// CreateCashAdvance creates a new cash advance for a therapist.
func (s *WalletService) CreateCashAdvance(ctx context.Context, therapistID int64, amount float64, repaymentRate float64, adminID int64, reason string) (*model.CashAdvance, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	txRepo := repository.NewWalletRepository(tx)

	wallet, err := txRepo.GetByTherapistID(ctx, therapistID)
	if err != nil {
		wallet, err = txRepo.CreateWallet(ctx, therapistID)
		if err != nil {
			return nil, fmt.Errorf("failed to get/create wallet")
		}
	}

	// Check for existing active advance
	existing, _ := txRepo.GetActiveAdvanceByTherapist(ctx, therapistID)
	if existing != nil {
		return nil, &ValidationError{Code: "therapist_id", Message: "Therapist already has an active cash advance"}
	}

	adv := &model.CashAdvance{
		WalletID:       wallet.WalletID,
		TherapistID:    therapistID,
		OriginalAmount: amount,
		RepaymentRate:  repaymentRate,
		ApprovedBy:     &adminID,
		Reason:         &reason,
	}

	if err := txRepo.CreateCashAdvance(ctx, adv); err != nil {
		slog.Error("failed to create cash advance", "therapist_id", therapistID, "error", err)
		return nil, fmt.Errorf("failed to create cash advance")
	}

	// Credit to available balance (advance is immediately available)
	if err := txRepo.UpdateBalances(ctx, wallet.WalletID, amount, 0); err != nil {
		return nil, err
	}

	// Create transaction
	desc := fmt.Sprintf("Cash advance: %s", reason)
	txn := &model.WalletTransaction{
		WalletID:     wallet.WalletID,
		Type:         "cash_advance",
		Amount:       amount,
		BalanceAfter: wallet.AvailableBalance + amount,
		PendingAfter: wallet.PendingBalance,
		Description:  &desc,
		ProcessedBy:  &adminID,
	}
	if err := txRepo.CreateTransaction(ctx, txn); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	slog.Info("cash advance created", "advance_id", adv.AdvanceID, "therapist_id", therapistID, "amount", amount, "admin_id", adminID)
	return adv, nil
}

// ListPayoutRequests returns all payout requests for a therapist.
func (s *WalletService) ListPayoutRequests(ctx context.Context, therapistID int64) ([]model.PayoutRequest, error) {
	return s.walletRepo.ListPayoutRequestsByTherapist(ctx, therapistID)
}

// ListPendingPayouts returns all pending payout requests (for admin).
func (s *WalletService) ListPendingPayouts(ctx context.Context) ([]model.PayoutRequest, error) {
	return s.walletRepo.ListPendingPayoutRequests(ctx)
}

// GetTransactionHistory returns paginated transaction history.
func (s *WalletService) GetTransactionHistory(ctx context.Context, therapistID int64, page, limit int) ([]model.WalletTransaction, int, error) {
	wallet, err := s.walletRepo.GetByTherapistID(ctx, therapistID)
	if err != nil {
		return nil, 0, fmt.Errorf("wallet not found")
	}
	offset := (page - 1) * limit
	return s.walletRepo.ListTransactions(ctx, wallet.WalletID, limit, offset)
}
