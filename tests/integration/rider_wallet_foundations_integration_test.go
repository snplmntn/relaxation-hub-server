package integration

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
	"github.com/snplmntn/relaxation-hub-server/tests/testhelpers"
	"github.com/stretchr/testify/require"
)

func TestIntegration_RiderWalletAndPerformanceReads_WithMigration(t *testing.T) {
	pool := SetupTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	tx, cleanup, err := testhelpers.BeginTestTx(context.Background(), pool)
	require.NoError(t, err)
	defer cleanup()

	applyRiderWalletFoundationsMigration(t, tx)

	riderID, err := testhelpers.CreateTestUser(context.Background(), tx, "Rider Wallet User", uniqueTestEmail("rider.wallet"), "rider")
	require.NoError(t, err)

	svc := service.NewRiderWalletService(tx)
	require.NoError(t, svc.CreateInitialRiderRecords(context.Background(), int64(riderID)))

	wallet, err := svc.GetWallet(context.Background(), int64(riderID))
	require.NoError(t, err)
	require.Equal(t, int64(riderID), wallet.RiderID)
	require.Equal(t, 0, wallet.BalanceCents)

	metrics, err := svc.GetPerformanceMetrics(context.Background(), int64(riderID))
	require.NoError(t, err)
	require.Equal(t, int64(riderID), metrics.RiderID)
	require.Equal(t, 0, metrics.TodayEarnedCents)
}

func TestIntegration_RiderPayoutMethodCRUD_WithMigration(t *testing.T) {
	pool := SetupTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	tx, cleanup, err := testhelpers.BeginTestTx(context.Background(), pool)
	require.NoError(t, err)
	defer cleanup()

	applyRiderWalletFoundationsMigration(t, tx)

	riderID, err := testhelpers.CreateTestUser(context.Background(), tx, "Rider Payout User", uniqueTestEmail("rider.payout"), "rider")
	require.NoError(t, err)

	svc := service.NewRiderWalletService(tx)

	method := &model.RiderPayoutMethod{
		RiderID:       int64(riderID),
		MethodType:    "gcash",
		ProviderName:  "GCash",
		AccountNumber: "09171234567",
		AccountName:   "Rider One",
		IsDefault:     false,
	}
	require.NoError(t, svc.AddPayoutMethod(context.Background(), method))
	require.NotZero(t, method.ID)

	methods, err := svc.GetPayoutMethods(context.Background(), int64(riderID))
	require.NoError(t, err)
	require.Len(t, methods, 1)
	require.True(t, methods[0].IsDefault)

	method.MethodType = "bank"
	method.ProviderName = "BPI"
	method.AccountNumber = "1234567890"
	method.AccountName = "Rider One Updated"
	method.IsDefault = true
	require.NoError(t, svc.UpdatePayoutMethod(context.Background(), int64(riderID), method))

	methods, err = svc.GetPayoutMethods(context.Background(), int64(riderID))
	require.NoError(t, err)
	require.Len(t, methods, 1)
	require.Equal(t, "BPI", methods[0].ProviderName)
	require.Equal(t, "bank", methods[0].MethodType)

	require.NoError(t, svc.DeletePayoutMethod(context.Background(), int64(riderID), method.ID))

	methods, err = svc.GetPayoutMethods(context.Background(), int64(riderID))
	require.NoError(t, err)
	require.Len(t, methods, 0)
}

