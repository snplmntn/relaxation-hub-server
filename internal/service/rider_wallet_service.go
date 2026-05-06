package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
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

// CreateInitialRiderRecords ensures wallet and performance rows exist for a rider.
func (s *RiderWalletService) CreateInitialRiderRecords(ctx context.Context, riderID int64) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := ensureRiderRecordsInTx(ctx, tx, riderID); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit rider record initialization: %w", err)
	}

	return nil
}

// GetWallet retrieves the rider's wallet with current balance.
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
		if err == pgx.ErrNoRows {
			if initErr := s.CreateInitialRiderRecords(ctx, riderID); initErr != nil {
				return nil, fmt.Errorf("failed to initialize rider wallet records: %w", initErr)
			}
			err = s.db.QueryRow(ctx, query, riderID).Scan(
				&wallet.RiderID,
				&wallet.BalanceCents,
				&wallet.TotalEarnedCents,
				&wallet.TotalWithdrawnCents,
				&wallet.CreatedAt,
				&wallet.UpdatedAt,
			)
			if err == nil {
				return &wallet, nil
			}
		}
		return nil, fmt.Errorf("failed to get rider wallet: %w", err)
	}

	return &wallet, nil
}

// GetTransactions retrieves transaction history for a rider.
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

// RequestPayout initiates a payout request (admin approval required).
func (s *RiderWalletService) RequestPayout(ctx context.Context, riderID int64, amountCents int, payoutMethodID int) error {
	if amountCents < 10000 {
		return fmt.Errorf("minimum payout is ₱100.00")
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := ensureRiderRecordsInTx(ctx, tx, riderID); err != nil {
		return err
	}

	var walletBalance int
	err = tx.QueryRow(ctx, `
		SELECT balance_cents
		FROM rider_wallets
		WHERE rider_id = $1
		FOR UPDATE
	`, riderID).Scan(&walletBalance)
	if err != nil {
		return fmt.Errorf("failed to lock rider wallet: %w", err)
	}

	var exists bool
	checkMethod := `SELECT EXISTS(SELECT 1 FROM rider_payout_methods WHERE id = $1 AND rider_id = $2)`
	err = tx.QueryRow(ctx, checkMethod, payoutMethodID, riderID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to validate payout method: %w", err)
	}
	if !exists {
		return fmt.Errorf("payout method not found or does not belong to rider")
	}

	var pendingReservedCents int
	err = tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(-amount_cents), 0)
		FROM rider_transactions
		WHERE rider_id = $1 AND transaction_type = 'payout' AND status = 'pending'
	`, riderID).Scan(&pendingReservedCents)
	if err != nil {
		return fmt.Errorf("failed to calculate pending payout reservations: %w", err)
	}

	availableBalance := walletBalance - pendingReservedCents
	if availableBalance < amountCents {
		return fmt.Errorf("insufficient available balance: have %d, reserved %d, requested %d", walletBalance, pendingReservedCents, amountCents)
	}

	var txID int
	err = tx.QueryRow(
		ctx,
		`
		INSERT INTO rider_transactions (rider_id, transaction_type, amount_cents, status, description, payout_method_id)
		VALUES ($1, 'payout', $2, 'pending', $3, $4)
		RETURNING transaction_id
	`,
		riderID,
		-amountCents,
		fmt.Sprintf("Payout request for ₱%.2f", float64(amountCents)/100),
		payoutMethodID,
	).Scan(&txID)
	if err != nil {
		return fmt.Errorf("failed to create payout transaction: %w", err)
	}

	return tx.Commit(ctx)
}

// GetPayoutMethods retrieves all payout methods for a rider.
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

// AddPayoutMethod adds a new payout method and optionally sets it as default.
func (s *RiderWalletService) AddPayoutMethod(ctx context.Context, method *model.RiderPayoutMethod) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if method.IsDefault {
		_, err = tx.Exec(ctx, "UPDATE rider_payout_methods SET is_default = FALSE WHERE rider_id = $1", method.RiderID)
		if err != nil {
			return fmt.Errorf("failed to unset existing default: %w", err)
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

	if err := ensureDefaultPayoutMethodInTx(ctx, tx, method.RiderID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// DeletePayoutMethod removes a payout method.
func (s *RiderWalletService) DeletePayoutMethod(ctx context.Context, riderID int64, methodID int) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	result, err := tx.Exec(ctx, `DELETE FROM rider_payout_methods WHERE id = $1 AND rider_id = $2`, methodID, riderID)
	if err != nil {
		return fmt.Errorf("failed to delete payout method: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("payout method not found or not owned by rider")
	}

	if err := ensureDefaultPayoutMethodInTx(ctx, tx, riderID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// UpdatePayoutMethod updates a rider-owned payout method.
func (s *RiderWalletService) UpdatePayoutMethod(ctx context.Context, riderID int64, method *model.RiderPayoutMethod) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var wasDefault bool
	err = tx.QueryRow(ctx, `
		SELECT is_default
		FROM rider_payout_methods
		WHERE id = $1 AND rider_id = $2
		FOR UPDATE
	`, method.ID, riderID).Scan(&wasDefault)
	if err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("payout method not found or not owned by rider")
		}
		return fmt.Errorf("failed to load payout method state: %w", err)
	}

	if method.IsDefault {
		_, err = tx.Exec(ctx, "UPDATE rider_payout_methods SET is_default = FALSE WHERE rider_id = $1", riderID)
		if err != nil {
			return fmt.Errorf("failed to unset existing default: %w", err)
		}
	}

	cmd, err := tx.Exec(ctx, `
		UPDATE rider_payout_methods
		SET method_type = $1,
			provider_name = $2,
			account_number = $3,
			account_name = $4,
			is_default = $5,
			updated_at = NOW()
		WHERE id = $6 AND rider_id = $7
	`, method.MethodType, method.ProviderName, method.AccountNumber, method.AccountName, method.IsDefault, method.ID, riderID)
	if err != nil {
		return fmt.Errorf("failed to update payout method: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("payout method not found or not owned by rider")
	}

	err = tx.QueryRow(ctx, `
		SELECT created_at, updated_at
		FROM rider_payout_methods
		WHERE id = $1 AND rider_id = $2
	`, method.ID, riderID).Scan(&method.CreatedAt, &method.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to reload payout method timestamps: %w", err)
	}

	if wasDefault && !method.IsDefault {
		if err := ensureDefaultPayoutMethodInTxWithExclusion(ctx, tx, riderID, &method.ID); err != nil {
			return err
		}
	} else if err := ensureDefaultPayoutMethodInTx(ctx, tx, riderID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// GetEmergencyContacts retrieves all emergency contacts for a rider.
func (s *RiderWalletService) GetEmergencyContacts(ctx context.Context, riderID int64) ([]model.RiderEmergencyContact, error) {
	rows, err := s.db.Query(ctx, `
		SELECT contact_id, rider_id, full_name, phone_number, relationship, is_primary, created_at, updated_at
		FROM rider_emergency_contacts
		WHERE rider_id = $1
		ORDER BY is_primary DESC, created_at DESC
	`, riderID)
	if err != nil {
		return nil, fmt.Errorf("failed to list emergency contacts: %w", err)
	}
	defer rows.Close()

	var contacts []model.RiderEmergencyContact
	for rows.Next() {
		var c model.RiderEmergencyContact
		if err := rows.Scan(
			&c.ContactID,
			&c.RiderID,
			&c.FullName,
			&c.PhoneNumber,
			&c.Relationship,
			&c.IsPrimary,
			&c.CreatedAt,
			&c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan emergency contact: %w", err)
		}
		contacts = append(contacts, c)
	}

	return contacts, rows.Err()
}

// AddEmergencyContact creates a rider emergency contact.
func (s *RiderWalletService) AddEmergencyContact(ctx context.Context, contact *model.RiderEmergencyContact) error {
	normalizedPhone, err := validateAndNormalizeEmergencyContactInput(contact.FullName, contact.PhoneNumber)
	if err != nil {
		return err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if contact.IsPrimary {
		_, err = tx.Exec(ctx, "UPDATE rider_emergency_contacts SET is_primary = FALSE WHERE rider_id = $1", contact.RiderID)
		if err != nil {
			return fmt.Errorf("failed to unset existing primary contact: %w", err)
		}
	}

	err = tx.QueryRow(ctx, `
		INSERT INTO rider_emergency_contacts (rider_id, full_name, phone_number, relationship, is_primary)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING contact_id, created_at, updated_at
	`, contact.RiderID, strings.TrimSpace(contact.FullName), normalizedPhone, contact.Relationship, contact.IsPrimary).
		Scan(&contact.ContactID, &contact.CreatedAt, &contact.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create emergency contact: %w", err)
	}

	contact.PhoneNumber = normalizedPhone
	if err := ensurePrimaryEmergencyContactInTx(ctx, tx, contact.RiderID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// UpdateEmergencyContact updates a rider-owned emergency contact.
func (s *RiderWalletService) UpdateEmergencyContact(ctx context.Context, riderID int64, contact *model.RiderEmergencyContact) error {
	normalizedPhone, err := validateAndNormalizeEmergencyContactInput(contact.FullName, contact.PhoneNumber)
	if err != nil {
		return err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var wasPrimary bool
	err = tx.QueryRow(ctx, `
		SELECT is_primary
		FROM rider_emergency_contacts
		WHERE contact_id = $1 AND rider_id = $2
		FOR UPDATE
	`, contact.ContactID, riderID).Scan(&wasPrimary)
	if err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("emergency contact not found or not owned by rider")
		}
		return fmt.Errorf("failed to load emergency contact state: %w", err)
	}

	if contact.IsPrimary {
		_, err = tx.Exec(ctx, "UPDATE rider_emergency_contacts SET is_primary = FALSE WHERE rider_id = $1", riderID)
		if err != nil {
			return fmt.Errorf("failed to unset existing primary contact: %w", err)
		}
	}

	cmd, err := tx.Exec(ctx, `
		UPDATE rider_emergency_contacts
		SET full_name = $1,
			phone_number = $2,
			relationship = $3,
			is_primary = $4,
			updated_at = NOW()
		WHERE contact_id = $5 AND rider_id = $6
	`, strings.TrimSpace(contact.FullName), normalizedPhone, contact.Relationship, contact.IsPrimary, contact.ContactID, riderID)
	if err != nil {
		return fmt.Errorf("failed to update emergency contact: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("emergency contact not found or not owned by rider")
	}

	err = tx.QueryRow(ctx, `
		SELECT created_at, updated_at
		FROM rider_emergency_contacts
		WHERE contact_id = $1 AND rider_id = $2
	`, contact.ContactID, riderID).Scan(&contact.CreatedAt, &contact.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to reload emergency contact timestamps: %w", err)
	}

	contact.PhoneNumber = normalizedPhone
	if wasPrimary && !contact.IsPrimary {
		if err := ensurePrimaryEmergencyContactInTxWithExclusion(ctx, tx, riderID, &contact.ContactID); err != nil {
			return err
		}
	} else if err := ensurePrimaryEmergencyContactInTx(ctx, tx, riderID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// DeleteEmergencyContact deletes a rider-owned emergency contact.
func (s *RiderWalletService) DeleteEmergencyContact(ctx context.Context, riderID int64, contactID int) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	cmd, err := tx.Exec(ctx, `
		DELETE FROM rider_emergency_contacts
		WHERE contact_id = $1 AND rider_id = $2
	`, contactID, riderID)
	if err != nil {
		return fmt.Errorf("failed to delete emergency contact: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("emergency contact not found or not owned by rider")
	}

	if err := ensurePrimaryEmergencyContactInTx(ctx, tx, riderID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// GetRiderTransaction retrieves a single rider transaction by ID.
func (s *RiderWalletService) GetRiderTransaction(ctx context.Context, transactionID int) (*model.RiderTransaction, error) {
	var txItem model.RiderTransaction
	err := s.db.QueryRow(ctx, `
		SELECT transaction_id, rider_id, transaction_type, amount_cents, ride_id, payout_method_id,
		       status, description, created_at, completed_at
		FROM rider_transactions
		WHERE transaction_id = $1
	`, transactionID).Scan(
		&txItem.TransactionID, &txItem.RiderID, &txItem.TransactionType, &txItem.AmountCents,
		&txItem.RideID, &txItem.PayoutMethodID, &txItem.Status, &txItem.Description,
		&txItem.CreatedAt, &txItem.CompletedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("transaction not found: %w", err)
	}
	return &txItem, nil
}

// ApprovePayout processes an approved payout (admin-only).
func (s *RiderWalletService) ApprovePayout(ctx context.Context, transactionID int) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var riderID int64
	var amountCents int
	var status string
	err = tx.QueryRow(ctx, `
		SELECT rider_id, amount_cents, status
		FROM rider_transactions
		WHERE transaction_id = $1 AND transaction_type = 'payout'
		FOR UPDATE
	`, transactionID).Scan(&riderID, &amountCents, &status)
	if err != nil {
		return fmt.Errorf("transaction not found: %w", err)
	}
	if status != "pending" {
		return fmt.Errorf("transaction is not pending (status: %s)", status)
	}
	if amountCents >= 0 {
		return fmt.Errorf("invalid payout transaction amount")
	}

	var walletBalance int
	err = tx.QueryRow(ctx, `
		SELECT balance_cents
		FROM rider_wallets
		WHERE rider_id = $1
		FOR UPDATE
	`, riderID).Scan(&walletBalance)
	if err != nil {
		return fmt.Errorf("failed to lock rider wallet: %w", err)
	}
	if walletBalance+amountCents < 0 {
		return fmt.Errorf("insufficient wallet balance for payout approval")
	}

	_, err = tx.Exec(ctx, `
		UPDATE rider_wallets
		SET 
			balance_cents = balance_cents + $1,
			total_withdrawn_cents = total_withdrawn_cents + $2,
			updated_at = NOW()
		WHERE rider_id = $3
	`, amountCents, -amountCents, riderID)
	if err != nil {
		return fmt.Errorf("failed to update wallet: %w", err)
	}

	cmd, err := tx.Exec(ctx, `
		UPDATE rider_transactions
		SET status = 'completed', completed_at = NOW()
		WHERE transaction_id = $1 AND status = 'pending'
	`, transactionID)
	if err != nil {
		return fmt.Errorf("failed to update transaction: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("transaction already processed")
	}

	return tx.Commit(ctx)
}

// RecordEarnings is called when a ride is completed (usually via trigger, but can be manual).
func (s *RiderWalletService) RecordEarnings(ctx context.Context, rideID int64, riderID int64, earningsCents int) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		UPDATE rides
		SET rider_earnings_cents = $1
		WHERE ride_id = $2 AND rider_id = $3
	`, earningsCents, rideID, riderID)
	if err != nil {
		return fmt.Errorf("failed to update ride earnings: %w", err)
	}

	return tx.Commit(ctx)
}

