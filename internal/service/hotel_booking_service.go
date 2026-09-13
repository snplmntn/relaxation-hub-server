package service

import (
	"context"
	"fmt"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"math"
	"strings"
	"time"
)

type HotelBookingService struct {
	access   *PartnerHotelService
	repo     repository.HotelBookingRepository
	bookings *BookingService
}

func NewHotelBookingService(access *PartnerHotelService, repo repository.HotelBookingRepository, bookings *BookingService) *HotelBookingService {
	return &HotelBookingService{access: access, repo: repo, bookings: bookings}
}
func (s *HotelBookingService) ListOptions(ctx context.Context) ([]model.HotelBookingOption, error) {
	return s.repo.ListOptions(ctx)
}
func (s *HotelBookingService) ListAnalyticsOptions(ctx context.Context) ([]model.HotelAnalyticsOption, error) {
	return s.repo.ListAnalyticsOptions(ctx)
}
func (s *HotelBookingService) Analytics(ctx context.Context, userID int64, days int) (*model.HotelAnalytics, error) {
	if days != 7 && days != 30 && days != 90 && days != 365 {
		return nil, fmt.Errorf("analytics range must be 7, 30, 90, or 365 days")
	}
	access, err := s.access.GetAccess(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.AnalyticsForHotel(ctx, access.PartnerHotelID, days)
}

// AnalyticsForHotel returns hotel-scoped performance data for operational
// staff. Route-level RBAC is responsible for limiting this entry point to
// admin and super_admin accounts.
func (s *HotelBookingService) AnalyticsForHotel(ctx context.Context, hotelID int64, days int) (*model.HotelAnalytics, error) {
	if hotelID <= 0 {
		return nil, fmt.Errorf("hotel ID must be positive")
	}
	if days != 7 && days != 30 && days != 90 && days != 365 {
		return nil, fmt.Errorf("analytics range must be 7, 30, 90, or 365 days")
	}
	location, err := time.LoadLocation("Asia/Manila")
	if err != nil {
		return nil, err
	}
	now := time.Now().In(location)
	businessNow := now.Add(-4 * time.Hour)
	today := time.Date(businessNow.Year(), businessNow.Month(), businessNow.Day(), 0, 0, 0, 0, location)
	from := today.AddDate(0, 0, -(days - 1))
	rangeStart := time.Date(from.Year(), from.Month(), from.Day(), 4, 0, 0, 0, location)
	rangeEnd := time.Date(today.Year(), today.Month(), today.Day(), 4, 0, 0, 0, location).AddDate(0, 0, 1)
	result, err := s.repo.Analytics(ctx, hotelID, rangeStart.UTC(), rangeEnd.UTC())
	if err != nil {
		return nil, err
	}
	result.Period = model.HotelAnalyticsPeriod{From: from.Format("2006-01-02"), To: today.Format("2006-01-02")}
	if result.Summary.CompletedBookings > 0 {
		result.Summary.AverageBookingValue = moneyRound(result.Summary.Revenue / float64(result.Summary.CompletedBookings))
	}
	if result.Summary.TotalBookings > 0 {
		result.Summary.CompletionRate = rateRound(float64(result.Summary.CompletedBookings) * 100 / float64(result.Summary.TotalBookings))
		result.Summary.CancellationRate = rateRound(float64(result.Summary.CancelledBookings) * 100 / float64(result.Summary.TotalBookings))
	}
	return result, nil
}

func moneyRound(value float64) float64 { return math.Round(value*100) / 100 }
func rateRound(value float64) float64  { return math.Round(value*10) / 10 }
func (s *HotelBookingService) List(ctx context.Context, userID int64, page int, status string) ([]model.HotelBooking, error) {
	access, err := s.access.GetAccess(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.repo.List(ctx, access.PartnerHotelID, 50, (page-1)*50, strings.ToLower(strings.TrimSpace(status)))
}
func (s *HotelBookingService) owner(ctx context.Context, userID, bookingID int64) (int64, error) {
	access, err := s.access.GetAccess(ctx, userID)
	if err != nil {
		return 0, err
	}
	return s.repo.Owner(ctx, access.PartnerHotelID, bookingID)
}
func (s *HotelBookingService) Update(ctx context.Context, userID, bookingID int64, req model.UpdateHotelBookingRequest) error {
	owner, err := s.owner(ctx, userID, bookingID)
	if err != nil {
		return err
	}
	name := strings.TrimSpace(req.GuestName)
	if name == "" || len(name) > 200 || len(req.Notes) > 10000 {
		return fmt.Errorf("client name is required (maximum 200 characters); notes must be at most 10000 characters")
	}
	b, err := s.bookings.GetByBookingID(ctx, bookingID)
	if err != nil {
		return err
	}
	switch b.Status {
	case "pending", "assigned", "on_the_way", "arrived":
	default:
		return fmt.Errorf("this booking can no longer be edited")
	}
	_, err = s.bookings.Update(ctx, bookingID, owner, &model.UpdateBookingRequest{GuestName: &name, Notes: &req.Notes})
	return err
}
func (s *HotelBookingService) Cancel(ctx context.Context, userID, bookingID int64, reason string) error {
	owner, err := s.owner(ctx, userID, bookingID)
	if err != nil {
		return err
	}
	reason = strings.TrimSpace(reason)
	if reason == "" || len(reason) > 1000 {
		return fmt.Errorf("provide a cancellation reason (maximum 1000 characters)")
	}
	reason = fmt.Sprintf("Hotel staff #%d: %s", userID, reason)
	_, err = s.bookings.UpdateStatus(ctx, bookingID, owner, model.RoleClient, &model.UpdateBookingStatusRequest{Status: "cancelled", CancellationReason: &reason})
	return err
}