func TestIntegration_RiderSafetyContactCRUD_WithMigration(t *testing.T) {
	pool := SetupTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	tx, cleanup, err := testhelpers.BeginTestTx(context.Background(), pool)
	require.NoError(t, err)
	defer cleanup()

	applyRiderWalletFoundationsMigration(t, tx)

	riderID, err := testhelpers.CreateTestUser(context.Background(), tx, "Rider Safety User", uniqueTestEmail("rider.safety"), "rider")
	require.NoError(t, err)

	svc := service.NewRiderWalletService(tx)
	relationship := "Sibling"
	contact := &model.RiderEmergencyContact{
		RiderID:      int64(riderID),
		FullName:     "Emergency Contact",
		PhoneNumber:  "09175551234",
		Relationship: &relationship,
		IsPrimary:    false,
	}
	require.NoError(t, svc.AddEmergencyContact(context.Background(), contact))
	require.NotZero(t, contact.ContactID)

	contacts, err := svc.GetEmergencyContacts(context.Background(), int64(riderID))
	require.NoError(t, err)
	require.Len(t, contacts, 1)
	require.Equal(t, "Emergency Contact", contacts[0].FullName)

	updatedRelationship := "Spouse"
	contact.FullName = "Primary Contact"
	contact.PhoneNumber = "09170000000"
	contact.Relationship = &updatedRelationship
	contact.IsPrimary = true
	require.NoError(t, svc.UpdateEmergencyContact(context.Background(), int64(riderID), contact))

	contacts, err = svc.GetEmergencyContacts(context.Background(), int64(riderID))
	require.NoError(t, err)
	require.Len(t, contacts, 1)
	require.Equal(t, "Primary Contact", contacts[0].FullName)
	require.True(t, contacts[0].IsPrimary)

	require.NoError(t, svc.DeleteEmergencyContact(context.Background(), int64(riderID), contact.ContactID))
	contacts, err = svc.GetEmergencyContacts(context.Background(), int64(riderID))
	require.NoError(t, err)
	require.Len(t, contacts, 0)
}

func TestIntegration_RiderWalletReadsSelfHealMissingRows(t *testing.T) {
	pool := SetupTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	tx, cleanup, err := testhelpers.BeginTestTx(context.Background(), pool)
	require.NoError(t, err)
	defer cleanup()

	applyRiderWalletFoundationsMigration(t, tx)

	riderID, err := testhelpers.CreateTestUser(context.Background(), tx, "Rider Self Heal", uniqueTestEmail("rider.self.heal"), "rider")
	require.NoError(t, err)

	svc := service.NewRiderWalletService(tx)

	wallet, err := svc.GetWallet(context.Background(), int64(riderID))
	require.NoError(t, err)
	require.Equal(t, int64(riderID), wallet.RiderID)

	metrics, err := svc.GetPerformanceMetrics(context.Background(), int64(riderID))
	require.NoError(t, err)
	require.Equal(t, int64(riderID), metrics.RiderID)
}

func TestIntegration_RiderPayoutReservationAndApprovalSafety(t *testing.T) {
	pool := SetupTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	tx, cleanup, err := testhelpers.BeginTestTx(context.Background(), pool)
	require.NoError(t, err)
	defer cleanup()

	applyRiderWalletFoundationsMigration(t, tx)

	riderID, err := testhelpers.CreateTestUser(context.Background(), tx, "Rider Payout Safety", uniqueTestEmail("rider.payout.safety"), "rider")
	require.NoError(t, err)
	rid := int64(riderID)

	_, err = tx.Exec(context.Background(), `
		INSERT INTO rider_wallets (rider_id, balance_cents, total_earned_cents, total_withdrawn_cents)
		VALUES ($1, 20000, 20000, 0)
		ON CONFLICT (rider_id) DO UPDATE SET balance_cents = 20000
	`, rid)
	require.NoError(t, err)

	var methodID int
	err = tx.QueryRow(context.Background(), `
		INSERT INTO rider_payout_methods (rider_id, method_type, provider_name, account_number, account_name, is_default)
		VALUES ($1, 'gcash', 'GCash', '09171234567', 'Payout Rider', true)
		RETURNING id
	`, rid).Scan(&methodID)
	require.NoError(t, err)

	svc := service.NewRiderWalletService(tx)
	require.NoError(t, svc.RequestPayout(context.Background(), rid, 15000, methodID))

	err = svc.RequestPayout(context.Background(), rid, 10000, methodID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "insufficient available balance")

	var txID int
	err = tx.QueryRow(context.Background(), `
		INSERT INTO rider_transactions (rider_id, transaction_type, amount_cents, status, description, payout_method_id)
		VALUES ($1, 'payout', -25000, 'pending', 'oversized pending payout', $2)
		RETURNING transaction_id
	`, rid, methodID).Scan(&txID)
	require.NoError(t, err)

	err = svc.ApprovePayout(context.Background(), txID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "insufficient wallet balance")
}

