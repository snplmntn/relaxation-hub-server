package service

import (
	"context"
	"errors"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"sort"
	"time"
)

var ErrHotelDayViewDate = errors.New("date must be a valid YYYY-MM-DD business date")

type HotelDayViewService struct {
	access *PartnerHotelService
	repo   repository.HotelDayViewRepository
}

func NewHotelDayViewService(access *PartnerHotelService, repo repository.HotelDayViewRepository) *HotelDayViewService {
	return &HotelDayViewService{access: access, repo: repo}
}

func (s *HotelDayViewService) Get(ctx context.Context, userID int64, date string) (*model.HotelDayView, error) {
	if _, err := s.access.GetAccess(ctx, userID); err != nil {
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
	for i := range therapists {
		therapists[i].BookedSlots = mergeHotelSlots(therapists[i].BookedSlots, start, end)
	}
	sort.SliceStable(therapists, func(i, j int) bool {
		if therapists[i].AcceptingBookings != therapists[j].AcceptingBookings {
			return therapists[i].AcceptingBookings
		}
		return therapists[i].Name < therapists[j].Name
	})
	return &model.HotelDayView{Date: date, Start: start, End: end, Timezone: manilaLocationName, Branches: branches, Therapists: therapists}, nil
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