// GetPerformanceMetrics retrieves rider performance stats.
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
		if err == pgx.ErrNoRows {
			if initErr := s.CreateInitialRiderRecords(ctx, riderID); initErr != nil {
				return nil, fmt.Errorf("failed to initialize rider performance metrics: %w", initErr)
			}
			err = s.db.QueryRow(ctx, query, riderID).Scan(
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
		} else {
			return nil, fmt.Errorf("failed to get performance metrics: %w", err)
		}
	}

	earningsQuery := `
		SELECT COALESCE(SUM(amount_cents), 0)
		FROM rider_transactions
		WHERE rider_id = $1 
		  AND transaction_type = 'ride_earning' 
		  AND created_at >= CURRENT_DATE
	`

	err = s.db.QueryRow(ctx, earningsQuery, riderID).Scan(&metrics.TodayEarnedCents)
	if err != nil {
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
		if item.AmountCents < 0 {
			item.AmountCents = -item.AmountCents
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

// IncrementOffersReceived updates the offer counter when a new ride offer is sent.
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

func validateAndNormalizeEmergencyContactInput(fullName, phoneNumber string) (string, error) {
	name := strings.TrimSpace(fullName)
	if name == "" || len(name) < 2 || len(name) > 120 {
		return "", fmt.Errorf("full_name must be between 2 and 120 characters")
	}

	normalizedPhone, err := normalizePhilippinePhone(phoneNumber)
	if err != nil {
		return "", err
	}
	return normalizedPhone, nil
}

func normalizePhilippinePhone(phoneNumber string) (string, error) {
	cleaner := regexp.MustCompile(`[^\d+]`)
	clean := cleaner.ReplaceAllString(strings.TrimSpace(phoneNumber), "")

	switch {
	case strings.HasPrefix(clean, "+639") && len(clean) == 13:
		return clean, nil
	case strings.HasPrefix(clean, "639") && len(clean) == 12:
		return "+" + clean, nil
	case strings.HasPrefix(clean, "09") && len(clean) == 11:
		return "+63" + clean[1:], nil
	default:
		return "", fmt.Errorf("phone_number must be a valid Philippine mobile number")
	}
}

func ensureRiderRecordsInTx(ctx context.Context, tx db.DBTX, riderID int64) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO rider_wallets (rider_id, balance_cents, total_earned_cents, total_withdrawn_cents, created_at, updated_at)
		VALUES ($1, 0, 0, 0, NOW(), NOW())
		ON CONFLICT (rider_id) DO NOTHING
	`, riderID)
	if err != nil {
		return fmt.Errorf("failed to initialize rider wallet: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO rider_performance_metrics (
			rider_id, total_offers_received, total_rides_accepted, total_rides_completed, total_rides_cancelled,
			acceptance_rate, completion_rate, average_rating, total_ratings, rating_sum, updated_at
		)
		VALUES ($1, 0, 0, 0, 0, 0, 0, NULL, 0, 0, NOW())
		ON CONFLICT (rider_id) DO NOTHING
	`, riderID)
	if err != nil {
		return fmt.Errorf("failed to initialize rider performance metrics: %w", err)
	}
	return nil
}

