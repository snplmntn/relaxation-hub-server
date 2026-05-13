package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

// WalletRepository handles wallet persistence operations.
type WalletRepository interface {
	// Wallet CRUD
	GetByTherapistID(ctx context.Context, therapistID int64) (*model.Wallet, error)
	CreateWallet(ctx context.Context, therapistID int64) (*model.Wallet, error)
	UpdateBalances(ctx context.Context, walletID int64, availableDelta, pendingDelta float64) error

	// Transactions
	CreateTransaction(ctx context.Context, txn *model.WalletTransaction) error
	ListTransactions(ctx context.Context, walletID int64, limit, offset int) ([]model.WalletTransaction, int, error)
	ListTransactionsKeyset(ctx context.Context, walletID int64, cursor *model.KeysetCursor, limit int) ([]model.WalletTransaction, error)

	// Payout Requests
	CreatePayoutRequest(ctx context.Context, req *model.PayoutRequest) error
	GetPayoutRequest(ctx context.Context, requestID int64) (*model.PayoutRequest, error)
	ListPayoutRequestsByTherapist(ctx context.Context, therapistID int64) ([]model.PayoutRequest, error)
	ListPendingPayoutRequests(ctx context.Context) ([]model.PayoutRequest, error)
	UpdatePayoutRequestStatus(ctx context.Context, requestID int64, status string, processedBy int64, reason, txnRef *string) error

	// Cash Advances
	CreateCashAdvance(ctx context.Context, adv *model.CashAdvance) error
	GetCashAdvance(ctx context.Context, advanceID int64) (*model.CashAdvance, error)
	GetActiveAdvanceByTherapist(ctx context.Context, therapistID int64) (*model.CashAdvance, error)
	UpdateAdvanceBalance(ctx context.Context, advanceID int64, repaymentAmount float64) error
	MarkAdvancePaidOff(ctx context.Context, advanceID int64) error
}

type walletRepoImpl struct {
	db db.DBTX
}

func NewWalletRepository(db db.DBTX) WalletRepository {
	return &walletRepoImpl{db: db}
}

