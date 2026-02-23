package service

import (
	"context"
	"fmt"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

type RiderWalletService struct {
	db db.DBTX
}

func NewRiderWalletService(db db.DBTX) *RiderWalletService {
	return &RiderWalletService{
		db: db,
	}
}

// GetWallet retrieves the rider's wallet with current balance
func (s *RiderWalletService) GetWallet(ctx context.Context, riderID int64) (*model.RiderWallet, error) {
	query := `
		SELECT rider_id, balance_cents, total_earned_cents, total_withdrawn_cents, created_at, updated_at
		FROM rider_wallets
		WHERE rider_id = $1
	`

	var wallet model.RiderWallet
	err := s.db.QueryRow(ctx, query, riderID).Scan(
		&wallet.RiderID,
		&wallet.BalanceCents,
		&wallet.TotalEarnedCents,
		&wallet.TotalWithdrawnCents,
		&wallet.CreatedAt,
		&wallet.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get rider wallet: %w", err)
	}

	return &wallet, nil
}

// GetTransactions retrieves transaction history for a rider
func (s *RiderWalletService) GetTransactions(ctx context.Context, riderID int64, limit int, offset int) ([]model.RiderTransaction, error) {
	query := `
		SELECT transaction_id, rider_id, transaction_type, amount_cents, ride_id, payout_method_id,
		       status, description, created_at, completed_at
		FROM rider_transactions
		WHERE rider_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := s.db.Query(ctx, query, riderID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get transactions: %w", err)
	}
	defer rows.Close()

	var transactions []model.RiderTransaction
	for rows.Next() {
		var tx model.RiderTransaction
		err := rows.Scan(
			&tx.TransactionID,
			&tx.RiderID,
			&tx.TransactionType,
			&tx.AmountCents,
			&tx.RideID,
			&tx.PayoutMethodID,
			&tx.Status,
			&tx.Description,
			&tx.CreatedAt,
			&tx.CompletedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan transaction: %w", err)
		}
		transactions = append(transactions, tx)
	}

	return transactions, nil
}

// RequestPayout initiates a payout request (admin approval required)
func (s *RiderWalletService) RequestPayout(ctx context.Context, riderID int64, amountCents int, payoutMethodID int) error {
	// Validate balance
	wallet, err := s.GetWallet(ctx, riderID)
	if err != nil {
		return err
	}

	if wallet.BalanceCents < amountCents {
		return fmt.Errorf("insufficient balance: have %d, requested %d", wallet.BalanceCents, amountCents)
	}

	// Minimum payout check (e.g., 100 PHP = 10000 cents)
	if amountCents < 10000 {
		return fmt.Errorf("minimum payout is ₱100.00")
	}

	// Validate payout method ownership
	var exists bool
	checkMethod := `SELECT EXISTS(SELECT 1 FROM rider_payout_methods WHERE id = $1 AND rider_id = $2)`
	err = s.db.QueryRow(ctx, checkMethod, payoutMethodID, riderID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to validate payout method: %w", err)
	}
	if !exists {
		return fmt.Errorf("payout method not found or does not belong to rider")
	}

	// Create pending payout transaction
	query := `
		INSERT INTO rider_transactions (rider_id, transaction_type, amount_cents, status, description, payout_method_id)
		VALUES ($1, 'payout', $2, 'pending', $3, $4)
		RETURNING transaction_id
	`

	var txID int
	err = s.db.QueryRow(
		ctx,
		query,
		riderID,
		-amountCents, // Negative for debit
		fmt.Sprintf("Payout request for ₱%.2f", float64(amountCents)/100),
		payoutMethodID,
	).Scan(&txID)

	if err != nil {
		return fmt.Errorf("failed to create payout transaction: %w", err)
	}

	// Note: Balance is NOT deducted until admin approves
	// Admin will call ApprovePayout() which updates the wallet

	return nil
}

// GetPayoutMethods retrieves all payout methods for a rider
func (s *RiderWalletService) GetPayoutMethods(ctx context.Context, riderID int64) ([]model.RiderPayoutMethod, error) {
	query := `
		SELECT id, rider_id, method_type, provider_name, account_number, account_name, is_default, created_at, updated_at
		FROM rider_payout_methods
		WHERE rider_id = $1
		ORDER BY is_default DESC, created_at DESC
	`

	rows, err := s.db.Query(ctx, query, riderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get payout methods: %w", err)
	}
	defer rows.Close()

	var methods []model.RiderPayoutMethod
	for rows.Next() {
		var m model.RiderPayoutMethod
		err := rows.Scan(
			&m.ID,
			&m.RiderID,
			&m.MethodType,
			&m.ProviderName,
			&m.AccountNumber,
			&m.AccountName,
			&m.IsDefault,
			&m.CreatedAt,
			&m.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan payout method: %w", err)
		}
		methods = append(methods, m)
	}

	return methods, nil
}

// AddPayoutMethod adds a new payout method and optionally sets it as default
func (s *RiderWalletService) AddPayoutMethod(ctx context.Context, method *model.RiderPayoutMethod) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// If setting as default, unset others
	if method.IsDefault {
		_, err = tx.Exec(ctx, "UPDATE rider_payout_methods SET is_default = FALSE WHERE rider_id = $1", method.RiderID)
		if err != nil {
			return fmt.Errorf("failed to unset existing default: %w", err)
		}
	} else {
		// If it's the first method, make it default regardless
		var count int
		err = tx.QueryRow(ctx, "SELECT COUNT(*) FROM rider_payout_methods WHERE rider_id = $1", method.RiderID).Scan(&count)
		if err == nil && count == 0 {
			method.IsDefault = true
		}
	}

	query := `
		INSERT INTO rider_payout_methods (rider_id, method_type, provider_name, account_number, account_name, is_default)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at
	`

	err = tx.QueryRow(
		ctx,
		query,
		method.RiderID,
		method.MethodType,
		method.ProviderName,
		method.AccountNumber,
		method.AccountName,
		method.IsDefault,
	).Scan(&method.ID, &method.CreatedAt, &method.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to add payout method: %w", err)
	}

	return tx.Commit(ctx)
}

// DeletePayoutMethod removes a payout method
func (s *RiderWalletService) DeletePayoutMethod(ctx context.Context, riderID int64, methodID int) error {
	// Don't allow deleting if it's the only one and has pending payouts?
	// For now simple delete
	query := `DELETE FROM rider_payout_methods WHERE id = $1 AND rider_id = $2`
	result, err := s.db.Exec(ctx, query, methodID, riderID)
	if err != nil {
		return fmt.Errorf("failed to delete payout method: %w", err)
	}

	rows := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("payout method not found or not owned by rider")
	}

	return nil
}

// GetRiderTransaction retrieves a single rider transaction by ID.
func (s *RiderWalletService) GetRiderTransaction(ctx context.Context, transactionID int) (*model.RiderTransaction, error) {
	var tx model.RiderTransaction
	err := s.db.QueryRow(ctx, `
		SELECT transaction_id, rider_id, transaction_type, amount_cents, ride_id, payout_method_id,
		       status, description, created_at, completed_at
		FROM rider_transactions
		WHERE transaction_id = $1
	`, transactionID).Scan(
		&tx.TransactionID, &tx.RiderID, &tx.TransactionType, &tx.AmountCents,
		&tx.RideID, &tx.PayoutMethodID, &tx.Status, &tx.Description,
		&tx.CreatedAt, &tx.CompletedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("transaction not found: %w", err)
	}
	return &tx, nil
}

// ApprovePayout processes an approved payout (admin-only)
func (s *RiderWalletService) ApprovePayout(ctx context.Context, transactionID int) error {
	// Get transaction details
	var riderID int64
	var amountCents int
	var status string

	query := `
		SELECT rider_id, amount_cents, status
		FROM rider_transactions
		WHERE transaction_id = $1 AND transaction_type = 'payout'
	`

	err := s.db.QueryRow(ctx, query, transactionID).Scan(&riderID, &amountCents, &status)
	if err != nil {
		return fmt.Errorf("transaction not found: %w", err)
	}

	if status != "pending" {
		return fmt.Errorf("transaction is not pending (status: %s)", status)
	}

	// Begin transaction
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Update wallet (deduct balance, add to withdrawn)
	updateWallet := `
		UPDATE rider_wallets
		SET 
			balance_cents = balance_cents + $1,
			total_withdrawn_cents = total_withdrawn_cents + $2,
			updated_at = NOW()
		WHERE rider_id = $3
	`

	_, err = tx.Exec(ctx, updateWallet, amountCents, -amountCents, riderID)
	if err != nil {
		return fmt.Errorf("failed to update wallet: %w", err)
	}

	// Mark transaction as completed
	updateTx := `
		UPDATE rider_transactions
		SET status = 'completed', completed_at = NOW()
		WHERE transaction_id = $1
	`

	_, err = tx.Exec(ctx, updateTx, transactionID)
	if err != nil {
		return fmt.Errorf("failed to update transaction: %w", err)
	}

	return tx.Commit(ctx)
}

// RecordEarnings is called when a ride is completed (usually via trigger, but can be manual)
func (s *RiderWalletService) RecordEarnings(ctx context.Context, rideID int64, riderID int64, earningsCents int) error {
	// This function is mostly handled by DB trigger, but can be called manually
	// for adjustments or if trigger fails

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Update ride earnings
	updateRide := `
		UPDATE rides
		SET rider_earnings_cents = $1
		WHERE ride_id = $2 AND rider_id = $3
	`

	_, err = tx.Exec(ctx, updateRide, earningsCents, rideID, riderID)
	if err != nil {
		return fmt.Errorf("failed to update ride earnings: %w", err)
	}

	// Trigger will handle wallet update and transaction creation

	return tx.Commit(ctx)
}

// GetPerformanceMetrics retrieves rider performance stats
func (s *RiderWalletService) GetPerformanceMetrics(ctx context.Context, riderID int64) (*model.RiderPerformanceMetrics, error) {
	query := `
		SELECT rider_id, total_offers_received, total_rides_accepted, total_rides_completed,
		       total_rides_cancelled, acceptance_rate, completion_rate, average_rating,
		       total_ratings, rating_sum, updated_at
		FROM rider_performance_metrics
		WHERE rider_id = $1
	`

	var metrics model.RiderPerformanceMetrics
	err := s.db.QueryRow(ctx, query, riderID).Scan(
		&metrics.RiderID,
		&metrics.TotalOffersReceived,
		&metrics.TotalRidesAccepted,
		&metrics.TotalRidesCompleted,
		&metrics.TotalRidesCancelled,
		&metrics.AcceptanceRate,
		&metrics.CompletionRate,
		&metrics.AverageRating,
		&metrics.TotalRatings,
		&metrics.RatingSum,
		&metrics.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get performance metrics: %w", err)
	}

	// Calculate today's earnings
	// For PostgreSQL, CURRENT_DATE returns the date at start of transaction
	// We want earnings from rides created (or completed/paid?) since start of today (local time might be tricky, assuming server time)
	// Ideally we'd pass the timezone offset, but for now we'll use server's local day or UTC day
	earningsQuery := `
		SELECT COALESCE(SUM(amount_cents), 0)
		FROM rider_transactions
		WHERE rider_id = $1 
		  AND transaction_type = 'ride_earning' 
		  AND created_at >= CURRENT_DATE
	`

	err = s.db.QueryRow(ctx, earningsQuery, riderID).Scan(&metrics.TodayEarnedCents)
	if err != nil {
		// Log error but don't fail the whole request?
		// For now, let's treat it as 0 if it fails, or just return error
		// return nil, fmt.Errorf("failed to get today's earnings: %w", err)
		// Safer to just default to 0
		metrics.TodayEarnedCents = 0
	}

	return &metrics, nil
}

// RiderPayoutListItem is the admin-facing view of a pending payout request.
type RiderPayoutListItem struct {
	TransactionID int     `json:"transaction_id"`
	RiderID       int64   `json:"rider_id"`
	RiderName     string  `json:"rider_name"`
	AmountCents   int     `json:"amount_cents"`
	AmountPHP     float64 `json:"amount_php"`
	Status        string  `json:"status"`
	Description   *string `json:"description,omitempty"`
	CreatedAt     string  `json:"created_at"`
}

// ListPendingRiderPayouts returns all pending payout requests from riders.
func (s *RiderWalletService) ListPendingRiderPayouts(ctx context.Context) ([]RiderPayoutListItem, error) {
	rows, err := s.db.Query(ctx, `
		SELECT rt.transaction_id, rt.rider_id, u.full_name, rt.amount_cents, rt.status, rt.description, rt.created_at
		FROM rider_transactions rt
		JOIN users u ON rt.rider_id = u.user_id
		WHERE rt.transaction_type = 'payout' AND rt.status = 'pending'
		ORDER BY rt.created_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list pending rider payouts: %w", err)
	}
	defer rows.Close()

	var items []RiderPayoutListItem
	for rows.Next() {
		var item RiderPayoutListItem
		var createdAt time.Time
		if err := rows.Scan(&item.TransactionID, &item.RiderID, &item.RiderName, &item.AmountCents, &item.Status, &item.Description, &createdAt); err != nil {
			return nil, err
		}
		item.AmountPHP = float64(item.AmountCents) / 100.0
		item.CreatedAt = createdAt.Format(time.RFC3339)
		items = append(items, item)
	}
	return items, rows.Err()
}

// RejectRiderPayout marks a payout transaction as failed.
func (s *RiderWalletService) RejectRiderPayout(ctx context.Context, transactionID int) error {
	cmd, err := s.db.Exec(ctx, `
		UPDATE rider_transactions
		SET status = 'failed', completed_at = NOW()
		WHERE transaction_id = $1 AND transaction_type = 'payout' AND status = 'pending'
	`, transactionID)
	if err != nil {
		return fmt.Errorf("failed to reject payout: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("transaction not found or not pending")
	}
	return nil
}

// IncrementOffersReceived updates the offer counter when a new ride offer is sent
func (s *RiderWalletService) IncrementOffersReceived(ctx context.Context, riderID int64) error {
	query := `
		UPDATE rider_performance_metrics
		SET total_offers_received = total_offers_received + 1,
		    updated_at = NOW()
		WHERE rider_id = $1
	`

	_, err := s.db.Exec(ctx, query, riderID)
	return err
}
