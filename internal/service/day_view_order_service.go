package service

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/broadcaster"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

const (
	DayViewOrderSourceAuto   = "auto"
	DayViewOrderSourceManual = "manual"
	manilaLocationName       = "Asia/Manila"
	manilaDayCutoffHour      = 6
)

type DayViewOrderService struct {
	repo repository.DayViewOrderRepository
}

func NewDayViewOrderService(repo repository.DayViewOrderRepository) *DayViewOrderService {
	return &DayViewOrderService{repo: repo}
}

func (s *DayViewOrderService) GetOrGenerateOrder(ctx context.Context, viewKey string) (*model.DayViewTherapistOrder, error) {
	scope, err := parseDayViewScope(viewKey)
	if err != nil {
		return nil, err
	}

	businessDate, err := currentBusinessDate()
	if err != nil {
		return nil, err
	}

	existing, err := s.repo.GetByViewAndBusinessDate(ctx, scope.ViewKey, businessDate)
	if err == nil {
		return existing, nil
	}
	if err != pgx.ErrNoRows {
		return nil, err
	}

	generated, err := s.generateAutoOrder(ctx, scope, businessDate)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Upsert(ctx, generated); err != nil {
		return nil, err
	}

	s.broadcastOrder(context.WithoutCancel(ctx), generated)

	return generated, nil
}

func (s *DayViewOrderService) SaveManualOrder(ctx context.Context, viewKey string, therapistIDs []int64, adminID int64) (*model.DayViewTherapistOrder, error) {
	scope, err := parseDayViewScope(viewKey)
	if err != nil {
		return nil, err
	}

	businessDate, err := currentBusinessDate()
	if err != nil {
		return nil, err
	}

	candidates, err := s.repo.ListTherapistsByBranch(ctx, scope.BranchID)
	if err != nil {
		return nil, err
	}

	eligible := make(map[int64]model.DayViewTherapistCandidate, len(candidates))
	for _, c := range candidates {
		eligible[c.TherapistID] = c
	}

	normalized := make([]int64, 0, len(candidates))
	seen := make(map[int64]struct{}, len(therapistIDs))
	for _, therapistID := range therapistIDs {
		if _, exists := eligible[therapistID]; !exists {
			return nil, fmt.Errorf("therapist_id %d is not eligible for %s", therapistID, scope.ViewKey)
		}
		if _, dup := seen[therapistID]; dup {
			return nil, fmt.Errorf("duplicate therapist_id %d", therapistID)
		}
		seen[therapistID] = struct{}{}
		normalized = append(normalized, therapistID)
	}

	for _, c := range candidates {
		if _, exists := seen[c.TherapistID]; exists {
			continue
		}
		normalized = append(normalized, c.TherapistID)
	}

	order := &model.DayViewTherapistOrder{
		ViewKey:          scope.ViewKey,
		BusinessDate:     businessDate,
		TherapistIDs:     normalized,
		Source:           DayViewOrderSourceManual,
		UpdatedByAdminID: &adminID,
	}

	if err := s.repo.Upsert(ctx, order); err != nil {
		return nil, err
	}

	s.broadcastOrder(context.WithoutCancel(ctx), order)

	return order, nil
}

func (s *DayViewOrderService) generateAutoOrder(ctx context.Context, scope dayViewScope, businessDate time.Time) (*model.DayViewTherapistOrder, error) {
	candidates, err := s.repo.ListTherapistsByBranch(ctx, scope.BranchID)
	if err != nil {
		return nil, err
	}

	if len(candidates) == 0 {
		return &model.DayViewTherapistOrder{
			ViewKey:      scope.ViewKey,
			BusinessDate: businessDate,
			TherapistIDs: []int64{},
			Source:       DayViewOrderSourceAuto,
		}, nil
	}

	ids := make([]int64, 0, len(candidates))
	byID := make(map[int64]model.DayViewTherapistCandidate, len(candidates))
	for _, c := range candidates {
		ids = append(ids, c.TherapistID)
		byID[c.TherapistID] = c
	}

	yesterdayStartUTC, yesterdayEndUTC, err := yesterdayWindowUTC(businessDate)
	if err != nil {
		return nil, err
	}

	hours, err := s.repo.GetTherapistHoursBetween(ctx, ids, yesterdayStartUTC, yesterdayEndUTC)
	if err != nil {
		return nil, err
	}

	// Sort ascending by hours so the therapist with fewest hours yesterday
	// appears first. Therapists with no bookings default to 0 hours and sort
	// to the very top, giving them priority for new assignments.
	sort.Slice(candidates, func(i, j int) bool {
		hi := hours[candidates[i].TherapistID]
		hj := hours[candidates[j].TherapistID]
		if hi != hj {
			return hi < hj
		}
		if candidates[i].Name != candidates[j].Name {
			return candidates[i].Name < candidates[j].Name
		}
		return candidates[i].TherapistID < candidates[j].TherapistID
	})

	orderedIDs := make([]int64, 0, len(candidates))
	for _, c := range candidates {
		orderedIDs = append(orderedIDs, c.TherapistID)
	}

	return &model.DayViewTherapistOrder{
		ViewKey:      scope.ViewKey,
		BusinessDate: businessDate,
		TherapistIDs: orderedIDs,
		Source:       DayViewOrderSourceAuto,
	}, nil
}

func (s *DayViewOrderService) broadcastOrder(ctx context.Context, order *model.DayViewTherapistOrder) {
	if order == nil {
		return
	}
	_ = broadcaster.BroadcastToAdmins(ctx, "day_view:therapist_order_updated", map[string]any{
		"view_key":            order.ViewKey,
		"business_date":       order.BusinessDate.Format("2006-01-02"),
		"source":              order.Source,
		"therapist_ids":       order.TherapistIDs,
		"updated_by_admin_id": order.UpdatedByAdminID,
	})
}

type dayViewScope struct {
	ViewKey  string
	BranchID *int64
}

func parseDayViewScope(viewKey string) (dayViewScope, error) {
	trimmed := strings.TrimSpace(viewKey)
	if trimmed == "freelance" {
		return dayViewScope{ViewKey: trimmed, BranchID: nil}, nil
	}
	if !strings.HasPrefix(trimmed, "branch:") {
		return dayViewScope{}, fmt.Errorf("invalid view_key")
	}
	rawID := strings.TrimPrefix(trimmed, "branch:")
	branchID, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || branchID <= 0 {
		return dayViewScope{}, fmt.Errorf("invalid view_key")
	}
	return dayViewScope{ViewKey: fmt.Sprintf("branch:%d", branchID), BranchID: &branchID}, nil
}

func currentBusinessDate() (time.Time, error) {
	location, err := time.LoadLocation(manilaLocationName)
	if err != nil {
		return time.Time{}, err
	}
	now := time.Now().In(location)
	if now.Hour() < manilaDayCutoffHour {
		now = now.AddDate(0, 0, -1)
	}
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location), nil
}

func yesterdayWindowUTC(businessDate time.Time) (time.Time, time.Time, error) {
	location, err := time.LoadLocation(manilaLocationName)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}

	currentDateLocal := time.Date(businessDate.In(location).Year(), businessDate.In(location).Month(), businessDate.In(location).Day(), 0, 0, 0, 0, location)
	yesterdayStartLocal := currentDateLocal.AddDate(0, 0, -1)
	yesterdayEndLocal := currentDateLocal

	return yesterdayStartLocal.UTC(), yesterdayEndLocal.UTC(), nil
}
