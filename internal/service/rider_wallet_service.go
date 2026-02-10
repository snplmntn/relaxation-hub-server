package service

import (
	"context"
	"fmt"

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
		SELECT transaction_id, rider_id, transaction_type, amount_cents, ride_id, 
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
func (s *RiderWalletService) RequestPayout(ctx context.Context, riderID int64, amountCents int) error {
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
	
	// Create pending payout transaction
	query := `
		INSERT INTO rider_transactions (rider_id, transaction_type, amount_cents, status, description)
		VALUES ($1, 'payout', $2, 'pending', $3)
		RETURNING transaction_id
	`
	
	var txID int
	err = s.db.QueryRow(
		ctx,
		query,
		riderID,
		-amountCents, // Negative for debit
		fmt.Sprintf("Payout request for ₱%.2f", float64(amountCents)/100),
	).Scan(&txID)
	
	if err != nil {
		return fmt.Errorf("failed to create payout transaction: %w", err)
	}
	
	// Note: Balance is NOT deducted until admin approves
	// Admin will call ApprovePayout() which updates the wallet
	
	return nil
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
