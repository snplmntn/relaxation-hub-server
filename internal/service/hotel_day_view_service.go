package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"sort"
	"time"
)

var ErrHotelDayViewDate = errors.New("date must be a valid YYYY-MM-DD business date")

const (
	hotelDayViewLimit   = 5
	hotelTherapistLabel = "Therapist"
)

type hotelDayViewOrderRepository interface {
	GetByViewAndBusinessDate(ctx context.Context, viewKey string, businessDate time.Time) (*model.DayViewTherapistOrder, error)
}

type HotelDayViewService struct {
	access *PartnerHotelService
	repo   repository.HotelDayViewRepository
	order  hotelDayViewOrderRepository
}

func NewHotelDayViewService(access *PartnerHotelService, repo repository.HotelDayViewRepository, order hotelDayViewOrderRepository) *HotelDayViewService {
	return &HotelDayViewService{access: access, repo: repo, order: order}
}

func (s *HotelDayViewService) Get(ctx context.Context, userID int64, date string) (*model.HotelDayView, error) {
	_, err := s.access.GetAccess(ctx, userID)
	if err != nil {
		return nil, err
	}
	loc := time.FixedZone(manilaLocationName, 8*60*60)
	day, err := time.ParseInLocation("2006-01-02", date, loc)
	if err != nil {
		return nil, ErrHotelDayViewDate
	}
	start, end := day.Add(manilaDayViewStartHour*time.Hour).UTC(), day.Add(manilaDayViewEndHour*time.Hour).UTC()
	branches, err := s.repo.ListBranches(ctx)
	if err != nil {
		return nil, err
	}
	therapists, err := s.repo.ListSchedule(ctx, start, end)
	if err != nil {
		return nil, err
	}
	if len(branches) == 0 {
		return &model.HotelDayView{Date: date, Start: start, End: end, Timezone: manilaLocationName, Branches: branches, Therapists: []model.HotelDayViewTherapist{}}, nil
	}

	branchID := branches[0].BranchID
	eligible := make([]model.HotelDayViewTherapist, 0, len(therapists))
	bookedMinutes := make(map[int64]int, len(therapists))
	for _, therapist := range therapists {
		if !therapist.AcceptingBookings || therapist.BranchID == nil || *therapist.BranchID != branchID {
			continue
		}
		bookedMinutes[therapist.TherapistID] = hotelBookedMinutes(therapist.BookedSlots, start, end)
		eligible = append(eligible, therapist)
	}

	orderIndex, err := s.orderIndex(ctx, branchID, day)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(eligible, func(i, j int) bool {
		left, right := eligible[i], eligible[j]
		if bookedMinutes[left.TherapistID] != bookedMinutes[right.TherapistID] {
			return bookedMinutes[left.TherapistID] < bookedMinutes[right.TherapistID]
		}
		leftIndex, leftOrdered := orderIndex[left.TherapistID]
		rightIndex, rightOrdered := orderIndex[right.TherapistID]
		if leftOrdered && rightOrdered && leftIndex != rightIndex {
			return leftIndex < rightIndex
		}
		if leftOrdered != rightOrdered {
			return leftOrdered
		}
		return left.TherapistID < right.TherapistID
	})
	if len(eligible) > hotelDayViewLimit {
		eligible = eligible[:hotelDayViewLimit]
	}
	for i := range eligible {
		eligible[i].Name = hotelTherapistLabel
		eligible[i].BookedSlots = mergeHotelSlots(eligible[i].BookedSlots, start, end)
	}
	return &model.HotelDayView{Date: date, Start: start, End: end, Timezone: manilaLocationName, Branches: branches, Therapists: eligible}, nil
}

func (s *HotelDayViewService) orderIndex(ctx context.Context, branchID int64, day time.Time) (map[int64]int, error) {
	result := map[int64]int{}
	if s.order == nil {
		return result, nil
	}
	order, err := s.order.GetByViewAndBusinessDate(ctx, fmt.Sprintf("branch:%d", branchID), day)
	if errors.Is(err, pgx.ErrNoRows) {
		// Missing order rows are normal for future schedules.
		return result, nil
	}
	if err != nil {
		return nil, err
	}
	if order == nil {
		return result, nil
	}
	for index, therapistID := range order.TherapistIDs {
		result[therapistID] = index
	}
	return result, nil
}

func hotelBookedMinutes(slots []model.HotelBookedSlot, start, end time.Time) int {
	total := time.Duration(0)
	for _, slot := range slots {
		from, to := slot.Start, slot.End
		if from.Before(start) {
			from = start
		}
		if to.After(end) {
			to = end
		}
		if from.Before(to) {
			total += to.Sub(from)
		}
	}
	return int(total / time.Minute)
}

func mergeHotelSlots(slots []model.HotelBookedSlot, start, end time.Time) []model.HotelBookedSlot {
	sort.Slice(slots, func(i, j int) bool { return slots[i].Start.Before(slots[j].Start) })
	merged := make([]model.HotelBookedSlot, 0)
	for _, slot := range slots {
		if slot.Start.Before(start) {
			slot.Start = start
		}
		if slot.End.After(end) {
			slot.End = end
		}
		if !slot.Start.Before(slot.End) {
			continue
		}
		if len(merged) > 0 && !slot.Start.After(merged[len(merged)-1].End) {
			if slot.End.After(merged[len(merged)-1].End) {
				merged[len(merged)-1].End = slot.End
			}
		} else {
			merged = append(merged, slot)
		}
	}
	return merged
}
