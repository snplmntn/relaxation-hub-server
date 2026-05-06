package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

type mockBranchRepo struct {
	getFunc    func(ctx context.Context, branchID int64) (*model.Branch, error)
	createFunc func(ctx context.Context, b *model.Branch) error
}

func (m *mockBranchRepo) Create(ctx context.Context, b *model.Branch) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, b)
	}
	return nil
}
func (m *mockBranchRepo) GetByID(ctx context.Context, branchID int64) (*model.Branch, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, branchID)
	}
	return nil, nil
}
func (m *mockBranchRepo) List(ctx context.Context, activeOnly bool) ([]model.Branch, error) {
	return nil, nil
}
func (m *mockBranchRepo) Update(ctx context.Context, branchID int64, updates map[string]interface{}) error {
	return nil
}

func TestGetBranch_Success(t *testing.T) {
	m := &mockBranchRepo{
		getFunc: func(ctx context.Context, branchID int64) (*model.Branch, error) {
			return &model.Branch{BranchID: branchID, BranchName: "Main Branch"}, nil
		},
	}

	svc := service.NewBranchService(m)
	h := NewBranchHandler(svc)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/branches/1", nil)

	rc := chi.NewRouteContext()
	rc.URLParams.Add("id", "1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rc))

	h.GetBranch(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var resp model.BranchResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if resp.BranchName != "Main Branch" {
		t.Fatalf("unexpected branch name: %s", resp.BranchName)
	}
}

func TestCreateBranch_Success(t *testing.T) {
	m := &mockBranchRepo{
		createFunc: func(ctx context.Context, b *model.Branch) error {
			b.BranchID = 42
			return nil
		},
	}

	svc := service.NewBranchService(m)
	h := NewBranchHandler(svc)

	body := `{"branch_name":"New","city":"City","province":"Prov"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/branches", bytesFromString(body))
	req.Header.Set("Content-Type", "application/json")

	h.CreateBranch(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", rr.Code)
	}

	var resp model.BranchResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if resp.BranchID != 42 {
		t.Fatalf("expected branch id 42, got %d", resp.BranchID)
	}
}

// helper to avoid importing bytes repeatedly in many small test files
func bytesFromString(s string) io.ReadCloser {
	return io.NopCloser(bytes.NewReader([]byte(s)))
}