func TestIntegration_RiderPayoutListUsesPositiveAmounts(t *testing.T) {
	pool := SetupTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	tx, cleanup, err := testhelpers.BeginTestTx(context.Background(), pool)
	require.NoError(t, err)
	defer cleanup()

	applyRiderWalletFoundationsMigration(t, tx)

	riderID, err := testhelpers.CreateTestUser(context.Background(), tx, "Rider List User", uniqueTestEmail("rider.list.user"), "rider")
	require.NoError(t, err)
	rid := int64(riderID)

	var methodID int
	err = tx.QueryRow(context.Background(), `
		INSERT INTO rider_payout_methods (rider_id, method_type, provider_name, account_number, account_name, is_default)
		VALUES ($1, 'gcash', 'GCash', '09179990000', 'List Rider', true)
		RETURNING id
	`, rid).Scan(&methodID)
	require.NoError(t, err)

	_, err = tx.Exec(context.Background(), `
		INSERT INTO rider_transactions (rider_id, transaction_type, amount_cents, status, description, payout_method_id)
		VALUES ($1, 'payout', -12000, 'pending', 'payout request', $2)
	`, rid, methodID)
	require.NoError(t, err)

	svc := service.NewRiderWalletService(tx)
	items, err := svc.ListPendingRiderPayouts(context.Background())
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, 12000, items[0].AmountCents)
	require.Equal(t, 120.0, items[0].AmountPHP)
}

func TestIntegration_DefaultPrimaryUniquenessConstraints(t *testing.T) {
	pool := SetupTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	tx, cleanup, err := testhelpers.BeginTestTx(context.Background(), pool)
	require.NoError(t, err)
	defer cleanup()

	applyRiderWalletFoundationsMigration(t, tx)

	riderID, err := testhelpers.CreateTestUser(context.Background(), tx, "Rider Unique User", uniqueTestEmail("rider.unique.user"), "rider")
	require.NoError(t, err)
	rid := int64(riderID)

	_, err = tx.Exec(context.Background(), `
		INSERT INTO rider_payout_methods (rider_id, method_type, provider_name, account_number, account_name, is_default)
		VALUES ($1, 'gcash', 'GCash', '09170001111', 'Unique One', true)
	`, rid)
	require.NoError(t, err)
	_, err = tx.Exec(context.Background(), `SAVEPOINT before_duplicate_default`)
	require.NoError(t, err)

	_, err = tx.Exec(context.Background(), `
		INSERT INTO rider_payout_methods (rider_id, method_type, provider_name, account_number, account_name, is_default)
		VALUES ($1, 'bank', 'BPI', '123456789', 'Unique Two', true)
	`, rid)
	require.Error(t, err)
	var pgErr *pgconn.PgError
	require.ErrorAs(t, err, &pgErr)
	require.Equal(t, "23505", pgErr.Code)
	_, err = tx.Exec(context.Background(), `ROLLBACK TO SAVEPOINT before_duplicate_default`)
	require.NoError(t, err)

	_, err = tx.Exec(context.Background(), `
		INSERT INTO rider_emergency_contacts (rider_id, full_name, phone_number, is_primary)
		VALUES ($1, 'Primary One', '+639170001111', true)
	`, rid)
	require.NoError(t, err)
	_, err = tx.Exec(context.Background(), `SAVEPOINT before_duplicate_primary`)
	require.NoError(t, err)

	_, err = tx.Exec(context.Background(), `
		INSERT INTO rider_emergency_contacts (rider_id, full_name, phone_number, is_primary)
		VALUES ($1, 'Primary Two', '+639170002222', true)
	`, rid)
	require.Error(t, err)
	require.ErrorAs(t, err, &pgErr)
	require.Equal(t, "23505", pgErr.Code)
	_, err = tx.Exec(context.Background(), `ROLLBACK TO SAVEPOINT before_duplicate_primary`)
	require.NoError(t, err)
}

