package service

import (
	"context"
	"fmt"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

// CashRemittanceService exposes therapist cash-on-hand reporting and remittance.
type CashRemittanceService struct {
	repo repository.CashRemittanceRepository
}

func NewCashRemittanceService(repo repository.CashRemittanceRepository) *CashRemittanceService {
	return &CashRemittanceService{repo: repo}
}

// ListCashOnHand returns every therapist currently holding cash, highest first.
func (s *CashRemittanceService) ListCashOnHand(ctx context.Context) ([]model.TherapistCashOnHand, error) {
	rows, err := s.repo.ListTherapistCashOnHand(ctx)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].TotalCollected = roundCurrency(rows[i].TotalCollected)
		rows[i].TotalRemitted = roundCurrency(rows[i].TotalRemitted)
		rows[i].CashOnHand = roundCurrency(rows[i].TotalCollected - rows[i].TotalRemitted)
	}
	return rows, nil
}

// RemitCash records a remittance. With a nil amount the full outstanding cash on
// hand is remitted; an explicit amount must be positive and not exceed it.
func (s *CashRemittanceService) RemitCash(ctx context.Context, req *model.CreateCashRemittanceRequest, actorID int64) (*model.CashRemittance, error) {
	if req == nil || req.TherapistID <= 0 {
		return nil, NewValidationError("invalid_therapist", "a valid therapist is required", map[string]string{"therapist_id": "required"})
	}

	summary, err := s.repo.GetTherapistCashOnHand(ctx, req.TherapistID)
	if err != nil {
		return nil, err
	}
	outstanding := roundCurrency(summary.TotalCollected - summary.TotalRemitted)

	amount := outstanding
	if req.Amount != nil {
		amount = roundCurrency(*req.Amount)
	}
	if amount <= 0 {
		return nil, NewValidationError("invalid_amount", "remittance amount must be greater than zero", map[string]string{"amount": "must be greater than zero"})
	}
	if amount > outstanding {
		return nil, NewValidationError(
			"amount_exceeds_outstanding",
			fmt.Sprintf("amount exceeds cash on hand (%.2f)", outstanding),
			map[string]string{"amount": "exceeds cash on hand"},
		)
	}

	remittance := &model.CashRemittance{
		TherapistID: req.TherapistID,
		Amount:      amount,
		Notes:       req.Notes,
		RemittedBy:  &actorID,
	}
	if err := s.repo.CreateRemittance(ctx, remittance); err != nil {
		return nil, err
	}
	return remittance, nil
}

// ListHistory returns recent remittances recorded for a therapist.
func (s *CashRemittanceService) ListHistory(ctx context.Context, therapistID int64, limit int) ([]model.CashRemittance, error) {
	if therapistID <= 0 {
		return nil, NewValidationError("invalid_therapist", "a valid therapist is required", map[string]string{"therapist_id": "required"})
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.repo.ListRemittancesByTherapist(ctx, therapistID, limit)
}
