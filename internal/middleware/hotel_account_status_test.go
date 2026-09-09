package middleware

import (
	"context"
	"github.com/snplmntn/relaxation-hub-server/internal/auth"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

type hotelStatusStore struct{ status string }

func (s *hotelStatusStore) FindUserByID(context.Context, int) (*model.User, error) {
	return &model.User{AccountStatus: s.status}, nil
}

func TestHotelReactivationDoesNotReuseInactiveStatus(t *testing.T) {
	for _, role := range []string{model.RoleHotelAdmin, model.RoleHotelStaff} {
		t.Run(role, func(t *testing.T) {
			store := &hotelStatusStore{status: "inactive"}
			token, err := auth.GenerateToken(7, role, "reactivation-test-key")
			require.NoError(t, err)
			h := AuthMiddleware(NewAccountStatusMiddleware(store)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })), "reactivation-test-key")
			request := func() int {
				req := httptest.NewRequest(http.MethodGet, "/api/v1/users/profile", nil)
				req.Header.Set("Authorization", "Bearer "+token)
				rr := httptest.NewRecorder()
				h.ServeHTTP(rr, req)
				return rr.Code
			}
			require.Equal(t, http.StatusForbidden, request())
			store.status = "active"
			require.Equal(t, http.StatusOK, request(), "reactivated hotel staff should be admitted immediately")
			store.status = "inactive"
			require.Equal(t, http.StatusForbidden, request())
		})
	}
}