func (r *walletRepoImpl) GetByTherapistID(ctx context.Context, therapistID int64) (*model.Wallet, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	var w model.Wallet
	err := r.db.QueryRow(ctx, `
		SELECT wallet_id, therapist_id, available_balance, pending_balance, 
		       total_earned, total_withdrawn, total_advances, minimum_payout,
		       last_payout_at, created_at, updated_at
		FROM therapist_wallets
		WHERE therapist_id = $1
	`, therapistID).Scan(
		&w.WalletID, &w.TherapistID, &w.AvailableBalance, &w.PendingBalance,
		&w.TotalEarned, &w.TotalWithdrawn, &w.TotalAdvances, &w.MinimumPayout,
		&w.LastPayoutAt, &w.CreatedAt, &w.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &w, nil
}

func (r *walletRepoImpl) CreateWallet(ctx context.Context, therapistID int64) (*model.Wallet, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	var w model.Wallet
	err := r.db.QueryRow(ctx, `
		INSERT INTO therapist_wallets (therapist_id)
		VALUES ($1)
		ON CONFLICT (therapist_id) DO UPDATE SET updated_at = NOW()
		RETURNING wallet_id, therapist_id, available_balance, pending_balance,
		          total_earned, total_withdrawn, total_advances, minimum_payout,
		          last_payout_at, created_at, updated_at
	`, therapistID).Scan(
		&w.WalletID, &w.TherapistID, &w.AvailableBalance, &w.PendingBalance,
		&w.TotalEarned, &w.TotalWithdrawn, &w.TotalAdvances, &w.MinimumPayout,
		&w.LastPayoutAt, &w.CreatedAt, &w.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &w, nil
}

func (r *walletRepoImpl) UpdateBalances(ctx context.Context, walletID int64, availableDelta, pendingDelta float64) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	cmd, err := r.db.Exec(ctx, `
		UPDATE therapist_wallets
		SET available_balance = available_balance + $1,
		    pending_balance = pending_balance + $2,
		    updated_at = NOW()
		WHERE wallet_id = $3
	`, availableDelta, pendingDelta, walletID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *walletRepoImpl) CreateTransaction(ctx context.Context, txn *model.WalletTransaction) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	return r.db.QueryRow(ctx, `
		INSERT INTO wallet_transactions 
		(wallet_id, booking_id, ledger_entry_id, type, amount, balance_after, pending_after, description, processed_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING transaction_id, created_at
	`, txn.WalletID, txn.BookingID, txn.LedgerEntryID, txn.Type, txn.Amount,
		txn.BalanceAfter, txn.PendingAfter, txn.Description, txn.ProcessedBy,
	).Scan(&txn.TransactionID, &txn.CreatedAt)
}

func (r *walletRepoImpl) ListTransactions(ctx context.Context, walletID int64, limit, offset int) ([]model.WalletTransaction, int, error) {
	ctx, cancel := db.WithLongQueryTimeout(ctx)
	defer cancel()

	var total int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM wallet_transactions WHERE wallet_id = $1`, walletID).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(ctx, `
		SELECT transaction_id, wallet_id, booking_id, ledger_entry_id, type, amount, 
		       balance_after, pending_after, description, processed_by, created_at
		FROM wallet_transactions
		WHERE wallet_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, walletID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var txns []model.WalletTransaction
	for rows.Next() {
		var t model.WalletTransaction
		if err := rows.Scan(
			&t.TransactionID, &t.WalletID, &t.BookingID, &t.LedgerEntryID, &t.Type, &t.Amount,
			&t.BalanceAfter, &t.PendingAfter, &t.Description, &t.ProcessedBy, &t.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		txns = append(txns, t)
	}
	return txns, total, rows.Err()
}

func (r *walletRepoImpl) ListTransactionsKeyset(ctx context.Context, walletID int64, cursor *model.KeysetCursor, limit int) ([]model.WalletTransaction, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	query := `
		SELECT transaction_id, wallet_id, booking_id, ledger_entry_id, type, amount, 
		       balance_after, pending_after, description, processed_by, created_at
		FROM wallet_transactions
		WHERE wallet_id = $1
		ORDER BY created_at DESC, transaction_id DESC
		LIMIT $2
	`
	args := []any{walletID, limit}
	if cursor != nil {
		query = `
			SELECT transaction_id, wallet_id, booking_id, ledger_entry_id, type, amount, 
			       balance_after, pending_after, description, processed_by, created_at
			FROM wallet_transactions
			WHERE wallet_id = $1
			  AND (created_at < $2 OR (created_at = $2 AND transaction_id < $3))
			ORDER BY created_at DESC, transaction_id DESC
			LIMIT $4
		`
		args = []any{walletID, cursor.CreatedAt, cursor.ID, limit}
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var txns []model.WalletTransaction
	for rows.Next() {
		var t model.WalletTransaction
		if err := rows.Scan(
			&t.TransactionID, &t.WalletID, &t.BookingID, &t.LedgerEntryID, &t.Type, &t.Amount,
			&t.BalanceAfter, &t.PendingAfter, &t.Description, &t.ProcessedBy, &t.CreatedAt,
		); err != nil {
			return nil, err
		}
		txns = append(txns, t)
	}
	return txns, rows.Err()
}

func (r *walletRepoImpl) CreatePayoutRequest(ctx context.Context, req *model.PayoutRequest) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	return r.db.QueryRow(ctx, `
		INSERT INTO payout_requests (wallet_id, therapist_id, amount, payout_method, account_details)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING request_id, status, created_at, updated_at
	`, req.WalletID, req.TherapistID, req.Amount, req.PayoutMethod, req.AccountDetails,
	).Scan(&req.RequestID, &req.Status, &req.CreatedAt, &req.UpdatedAt)
}

func (r *walletRepoImpl) GetPayoutRequest(ctx context.Context, requestID int64) (*model.PayoutRequest, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	var req model.PayoutRequest
	err := r.db.QueryRow(ctx, `
		SELECT request_id, wallet_id, therapist_id, amount, payout_method, account_details,
		       status, processed_by, processed_at, rejection_reason, transaction_reference,
		       created_at, updated_at
		FROM payout_requests
		WHERE request_id = $1
	`, requestID).Scan(
		&req.RequestID, &req.WalletID, &req.TherapistID, &req.Amount, &req.PayoutMethod, &req.AccountDetails,
		&req.Status, &req.ProcessedBy, &req.ProcessedAt, &req.RejectionReason, &req.TransactionReference,
		&req.CreatedAt, &req.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &req, nil
}

func (r *walletRepoImpl) ListPayoutRequestsByTherapist(ctx context.Context, therapistID int64) ([]model.PayoutRequest, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	rows, err := r.db.Query(ctx, `
		SELECT request_id, wallet_id, therapist_id, amount, payout_method, account_details,
		       status, processed_by, processed_at, rejection_reason, transaction_reference,
		       created_at, updated_at
		FROM payout_requests
		WHERE therapist_id = $1
		ORDER BY created_at DESC
	`, therapistID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reqs []model.PayoutRequest
	for rows.Next() {
		var req model.PayoutRequest
		if err := rows.Scan(
			&req.RequestID, &req.WalletID, &req.TherapistID, &req.Amount, &req.PayoutMethod, &req.AccountDetails,
			&req.Status, &req.ProcessedBy, &req.ProcessedAt, &req.RejectionReason, &req.TransactionReference,
			&req.CreatedAt, &req.UpdatedAt,
		); err != nil {
			return nil, err
		}
		reqs = append(reqs, req)
	}
	return reqs, rows.Err()
}

func (r *walletRepoImpl) ListPendingPayoutRequests(ctx context.Context) ([]model.PayoutRequest, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	rows, err := r.db.Query(ctx, `
		SELECT request_id, wallet_id, therapist_id, amount, payout_method, account_details,
		       status, processed_by, processed_at, rejection_reason, transaction_reference,
		       created_at, updated_at
		FROM payout_requests
		WHERE status = 'pending'
		ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reqs []model.PayoutRequest
	for rows.Next() {
		var req model.PayoutRequest
		if err := rows.Scan(
			&req.RequestID, &req.WalletID, &req.TherapistID, &req.Amount, &req.PayoutMethod, &req.AccountDetails,
			&req.Status, &req.ProcessedBy, &req.ProcessedAt, &req.RejectionReason, &req.TransactionReference,
			&req.CreatedAt, &req.UpdatedAt,
		); err != nil {
			return nil, err
		}
		reqs = append(reqs, req)
	}
	return reqs, rows.Err()
}

func (r *walletRepoImpl) UpdatePayoutRequestStatus(ctx context.Context, requestID int64, status string, processedBy int64, reason, txnRef *string) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	cmd, err := r.db.Exec(ctx, `
		UPDATE payout_requests
		SET status = $1, processed_by = $2, processed_at = NOW(), 
		    rejection_reason = COALESCE($3, rejection_reason),
		    transaction_reference = COALESCE($4, transaction_reference),
		    updated_at = NOW()
		WHERE request_id = $5
	`, status, processedBy, reason, txnRef, requestID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *walletRepoImpl) CreateCashAdvance(ctx context.Context, adv *model.CashAdvance) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	return r.db.QueryRow(ctx, `
		INSERT INTO cash_advances (wallet_id, therapist_id, original_amount, remaining_balance, repayment_rate, approved_by, approved_at, reason)
		VALUES ($1, $2, $3, $3, $4, $5, NOW(), $6)
		RETURNING advance_id, status, created_at, updated_at
	`, adv.WalletID, adv.TherapistID, adv.OriginalAmount, adv.RepaymentRate, adv.ApprovedBy, adv.Reason,
	).Scan(&adv.AdvanceID, &adv.Status, &adv.CreatedAt, &adv.UpdatedAt)
}

func (r *walletRepoImpl) GetCashAdvance(ctx context.Context, advanceID int64) (*model.CashAdvance, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	var adv model.CashAdvance
	err := r.db.QueryRow(ctx, `
		SELECT advance_id, wallet_id, therapist_id, original_amount, remaining_balance, repayment_rate,
		       status, approved_by, approved_at, reason, paid_off_at, created_at, updated_at
		FROM cash_advances
		WHERE advance_id = $1
	`, advanceID).Scan(
		&adv.AdvanceID, &adv.WalletID, &adv.TherapistID, &adv.OriginalAmount, &adv.RemainingBalance, &adv.RepaymentRate,
		&adv.Status, &adv.ApprovedBy, &adv.ApprovedAt, &adv.Reason, &adv.PaidOffAt, &adv.CreatedAt, &adv.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &adv, nil
}

func (r *walletRepoImpl) GetActiveAdvanceByTherapist(ctx context.Context, therapistID int64) (*model.CashAdvance, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	var adv model.CashAdvance
	err := r.db.QueryRow(ctx, `
		SELECT advance_id, wallet_id, therapist_id, original_amount, remaining_balance, repayment_rate,
		       status, approved_by, approved_at, reason, paid_off_at, created_at, updated_at
		FROM cash_advances
		WHERE therapist_id = $1 AND status = 'active'
		ORDER BY created_at DESC
		LIMIT 1
	`, therapistID).Scan(
		&adv.AdvanceID, &adv.WalletID, &adv.TherapistID, &adv.OriginalAmount, &adv.RemainingBalance, &adv.RepaymentRate,
		&adv.Status, &adv.ApprovedBy, &adv.ApprovedAt, &adv.Reason, &adv.PaidOffAt, &adv.CreatedAt, &adv.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // No active advance
		}
		return nil, err
	}
	return &adv, nil
}

func (r *walletRepoImpl) UpdateAdvanceBalance(ctx context.Context, advanceID int64, repaymentAmount float64) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	cmd, err := r.db.Exec(ctx, `
		UPDATE cash_advances
		SET remaining_balance = remaining_balance - $1, updated_at = NOW()
		WHERE advance_id = $2 AND status = 'active'
	`, repaymentAmount, advanceID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("advance not found or not active")
	}
	return nil
}

func (r *walletRepoImpl) MarkAdvancePaidOff(ctx context.Context, advanceID int64) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	now := time.Now()
	cmd, err := r.db.Exec(ctx, `
		UPDATE cash_advances
		SET status = 'paid_off', remaining_balance = 0, paid_off_at = $1, updated_at = NOW()
		WHERE advance_id = $2
	`, now, advanceID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
