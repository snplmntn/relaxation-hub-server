package service

import (
	"context"
	"fmt"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

// BookingQuote is what a booking will cost, priced before it exists.
//
// It deliberately reuses the same service resolution, duration allocation and
// pricing helpers the create path uses, so the amount a customer is charged
// online cannot drift from the total the booking is later written with. It does
// NOT apply vouchers: online payment is the only caller, and vouchers are
// rejected there.
type BookingQuote struct {
	RawTotal   float64
	Discount   float64
	FinalTotal float64
}

// QuoteBooking prices a single booking request for a client, including the VIP
// discount when the client is entitled to it.
func (s *BookingService) QuoteBooking(ctx context.Context, clientID int64, req *model.CreateBookingRequest) (*BookingQuote, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	selection, err := s.resolveBookingServices(ctx, req.ServiceIDs, req.ServiceID)
	if err != nil {
		return nil, err
	}
	for _, item := range selection.Items {
		if item.Service != nil && !item.Service.IsActive {
			return nil, NewValidationError("inactive_service", fmt.Sprintf("service %q is not active", item.Service.Name), map[string]string{"service_ids": fmt.Sprintf("%d is not active", item.ServiceID)})
		}
	}
	if req.DurationMinutes == 0 {
		req.DurationMinutes = selection.TotalBaseDuration
	}
	if err := applyBookingServiceDurationAllocations(selection, req.ServiceDurations, req.DurationMinutes); err != nil {
		return nil, err
	}

	rawTotal := bookingPriceForDuration(selection, req.DurationMinutes)

	var client *model.User
	if s.userRepo != nil {
		if u, uerr := s.userRepo.FindUserByID(ctx, int(clientID)); uerr == nil {
			client = u
		}
	}
	discount := 0.0
	if d := vipDiscountForClient(client, rawTotal); d != nil {
		discount = *d
	}

	return &BookingQuote{
		RawTotal:   roundCurrency(rawTotal),
		Discount:   roundCurrency(discount),
		FinalTotal: roundCurrency(rawTotal - discount),
	}, nil
}

// QuoteGroup prices a group booking request for a client, including VIP.
func (s *BookingGroupService) QuoteGroup(ctx context.Context, clientID int64, req *model.CreateBookingGroupRequest, clientFacing bool) (*BookingQuote, error) {
	if req == nil || len(req.Bookings) == 0 {
		return nil, fmt.Errorf("at least one booking is required")
	}
	scheduledStart, err := parseGroupScheduledStart(req.ScheduledStart)
	if err != nil {
		return nil, err
	}
	if scheduledStart == nil {
		now := time.Now().UTC()
		scheduledStart = &now
	}

	_, rawTotal, _, err := s.prepareBookingGroupDetails(ctx, *scheduledStart, req.Bookings, clientFacing)
	if err != nil {
		return nil, err
	}

	discount := 0.0
	if d, verr := s.groupVIPDiscount(ctx, clientID, rawTotal); verr == nil && d != nil {
		discount = *d
	}

	return &BookingQuote{
		RawTotal:   roundCurrency(rawTotal),
		Discount:   roundCurrency(discount),
		FinalTotal: roundCurrency(rawTotal - discount),
	}, nil
}