func TestIntegration_PayoutMethodSiblingPromotion_OnUnsetAndDelete(t *testing.T) {
	pool := SetupTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	tx, cleanup, err := testhelpers.BeginTestTx(context.Background(), pool)
	require.NoError(t, err)
	defer cleanup()

	applyRiderWalletFoundationsMigration(t, tx)

	riderID, err := testhelpers.CreateTestUser(context.Background(), tx, "Rider Payout Sibling", uniqueTestEmail("rider.payout.sibling"), "rider")
	require.NoError(t, err)
	rid := int64(riderID)
	svc := service.NewRiderWalletService(tx)

	first := &model.RiderPayoutMethod{
		RiderID:       rid,
		MethodType:    "gcash",
		ProviderName:  "GCash",
		AccountNumber: "09170010001",
		AccountName:   "First Default",
		IsDefault:     true,
	}
	require.NoError(t, svc.AddPayoutMethod(context.Background(), first))

	second := &model.RiderPayoutMethod{
		RiderID:       rid,
		MethodType:    "bank",
		ProviderName:  "BPI",
		AccountNumber: "1234567890",
		AccountName:   "Second Sibling",
		IsDefault:     false,
	}
	require.NoError(t, svc.AddPayoutMethod(context.Background(), second))

	first.MethodType = "gcash"
	first.ProviderName = "GCash Updated"
	first.AccountNumber = "09170010001"
	first.AccountName = "First Unset"
	first.IsDefault = false
	require.NoError(t, svc.UpdatePayoutMethod(context.Background(), rid, first))

	methods, err := svc.GetPayoutMethods(context.Background(), rid)
	require.NoError(t, err)
	require.Len(t, methods, 2)
	var secondIsDefault bool
	for _, m := range methods {
		if m.ID == second.ID {
			secondIsDefault = m.IsDefault
		}
	}
	require.True(t, secondIsDefault)

	require.NoError(t, svc.DeletePayoutMethod(context.Background(), rid, second.ID))
	methods, err = svc.GetPayoutMethods(context.Background(), rid)
	require.NoError(t, err)
	require.Len(t, methods, 1)
	require.Equal(t, first.ID, methods[0].ID)
	require.True(t, methods[0].IsDefault)
}

func TestIntegration_EmergencyContactSiblingPromotion_OnUnsetAndDelete(t *testing.T) {
	pool := SetupTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	tx, cleanup, err := testhelpers.BeginTestTx(context.Background(), pool)
	require.NoError(t, err)
	defer cleanup()

	applyRiderWalletFoundationsMigration(t, tx)

	riderID, err := testhelpers.CreateTestUser(context.Background(), tx, "Rider Contact Sibling", uniqueTestEmail("rider.contact.sibling"), "rider")
	require.NoError(t, err)
	rid := int64(riderID)
	svc := service.NewRiderWalletService(tx)

	rel1 := "Sibling"
	first := &model.RiderEmergencyContact{
		RiderID:      rid,
		FullName:     "Primary One",
		PhoneNumber:  "09170020001",
		Relationship: &rel1,
		IsPrimary:    true,
	}
	require.NoError(t, svc.AddEmergencyContact(context.Background(), first))

	rel2 := "Parent"
	second := &model.RiderEmergencyContact{
		RiderID:      rid,
		FullName:     "Secondary Two",
		PhoneNumber:  "09170020002",
		Relationship: &rel2,
		IsPrimary:    false,
	}
	require.NoError(t, svc.AddEmergencyContact(context.Background(), second))

	first.FullName = "Primary Unset"
	first.PhoneNumber = "+639170020001"
	first.IsPrimary = false
	require.NoError(t, svc.UpdateEmergencyContact(context.Background(), rid, first))

	contacts, err := svc.GetEmergencyContacts(context.Background(), rid)
	require.NoError(t, err)
	require.Len(t, contacts, 2)
	var secondIsPrimary bool
	for _, c := range contacts {
		if c.ContactID == second.ContactID {
			secondIsPrimary = c.IsPrimary
		}
	}
	require.True(t, secondIsPrimary)

	require.NoError(t, svc.DeleteEmergencyContact(context.Background(), rid, second.ContactID))
	contacts, err = svc.GetEmergencyContacts(context.Background(), rid)
	require.NoError(t, err)
	require.Len(t, contacts, 1)
	require.Equal(t, first.ContactID, contacts[0].ContactID)
	require.True(t, contacts[0].IsPrimary)
}

func applyRiderWalletFoundationsMigration(t *testing.T, d db.DBTX) {
	t.Helper()

	var exists bool
	err := d.QueryRow(context.Background(), `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = 'public'
			  AND table_name = 'rider_wallets'
		)
	`).Scan(&exists)
	require.NoError(t, err)

	if !exists {
		t.Fatalf("expected consolidated test schema to include rider wallet foundations")
	}
}
