package service

import (
	"context"
	"testing"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

type fakeCashRepo struct {
	collected float64
	remitted  float64
	created   *model.CashRemittance
}

func (f *fakeCashRepo) ListTherapistCashOnHand(_ context.Context, _, _ *time.Time) ([]model.TherapistCashOnHand, error) {
	return nil, nil
}

func (f *fakeCashRepo) GetTherapistCashOnHand(_ context.Context, therapistID int64) (*model.TherapistCashOnHand, error) {
	return &model.TherapistCashOnHand{
		TherapistID:   therapistID,
		Cash:          f.collected,
		TotalRemitted: f.remitted,
		CashOnHand:    f.collected - f.remitted,
	}, nil
}

func (f *fakeCashRepo) CreateRemittance(_ context.Context, r *model.CashRemittance) error {
	r.RemittanceID = 1
	f.created = r
	return nil
}

func (f *fakeCashRepo) ListRemittancesByTherapist(_ context.Context, _ int64, _ int) ([]model.CashRemittance, error) {
	return nil, nil
}

func TestRemitCash_NilAmountRemitsFullOutstanding(t *testing.T) {
	repo := &fakeCashRepo{collected: 1000, remitted: 200}
	svc := NewCashRemittanceService(repo)

	rem, err := svc.RemitCash(context.Background(), &model.CreateCashRemittanceRequest{TherapistID: 7}, 99)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rem.Amount != 800 {
		t.Fatalf("expected full outstanding 800, got %v", rem.Amount)
	}
	if rem.RemittedBy == nil || *rem.RemittedBy != 99 {
		t.Fatalf("expected remitted_by 99, got %v", rem.RemittedBy)
	}
}

func TestRemitCash_PartialAmountAccepted(t *testing.T) {
	repo := &fakeCashRepo{collected: 1000, remitted: 200}
	svc := NewCashRemittanceService(repo)

	amount := 300.0
	rem, err := svc.RemitCash(context.Background(), &model.CreateCashRemittanceRequest{TherapistID: 7, Amount: &amount}, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rem.Amount != 300 {
		t.Fatalf("expected 300, got %v", rem.Amount)
	}
}

func TestRemitCash_AmountExceedingOutstandingRejected(t *testing.T) {
	repo := &fakeCashRepo{collected: 1000, remitted: 200} // outstanding 800
	svc := NewCashRemittanceService(repo)

	amount := 900.0
	_, err := svc.RemitCash(context.Background(), &model.CreateCashRemittanceRequest{TherapistID: 7, Amount: &amount}, 1)
	ve, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if ve.Code != "amount_exceeds_outstanding" {
		t.Fatalf("expected amount_exceeds_outstanding, got %s", ve.Code)
	}
	if repo.created != nil {
		t.Fatalf("no remittance should be created on rejection")
	}
}

func TestRemitCash_NoOutstandingRejected(t *testing.T) {
	repo := &fakeCashRepo{collected: 500, remitted: 500} // outstanding 0
	svc := NewCashRemittanceService(repo)

	_, err := svc.RemitCash(context.Background(), &model.CreateCashRemittanceRequest{TherapistID: 7}, 1)
	ve, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if ve.Code != "invalid_amount" {
		t.Fatalf("expected invalid_amount, got %s", ve.Code)
	}
}
