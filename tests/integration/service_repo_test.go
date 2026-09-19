package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"github.com/snplmntn/relaxation-hub-server/tests/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceRepo_CreateAndGet(t *testing.T) {
	pool := SetupTestDB(t)

	testhelpers.WithTransaction(t, pool, func(tx pgx.Tx) {
		repo := repository.NewServiceRepository(tx)
		ctx := context.Background()

		svc := &model.Service{
			Name:                "Deep Tissue Massage",
			Description:         "Therapeutic massage",
			BasePrice:           1200.00,
			DurationMinutes:     60,
			Category:            "Massage",
			IsActive:            true,
			TherapistCommission: float64Ptr(800.00),
		}

		// Create
		err := repo.Create(ctx, svc)
		require.NoError(t, err)
		assert.NotZero(t, svc.ServiceID)

		// GetByID
		retrieved, err := repo.GetByID(ctx, svc.ServiceID)
		require.NoError(t, err)
		assert.Equal(t, svc.Name, retrieved.Name)
		assert.Equal(t, svc.BasePrice, retrieved.BasePrice)
		assert.Equal(t, svc.IsActive, retrieved.IsActive)

		// GetByIDs
		batch, err := repo.GetByIDs(ctx, []int64{svc.ServiceID})
		require.NoError(t, err)
		assert.Len(t, batch, 1)
		assert.Equal(t, svc.ServiceID, batch[0].ServiceID)
	})
}

func TestServiceRepo_Update(t *testing.T) {
	pool := SetupTestDB(t)

	testhelpers.WithTransaction(t, pool, func(tx pgx.Tx) {
		repo := repository.NewServiceRepository(tx)
		ctx := context.Background()

		svc := &model.Service{
			Name:            "Swedish Massage",
			BasePrice:       1000.00,
			DurationMinutes: 60,
			Category:        "Massage",
			IsActive:        true,
		}
		require.NoError(t, repo.Create(ctx, svc))

		// Update
		updates := map[string]interface{}{
			"name":       "Premium Swedish Massage",
			"base_price": 1500.00,
		}
		err := repo.Update(ctx, svc.ServiceID, updates)
		require.NoError(t, err)

		// Verify
		updated, err := repo.GetByID(ctx, svc.ServiceID)
		require.NoError(t, err)
		assert.Equal(t, "Premium Swedish Massage", updated.Name)
		assert.Equal(t, 1500.00, updated.BasePrice)
	})
}