func ensureDefaultPayoutMethodInTx(ctx context.Context, tx db.DBTX, riderID int64) error {
	return ensureDefaultPayoutMethodInTxWithExclusion(ctx, tx, riderID, nil)
}

func ensureDefaultPayoutMethodInTxWithExclusion(ctx context.Context, tx db.DBTX, riderID int64, excludeMethodID *int) error {
	var count int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM rider_payout_methods WHERE rider_id = $1`, riderID).Scan(&count); err != nil {
		return fmt.Errorf("failed to count payout methods: %w", err)
	}
	if count == 0 {
		return nil
	}

	var defaultCount int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM rider_payout_methods WHERE rider_id = $1 AND is_default = TRUE`, riderID).Scan(&defaultCount); err != nil {
		return fmt.Errorf("failed to count default payout methods: %w", err)
	}
	if defaultCount > 0 {
		return nil
	}

	query := `
		UPDATE rider_payout_methods
		SET is_default = TRUE, updated_at = NOW()
		WHERE id = (
			SELECT id FROM rider_payout_methods
			WHERE rider_id = $1
	`
	args := []interface{}{riderID}
	if excludeMethodID != nil {
		query += ` AND id <> $2`
		args = append(args, *excludeMethodID)
	}
	query += `
			ORDER BY updated_at DESC, id DESC
			LIMIT 1
		)
	`
	cmd, err := tx.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to enforce default payout method: %w", err)
	}
	if cmd.RowsAffected() == 0 && excludeMethodID != nil {
		return ensureDefaultPayoutMethodInTxWithExclusion(ctx, tx, riderID, nil)
	}
	return nil
}

