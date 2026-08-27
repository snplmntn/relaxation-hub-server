package repository

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/stretchr/testify/mock"
)

func TestServiceAreaRepoGetByNameMatchesWhenInputContainsStoredName(t *testing.T) {
	t.Parallel()

	mockDB := new(MockDBTX)
	row := new(MockRow)
	repo := NewServiceAreaRepository(mockDB)

	mockDB.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		normalized := strings.ToLower(sql)
		return strings.Contains(normalized, "name ilike $1") &&
			strings.Contains(normalized, "$1 ilike '%' || name || '%'")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) == 2 && args[0] == "%Makati City%" && args[1] == model.ServiceAreaLevelCity
	})).Return(row).Once()
	row.On("Scan", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(pgx.ErrNoRows).Once()

	_, err := repo.GetByName(context.Background(), "Makati City", "city")
	if err != ErrAreaNotFound {
		t.Fatalf("expected ErrAreaNotFound from mocked empty row, got %v", err)
	}

	mockDB.AssertExpectations(t)
	row.AssertExpectations(t)
}

func TestServiceAreaRepoListTopDemandIncludesAllStatusesWithRequests(t *testing.T) {
	t.Parallel()

	mockDB := new(MockDBTX)
	rows := new(MockRows)
	repo := NewServiceAreaRepository(mockDB)

	mockDB.On("Query", mock.Anything, mock.MatchedBy(func(sql string) bool {
		normalized := strings.ToLower(sql)
		return strings.Contains(normalized, "from service_areas sa") &&
			strings.Contains(normalized, "join area_coverage_requests acr") &&
			!strings.Contains(normalized, "sa.status = 'not_supported'")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) == 1 && args[0] == 20
	})).Return(rows, nil).Once()
	rows.On("Next").Return(false).Once()
	rows.On("Err").Return(nil).Once()
	rows.On("Close").Return().Once()
	rows.On("CommandTag").Return(pgconn.CommandTag{}).Maybe()

	areas, err := repo.ListTopDemand(context.Background(), 20)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(areas) != 0 {
		t.Fatalf("expected no scanned rows, got %d", len(areas))
	}

	mockDB.AssertExpectations(t)
	rows.AssertExpectations(t)
}
