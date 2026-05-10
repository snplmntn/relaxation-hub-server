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

func TestUserRepo_CreateAndRetrieve(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	pool := testhelpers.SetupTestDB(t)

	testhelpers.WithTransaction(t, pool, func(tx pgx.Tx) {
		repo := repository.NewUserRepository(tx)
		ctx := context.Background()

		// Create User & Identity
		timestamp := time.Now().UnixNano()
		user := model.User{
			FullName:     "Integration User",
			Role:         "client",
			PrimaryEmail: fmt.Sprintf("int+%d@test.com", timestamp),
			PrimaryPhone: fmt.Sprintf("+6399%012d", timestamp%1000000000000),
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
		identity := model.UserAuthIdentity{
			Provider:     "email",
			ProviderKey:  user.PrimaryEmail,
			PasswordHash: "hashed_secret",
			CreatedAt:    time.Now(),
		}

		err := repo.CreateUserAndIdentity(ctx, user, identity)
		require.NoError(t, err)

		// Find Identity
		foundIdentity, err := repo.FindIdentityByKey(ctx, "email", user.PrimaryEmail)
		require.NoError(t, err)
		assert.Equal(t, identity.Provider, foundIdentity.Provider)
		assert.NotZero(t, foundIdentity.UserID)

		// Find User By ID
		foundUser, err := repo.FindUserByID(ctx, int(foundIdentity.UserID))
		require.NoError(t, err)
		assert.Equal(t, user.FullName, foundUser.FullName)
		assert.Equal(t, user.PrimaryEmail, foundUser.PrimaryEmail)
	})
}

func TestUserRepo_Blocking(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	pool := testhelpers.SetupTestDB(t)

	testhelpers.WithTransaction(t, pool, func(tx pgx.Tx) {
		repo := repository.NewUserRepository(tx)
		ctx := context.Background()

		// Create test users directly in the transaction
		var id1, id2 int64
		err := tx.QueryRow(ctx, `INSERT INTO users (primary_phone, role, full_name) VALUES ('+639111111111', 'client', 'Blocker User') RETURNING user_id`).Scan(&id1)
		require.NoError(t, err)

		err = tx.QueryRow(ctx, `INSERT INTO users (primary_phone, role, full_name) VALUES ('+639222222222', 'client', 'Blocked User') RETURNING user_id`).Scan(&id2)
		require.NoError(t, err)

		// Block
		err = repo.BlockUser(ctx, id1, id2)
		require.NoError(t, err)

		// Check IsBlocked
		blocked, err := repo.IsBlocked(ctx, id1, id2)
		require.NoError(t, err)
		assert.True(t, blocked)

		// Reverse check
		blocked, err = repo.IsBlocked(ctx, id2, id1)
		require.NoError(t, err)
		assert.True(t, blocked)

		// Unblock
		err = repo.UnblockUser(ctx, id1, id2)
		require.NoError(t, err)

		// Check again
		blocked, err = repo.IsBlocked(ctx, id1, id2)
		require.NoError(t, err)
		assert.False(t, blocked)
	})
}