func TestServiceRepo_Delete(t *testing.T) {
	pool := SetupTestDB(t)

	testhelpers.WithTransaction(t, pool, func(tx pgx.Tx) {
		repo := repository.NewServiceRepository(tx)
		ctx := context.Background()

		svc := &model.Service{
			Name:            "To Delete",
			BasePrice:       500,
			DurationMinutes: 30,
			Category:        "Addon",
			IsActive:        true,
		}
		require.NoError(t, repo.Create(ctx, svc))

		// Delete
		err := repo.Delete(ctx, svc.ServiceID)
		require.NoError(t, err)

		// Verify retrieval fails
		_, err = repo.GetByID(ctx, svc.ServiceID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no rows in result set")
	})
}

func TestServiceRepo_ListActiveAndUnavailable(t *testing.T) {
	pool := SetupTestDB(t)

	testhelpers.WithTransaction(t, pool, func(tx pgx.Tx) {
		repo := repository.NewServiceRepository(tx)
		ctx := context.Background()

		// Setup: 1 Active, 1 Inactive, 1 Deleted
		active := &model.Service{Name: "Active Svc", IsActive: true, BasePrice: 100, DurationMinutes: 60, Category: "Test"}
		require.NoError(t, repo.Create(ctx, active))

		inactive := &model.Service{Name: "Inactive Svc", IsActive: false, BasePrice: 100, DurationMinutes: 60, Category: "Test"}
		require.NoError(t, repo.Create(ctx, inactive))

		deleted := &model.Service{Name: "Deleted Svc", IsActive: true, BasePrice: 100, DurationMinutes: 60, Category: "Test"}
		require.NoError(t, repo.Create(ctx, deleted))
		require.NoError(t, repo.Delete(ctx, deleted.ServiceID))

		// List Active
		actives, err := repo.ListActive(ctx)
		require.NoError(t, err)

		// The DB might contain other seeded data, so we check if our active service exists
		foundActive := false
		for _, s := range actives {
			if s.ServiceID == active.ServiceID {
				foundActive = true
			}
			if s.ServiceID == inactive.ServiceID {
				t.Error("Inactive service found in ListActive")
			}
			if s.ServiceID == deleted.ServiceID {
				t.Error("Deleted service found in ListActive")
			}
		}
		assert.True(t, foundActive, "Active service should be in ListActive")

		// List Unavailable
		unavail, err := repo.ListUnavailable(ctx)
		require.NoError(t, err)

		foundInactive := false
		for _, s := range unavail {
			if s.ServiceID == inactive.ServiceID {
				foundInactive = true
			}
			if s.ServiceID == active.ServiceID {
				t.Error("Active service found in ListUnavailable")
			}
		}
		assert.True(t, foundInactive, "Inactive service should be in ListUnavailable")
	})
}

func TestServiceRepo_ListRecentByUserDeduplicatesBeforeLimit(t *testing.T) {
	pool := SetupTestDB(t)

	testhelpers.WithTransaction(t, pool, func(tx pgx.Tx) {
		repo := repository.NewServiceRepository(tx)
		ctx := context.Background()
		clientID := createServiceRepoUser(t, ctx, tx, "client")
		now := time.Now().UTC().Truncate(time.Second)

		first := createServiceRepoService(t, ctx, repo, "Recent First", true)
		second := createServiceRepoService(t, ctx, repo, "Recent Second", true)
		third := createServiceRepoService(t, ctx, repo, "Recent Third", true)
		fourth := createServiceRepoService(t, ctx, repo, "Recent Fourth", true)

		insertServiceRepoBooking(t, ctx, tx, clientID, first.ServiceID, "completed", now.Add(-1*time.Hour))
		insertServiceRepoBooking(t, ctx, tx, clientID, second.ServiceID, "completed", now.Add(-90*time.Minute))
		insertServiceRepoBooking(t, ctx, tx, clientID, second.ServiceID, "completed", now.Add(-2*time.Hour))
		insertServiceRepoBooking(t, ctx, tx, clientID, third.ServiceID, "completed", now.Add(-3*time.Hour))
		insertServiceRepoBooking(t, ctx, tx, clientID, fourth.ServiceID, "completed", now.Add(-4*time.Hour))

		services, err := repo.ListRecentByUser(ctx, clientID)
		require.NoError(t, err)

		assert.Equal(t, []int64{first.ServiceID, second.ServiceID, third.ServiceID}, serviceRepoServiceIDs(services))
	})
}

func TestServiceRepo_ListRecentByUserFiltersOldAndDeletedServices(t *testing.T) {
	pool := SetupTestDB(t)

	testhelpers.WithTransaction(t, pool, func(tx pgx.Tx) {
		repo := repository.NewServiceRepository(tx)
		ctx := context.Background()
		clientID := createServiceRepoUser(t, ctx, tx, "client")
		now := time.Now().UTC().Truncate(time.Second)

		first := createServiceRepoService(t, ctx, repo, "Recent Filter First", true)
		second := createServiceRepoService(t, ctx, repo, "Recent Filter Second", true)
		old := createServiceRepoService(t, ctx, repo, "Recent Filter Old", true)
		deleted := createServiceRepoService(t, ctx, repo, "Recent Filter Deleted", true)
		require.NoError(t, repo.Delete(ctx, deleted.ServiceID))

		insertServiceRepoBooking(t, ctx, tx, clientID, deleted.ServiceID, "completed", now.Add(-30*time.Minute))
		insertServiceRepoBooking(t, ctx, tx, clientID, first.ServiceID, "completed", now.Add(-1*time.Hour))
		insertServiceRepoBooking(t, ctx, tx, clientID, second.ServiceID, "completed", now.Add(-2*time.Hour))
		insertServiceRepoBooking(t, ctx, tx, clientID, old.ServiceID, "completed", now.Add(-31*24*time.Hour))

		services, err := repo.ListRecentByUser(ctx, clientID)
		require.NoError(t, err)

		assert.Equal(t, []int64{first.ServiceID, second.ServiceID}, serviceRepoServiceIDs(services))
	})
}

func TestServiceRepo_ListPopular(t *testing.T) {
	pool := SetupTestDB(t)

	testhelpers.WithTransaction(t, pool, func(tx pgx.Tx) {
		repo := repository.NewServiceRepository(tx)
		ctx := context.Background()
		clientID := createServiceRepoUser(t, ctx, tx, "client")
		now := time.Now().UTC().Truncate(time.Second)

		first := createServiceRepoService(t, ctx, repo, "Popular First", true)
		second := createServiceRepoService(t, ctx, repo, "Popular Second", true)
		third := createServiceRepoService(t, ctx, repo, "Popular Third", true)
		fourth := createServiceRepoService(t, ctx, repo, "Popular Fourth", true)
		pendingOnly := createServiceRepoService(t, ctx, repo, "Popular Pending Only", true)
		oldOnly := createServiceRepoService(t, ctx, repo, "Popular Old Only", true)
		inactive := createServiceRepoService(t, ctx, repo, "Popular Inactive", false)

		insertServiceRepoBookings(t, ctx, tx, clientID, first.ServiceID, "completed", 120, now.Add(-1*24*time.Hour))
		insertServiceRepoBookings(t, ctx, tx, clientID, second.ServiceID, "completed", 110, now.Add(-2*24*time.Hour))
		insertServiceRepoBookings(t, ctx, tx, clientID, third.ServiceID, "completed", 100, now.Add(-3*24*time.Hour))
		insertServiceRepoBookings(t, ctx, tx, clientID, fourth.ServiceID, "completed", 90, now.Add(-4*24*time.Hour))
		insertServiceRepoBookings(t, ctx, tx, clientID, pendingOnly.ServiceID, "pending", 130, now.Add(-1*24*time.Hour))
		insertServiceRepoBookings(t, ctx, tx, clientID, oldOnly.ServiceID, "completed", 140, now.Add(-45*24*time.Hour))
		insertServiceRepoBookings(t, ctx, tx, clientID, inactive.ServiceID, "completed", 150, now.Add(-1*24*time.Hour))

		services, err := repo.ListPopular(ctx)
		require.NoError(t, err)

		assert.Equal(t, []int64{first.ServiceID, second.ServiceID, third.ServiceID}, serviceRepoServiceIDs(services))
	})
}

func createServiceRepoService(t *testing.T, ctx context.Context, repo repository.ServiceRepository, name string, isActive bool) *model.Service {
	t.Helper()

	svc := &model.Service{Name: name, IsActive: isActive, BasePrice: 100, DurationMinutes: 60, Category: "Test"}
	require.NoError(t, repo.Create(ctx, svc))

	return svc
}

func createServiceRepoUser(t *testing.T, ctx context.Context, tx pgx.Tx, role string) int64 {
	t.Helper()

	var userID int64
	suffix := time.Now().UnixNano()
	err := tx.QueryRow(ctx, `
		INSERT INTO users (full_name, role, primary_email)
		VALUES ($1, $2, $3)
		RETURNING user_id
	`, fmt.Sprintf("Service Repo %s User", role), role, fmt.Sprintf("service-repo-%s-%d@example.test", role, suffix)).Scan(&userID)
	require.NoError(t, err)

	return userID
}

func insertServiceRepoBooking(t *testing.T, ctx context.Context, tx pgx.Tx, clientID, serviceID int64, status string, createdAt time.Time) {
	t.Helper()

	_, err := tx.Exec(ctx, `
		INSERT INTO bookings (client_id, service_id, payment_method, status, raw_total, final_total, duration_minutes, created_at)
		VALUES ($1, $2, 'cash', $3, 100, 100, 60, $4)
	`, clientID, serviceID, status, createdAt)
	require.NoError(t, err)
}

func insertServiceRepoBookings(t *testing.T, ctx context.Context, tx pgx.Tx, clientID, serviceID int64, status string, count int, createdAt time.Time) {
	t.Helper()

	_, err := tx.Exec(ctx, `
		INSERT INTO bookings (client_id, service_id, payment_method, status, raw_total, final_total, duration_minutes, created_at)
		SELECT $1, $2, 'cash', $3, 100, 100, 60, $4
		FROM generate_series(1, $5)
	`, clientID, serviceID, status, createdAt, count)
	require.NoError(t, err)
}

func serviceRepoServiceIDs(services []model.Service) []int64 {
	ids := make([]int64, 0, len(services))
	for _, service := range services {
		ids = append(ids, service.ServiceID)
	}
	return ids
}