func ensurePrimaryEmergencyContactInTx(ctx context.Context, tx db.DBTX, riderID int64) error {
	return ensurePrimaryEmergencyContactInTxWithExclusion(ctx, tx, riderID, nil)
}

func ensurePrimaryEmergencyContactInTxWithExclusion(ctx context.Context, tx db.DBTX, riderID int64, excludeContactID *int) error {
	var count int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM rider_emergency_contacts WHERE rider_id = $1`, riderID).Scan(&count); err != nil {
		return fmt.Errorf("failed to count emergency contacts: %w", err)
	}
	if count == 0 {
		return nil
	}

	var primaryCount int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM rider_emergency_contacts WHERE rider_id = $1 AND is_primary = TRUE`, riderID).Scan(&primaryCount); err != nil {
		return fmt.Errorf("failed to count primary emergency contacts: %w", err)
	}
	if primaryCount > 0 {
		return nil
	}

	query := `
		UPDATE rider_emergency_contacts
		SET is_primary = TRUE, updated_at = NOW()
		WHERE contact_id = (
			SELECT contact_id FROM rider_emergency_contacts
			WHERE rider_id = $1
	`
	args := []interface{}{riderID}
	if excludeContactID != nil {
		query += ` AND contact_id <> $2`
		args = append(args, *excludeContactID)
	}
	query += `
			ORDER BY updated_at DESC, contact_id DESC
			LIMIT 1
		)
	`
	cmd, err := tx.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to enforce primary emergency contact: %w", err)
	}
	if cmd.RowsAffected() == 0 && excludeContactID != nil {
		return ensurePrimaryEmergencyContactInTxWithExclusion(ctx, tx, riderID, nil)
	}
	return nil
}
