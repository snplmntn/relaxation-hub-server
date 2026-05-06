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

func TestTherapistRepo_CreateAndGet(t *testing.T) {
	pool := SetupTestDB(t)

	testhelpers.WithTransaction(t, pool, func(tx pgx.Tx) {
		repo := repository.NewTherapistRepository(tx)
		userRepo := repository.NewUserRepository(tx)
		ctx := context.Background()

		// 1. Create Base User
		user := &model.User{
			FullName:     "Therapist Tester",
			PrimaryEmail: testhelpers.RandomEmail("therapist"),
			Role:         "therapist",
		}
		require.NoError(t, userRepo.Create(ctx, user))

		// 2. Create Therapist Profile (creates empty profile)
		err := repo.CreateProfile(ctx, int64(user.UserID))
		require.NoError(t, err)

		// 3. Update Profile with details
		bio := "Expert massage therapist"
		yearsExp := 5
		updates := map[string]interface{}{
			"bio":                bio,
			"years_experience":   yearsExp,
			"is_verified":        true,
			"accept_assignments": true,
		}
		err = repo.UpdateProfile(ctx, int64(user.UserID), updates)
		require.NoError(t, err)

		// 4. Retrieve and Verify
		retrieved, err := repo.GetProfile(ctx, int64(user.UserID))
		require.NoError(t, err)
		assert.Equal(t, bio, *retrieved.Bio)
		assert.Equal(t, yearsExp, *retrieved.YearsExperience)
		assert.True(t, retrieved.IsVerified) // Assuming DB stores/returns bool correctly or test setup handles it
	})
}

func TestTherapistRepo_Update(t *testing.T) {
	pool := SetupTestDB(t)

	testhelpers.WithTransaction(t, pool, func(tx pgx.Tx) {
		repo := repository.NewTherapistRepository(tx)
		userRepo := repository.NewUserRepository(tx)
		ctx := context.Background()

		// Setup
		user := &model.User{
			FullName:     "Update Tester",
			PrimaryEmail: testhelpers.RandomEmail("update_therapist"),
			Role:         "therapist",
		}
		require.NoError(t, userRepo.Create(ctx, user))
		require.NoError(t, repo.CreateProfile(ctx, int64(user.UserID)))

		// Update
		newBio := "Updated bio"
		newYears := 10
		updates := map[string]interface{}{
			"bio":              newBio,
			"years_experience": newYears,
		}
		err := repo.UpdateProfile(ctx, int64(user.UserID), updates)
		require.NoError(t, err)

		// Verify
		updated, err := repo.GetProfile(ctx, int64(user.UserID))
		require.NoError(t, err)
		assert.Equal(t, newBio, *updated.Bio)
		assert.Equal(t, newYears, *updated.YearsExperience)
	})
}

func TestTherapistRepo_ServiceManagement(t *testing.T) {
	pool := SetupTestDB(t)

	testhelpers.WithTransaction(t, pool, func(tx pgx.Tx) {
		repo := repository.NewTherapistRepository(tx)
		userRepo := repository.NewUserRepository(tx)
		ctx := context.Background()

		// Setup User
		user := &model.User{
			FullName:     "Service Tester",
			PrimaryEmail: testhelpers.RandomEmail("service_therapist"),
			Role:         "therapist",
		}
		require.NoError(t, userRepo.Create(ctx, user))

		// Create Profile
		require.NoError(t, repo.CreateProfile(ctx, int64(user.UserID)))

		// Create a Service manually
		var serviceID int64
		err := tx.QueryRow(ctx, `INSERT INTO services (name, description, duration_minutes, base_price, category) VALUES ($1, $2, $3, $4, $5) RETURNING service_id`,
			"Test Massage", "Desc", 60, 500, "Massage").Scan(&serviceID)
		require.NoError(t, err)

		// Add Service to Therapist
		ts := &model.TherapistService{
			TherapistID:  int64(user.UserID),
			ServiceID:    serviceID,
			SupportsSoft: true,
		}
		err = repo.AddService(ctx, ts)
		require.NoError(t, err)

		// List Services - uses GetServices which returns []int64 per interface
		serviceIDs, err := repo.GetServices(ctx, int64(user.UserID))
		require.NoError(t, err)
		assert.Len(t, serviceIDs, 1)
		assert.Equal(t, serviceID, serviceIDs[0])

		// Remove Service
		err = repo.RemoveService(ctx, int64(user.UserID), serviceID)
		require.NoError(t, err)

		// Verify Empty
		serviceIDs, err = repo.GetServices(ctx, int64(user.UserID))
		require.NoError(t, err)
		assert.Len(t, serviceIDs, 0)
	})
}
