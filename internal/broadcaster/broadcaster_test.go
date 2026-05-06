package broadcaster

import (
	"context"
	"errors"
	"testing"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	ws "github.com/snplmntn/relaxation-hub-server/internal/websocket"
	"github.com/stretchr/testify/assert"
)

type stubUserRepo struct {
	repository.UserRepository
	listUsers func(ctx context.Context, role string) ([]model.User, error)
}

func (s *stubUserRepo) ListUsers(ctx context.Context, role string) ([]model.User, error) {
	if s.listUsers != nil {
		return s.listUsers(ctx, role)
	}
	return nil, nil
}

func TestBroadcastToAdmins_IncludesSuperAdmins(t *testing.T) {
	origSendToUsers := sendToUsers
	origHub := hub
	origUserRepo := userRepo
	t.Cleanup(func() {
		sendToUsers = origSendToUsers
		hub = origHub
		userRepo = origUserRepo
	})

	SetHub(ws.NewHub())

	roles := make([]string, 0, 2)
	SetUserRepo(&stubUserRepo{
		listUsers: func(_ context.Context, role string) ([]model.User, error) {
			roles = append(roles, role)
			switch role {
			case "admin":
				return []model.User{{UserID: 101}, {UserID: 102}}, nil
			case "super_admin":
				return []model.User{{UserID: 102}, {UserID: 201}}, nil
			default:
				return nil, nil
			}
		},
	})

	var gotIDs []int64
	sendToUsers = func(userIDs []int64, event string, data interface{}) error {
		gotIDs = append([]int64(nil), userIDs...)
		assert.Equal(t, "booking.updated", event)
		assert.NotNil(t, data)
		return nil
	}

	err := BroadcastToAdmins(context.Background(), "booking:updated", map[string]any{"booking_id": 1})
	assert.NoError(t, err)
	assert.Equal(t, []string{"admin", "super_admin"}, roles)
	assert.ElementsMatch(t, []int64{101, 102, 201}, gotIDs)
}

func TestBroadcastToAdmins_ReturnsErrorWhenSuperAdminLookupFails(t *testing.T) {
	origSendToUsers := sendToUsers
	origHub := hub
	origUserRepo := userRepo
	t.Cleanup(func() {
		sendToUsers = origSendToUsers
		hub = origHub
		userRepo = origUserRepo
	})

	SetHub(ws.NewHub())
	SetUserRepo(&stubUserRepo{
		listUsers: func(_ context.Context, role string) ([]model.User, error) {
			if role == "admin" {
				return []model.User{{UserID: 101}}, nil
			}
			if role == "super_admin" {
				return nil, errors.New("list failed")
			}
			return nil, nil
		},
	})

	sendCalled := false
	sendToUsers = func(userIDs []int64, event string, data interface{}) error {
		sendCalled = true
		return nil
	}

	err := BroadcastToAdmins(context.Background(), "booking:updated", map[string]any{"booking_id": 1})
	assert.Error(t, err)
	assert.False(t, sendCalled)
}

func TestBroadcastToAdmins_NoOpWhenHubNil(t *testing.T) {
	origHub := hub
	origUserRepo := userRepo
	t.Cleanup(func() {
		hub = origHub
		userRepo = origUserRepo
	})

	SetHub(nil)
	SetUserRepo(&stubUserRepo{
		listUsers: func(_ context.Context, role string) ([]model.User, error) {
			return nil, errors.New("should not be called")
		},
	})

	err := BroadcastToAdmins(context.Background(), "booking:updated", map[string]any{"booking_id": 1})
	assert.NoError(t, err)
}

func TestBroadcastToAdmins_NoOpWhenUserRepoNil(t *testing.T) {
	origHub := hub
	origUserRepo := userRepo
	t.Cleanup(func() {
		hub = origHub
		userRepo = origUserRepo
	})

	SetHub(ws.NewHub())
	SetUserRepo(nil)

	err := BroadcastToAdmins(context.Background(), "booking:updated", map[string]any{"booking_id": 1})
	assert.NoError(t, err)
}
