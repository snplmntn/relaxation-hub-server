package integration

import (
	"context"
	"testing"

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
