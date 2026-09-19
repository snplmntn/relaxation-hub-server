package repository_test

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/config"
	"github.com/snplmntn/relaxation-hub-server/internal/handler"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type failedHotelAccessRepo struct {
	repository.PartnerHotelRepository
	failure error
}

func (r failedHotelAccessRepo) GetHotelAccess(context.Context, int64) (*model.HotelAccess, error) {
	return nil, r.failure
}

func checkHotelLoginErrorClassification(t *testing.T, ctx context.Context, users repository.UserRepository, hotels repository.PartnerHotelRepository, email string) {
	for _, test := range []struct {
		name    string
		failure error
		status  int
	}{
		{"database failure", fmt.Errorf("private database connection error"), http.StatusServiceUnavailable},
		{"inactive membership", pgx.ErrNoRows, http.StatusForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			svc := service.NewAuthService(users, &config.Config{JWTKey: "error-test-key"}, failedHotelAccessRepo{PartnerHotelRepository: hotels, failure: test.failure})
			h := handler.NewAuthHandler(svc, nil, nil)
			payload, err := json.Marshal(map[string]string{"provider": "email", "provider_key": email, "password": "TestingPass1!"})
			require.NoError(t, err)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/login", strings.NewReader(string(payload))).WithContext(ctx)
			rr := httptest.NewRecorder()
			h.HandleLogin(rr, req)
			require.Equal(t, test.status, rr.Code, rr.Body.String())
			require.NotContains(t, rr.Body.String(), "private database")
		})
	}
}
