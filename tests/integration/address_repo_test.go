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

// mockUserRepo is a mock implementation of repository.UserRepository for testing purposes.
// This struct and its methods are added here as per the user's instruction,
// although typically mocks would reside in a separate mock file.
type mockUserRepo struct {
	CreateUserAndIdentityFunc func(ctx context.Context, user model.User, identity model.UserAuthIdentity) error
	FindIdentityByKeyFunc     func(ctx context.Context, provider, key string) (*model.UserAuthIdentity, error)
}

func (m *mockUserRepo) CreateUserAndIdentity(ctx context.Context, user model.User, identity model.UserAuthIdentity) error {
	return m.CreateUserAndIdentityFunc(ctx, user, identity)
}

func (m *mockUserRepo) Create(ctx context.Context, user *model.User) error {
	// Simple mock implementation for testing
	user.UserID = 123 // Assign a dummy ID
	return nil
}

func (m *mockUserRepo) FindIdentityByKey(ctx context.Context, provider, key string) (*model.UserAuthIdentity, error) {
	return m.FindIdentityByKeyFunc(ctx, provider, key)
}

func TestAddressRepo_CreateAndRetrieve(t *testing.T) {
	pool := SetupTestDB(t)

	testhelpers.WithTransaction(t, pool, func(tx pgx.Tx) {
		repo := repository.NewAddressRepository(tx)
		userRepo := repository.NewUserRepository(tx)
		ctx := context.Background()

		// 1. Create a test user first (foreign key constraint)
		user := &model.User{
			FullName:     "Address Tester",
			PrimaryEmail: testhelpers.RandomEmail("addresstester"),
			Role:         "client",
		}
		err := userRepo.Create(ctx, user)
		require.NoError(t, err)

		// 2. Create an address
		lat := 14.5995
		lon := 120.9842

		addr := &model.Address{
			UserID:     int64(user.UserID),
			Label:      "Home",
			Street:     "123 Main St",
			Barangay:   "Brgy 1",
			City:       "Manila",
			Province:   "Metro Manila",
			PostalCode: "1000",
			Country:    "Philippines",
			Latitude:   &lat,
			Longitude:  &lon,
			IsDefault:  true,
		}

		err = repo.Create(ctx, addr)
		require.NoError(t, err)
		assert.NotZero(t, addr.AddressID)
		assert.True(t, addr.IsDefault)

		// 3. Retrieve
		retrieved, err := repo.GetByID(ctx, addr.AddressID, int64(user.UserID))
		require.NoError(t, err)
		assert.Equal(t, addr.Street, retrieved.Street)
		assert.Equal(t, addr.City, retrieved.City)
		assert.NotNil(t, retrieved.Latitude)
		assert.Equal(t, lat, *retrieved.Latitude)
	})
}

func TestAddressRepo_SetDefaultLogic(t *testing.T) {
	pool := SetupTestDB(t)

	testhelpers.WithTransaction(t, pool, func(tx pgx.Tx) {
		repo := repository.NewAddressRepository(tx)
		userRepo := repository.NewUserRepository(tx)
		ctx := context.Background()

		// Create user
		user := &model.User{
			FullName:     "Default Logic Tester",
			PrimaryEmail: testhelpers.RandomEmail("defaulttester"),
			Role:         "client",
		}
		require.NoError(t, userRepo.Create(ctx, user))

		// Create first address - should be default automatically
		addr1 := &model.Address{UserID: int64(user.UserID), Label: "Addr1", Street: "St 1", City: "City", Country: "PH"}
		require.NoError(t, repo.Create(ctx, addr1))
		assert.True(t, addr1.IsDefault)

		// Create second address - not default
		addr2 := &model.Address{UserID: int64(user.UserID), Label: "Addr2", Street: "St 2", City: "City", Country: "PH"}
		require.NoError(t, repo.Create(ctx, addr2))
		assert.False(t, addr2.IsDefault)

		// Set second as default
		err := repo.SetDefault(ctx, addr2.AddressID, int64(user.UserID))
		require.NoError(t, err)

		// Verify swap
		a1, _ := repo.GetByID(ctx, addr1.AddressID, int64(user.UserID))
		a2, _ := repo.GetByID(ctx, addr2.AddressID, int64(user.UserID))

		assert.False(t, a1.IsDefault)
		assert.True(t, a2.IsDefault)
	})
}

func TestAddressRepo_SoftDelete(t *testing.T) {
	pool := SetupTestDB(t)

	testhelpers.WithTransaction(t, pool, func(tx pgx.Tx) {
		repo := repository.NewAddressRepository(tx)
		userRepo := repository.NewUserRepository(tx)
		ctx := context.Background()

		user := &model.User{
			FullName:     "Delete Tester",
			PrimaryEmail: testhelpers.RandomEmail("deletetester"),
			Role:         "client",
		}
		require.NoError(t, userRepo.Create(ctx, user))

		addr := &model.Address{UserID: int64(user.UserID), Label: "To Delete", Street: "St", City: "City", Country: "PH"}
		require.NoError(t, repo.Create(ctx, addr))

		// Delete
		err := repo.SoftDelete(ctx, addr.AddressID, int64(user.UserID))
		require.NoError(t, err)

		// Verify retrieval fails
		_, err = repo.GetByID(ctx, addr.AddressID, int64(user.UserID))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no rows in result set")

		// Verify list excludes it
		list, err := repo.ListForUser(ctx, int64(user.UserID), false)
		require.NoError(t, err)
		assert.Len(t, list, 0)
	})
}
