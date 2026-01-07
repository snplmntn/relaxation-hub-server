package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

// BookingGroupService handles complex booking group operations.
type BookingGroupService struct {
	db              db.DBTX
	groupRepo       repository.BookingGroupRepository
	bookingRepo     repository.BookingRepository
	addonRepo       repository.BookingAddonRepository
	productRepo     repository.ProductRepository
	serviceRepo     repository.ServiceRepository
	queueRepo       repository.AssignmentQueueRepository
}

// NewBookingGroupService creates a new BookingGroupService.
func NewBookingGroupService(
	db db.DBTX,
	groupRepo repository.BookingGroupRepository,
	bookingRepo repository.BookingRepository,
	addonRepo repository.BookingAddonRepository,
	productRepo repository.ProductRepository,
	serviceRepo repository.ServiceRepository,
	queueRepo repository.AssignmentQueueRepository,
) *BookingGroupService {
	return &BookingGroupService{
		db:          db,
		groupRepo:   groupRepo,
		bookingRepo: bookingRepo,
		addonRepo:   addonRepo,
		productRepo: productRepo,
		serviceRepo: serviceRepo,
		queueRepo:   queueRepo,
	}
}

// CreateBookingGroup creates a booking group with multiple bookings and add-ons.
func (s *BookingGroupService) CreateBookingGroup(ctx context.Context, clientID int64, req *model.CreateBookingGroupRequest) (*model.BookingGroup, error) {
	if req == nil || len(req.Bookings) == 0 {
		return nil, fmt.Errorf("at least one booking is required")
	}

	// Parse scheduled start
	var scheduledStart *time.Time
	if req.ScheduledStart != "" {
		t, err := time.Parse(time.RFC3339, req.ScheduledStart)
		if err != nil {
			return nil, fmt.Errorf("invalid scheduled_start: %w", err)
		}
		scheduledStart = &t
	} else {
		now := time.Now()
		scheduledStart = &now
	}

	// Start transaction
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// 1. Calculate totals and validate services
	var rawTotal float64
	bookingDetails := make([]struct {
		Service        *model.Service
		Req            model.CreateGroupBookingRequest
		CalculatedCost float64
		StartTime      time.Time
	}, len(req.Bookings))

	// Sort by sequence to calculate start times
	// For simplicity, assume they are already sorted
	currentStartTime := *scheduledStart

	for i, bReq := range req.Bookings {
		svc, err := s.serviceRepo.GetByID(ctx, bReq.ServiceID)
		if err != nil {
			return nil, fmt.Errorf("invalid service %d: %w", bReq.ServiceID, err)
		}

		duration := bReq.DurationMinutes
		if duration <= 0 {
			duration = svc.DurationMinutes
		}

		// Calculate cost
		basePrice := svc.BasePrice
		extraCost := 0.0
		if duration > svc.DurationMinutes && svc.DurationMinutes > 0 {
			diff := duration - svc.DurationMinutes
			ratePerMinute := svc.BasePrice / float64(svc.DurationMinutes)
			extraCost = ratePerMinute * float64(diff)
		}
		cost := basePrice + extraCost

		// Add addon costs
		for _, addon := range bReq.Addons {
			product, err := s.productRepo.GetByID(ctx, addon.ProductID)
			if err != nil {
				return nil, fmt.Errorf("invalid product %d: %w", addon.ProductID, err)
			}
			cost += product.Price * float64(addon.Quantity)
		}

		rawTotal += cost

		// Calculate start time
		var startTime time.Time
		if bReq.StartCondition == "after_previous" && i > 0 {
			prevDetail := bookingDetails[i-1]
			startTime = prevDetail.StartTime.Add(time.Duration(prevDetail.Req.DurationMinutes) * time.Minute)
		} else {
			startTime = currentStartTime
		}

		bookingDetails[i] = struct {
			Service        *model.Service
			Req            model.CreateGroupBookingRequest
			CalculatedCost float64
			StartTime      time.Time
		}{
			Service:        svc,
			Req:            bReq,
			CalculatedCost: cost,
			StartTime:      startTime,
		}
	}

	// 2. Create the BookingGroup
	group := &model.BookingGroup{
		ClientID:       clientID,
		AddressID:      req.AddressID,
		ScheduledStart: scheduledStart,
		RawTotal:       rawTotal,
		Discount:       0, // TODO: Apply voucher
		FinalTotal:     rawTotal,
		PaymentMethod:  strings.TrimSpace(req.PaymentMethod),
		Status:         "pending",
	}

	if err := s.groupRepo.CreateTx(ctx, tx, group); err != nil {
		return nil, fmt.Errorf("failed to create booking group: %w", err)
	}

	// 3. Create individual Bookings
	var createdBookings []model.Booking
	for _, detail := range bookingDetails {
		bReq := detail.Req
		startTime := detail.StartTime

		duration := bReq.DurationMinutes
		if duration <= 0 {
			duration = detail.Service.DurationMinutes
		}

		booking := &model.Booking{
			ClientID:        clientID,
			ServiceID:       &bReq.ServiceID,
			AddressID:       req.AddressID,
			GenderPref:      strings.TrimSpace(bReq.GenderPref),
			PressurePref:    strings.TrimSpace(bReq.PressurePref),
			Notes:           strings.TrimSpace(bReq.Notes),
			DurationMinutes: duration,
			ScheduledStart:  &startTime,
			RawTotal:        &detail.CalculatedCost,
			FinalTotal:      &detail.CalculatedCost,
			Status:          "pending",
			GroupID:         &group.GroupID,
			GuestName:       bReq.GuestName,
			SequenceNumber:  bReq.SequenceNumber,
			StartCondition:  bReq.StartCondition,
		}

		// Use CreateTx from bookingRepo
		if err := s.bookingRepo.CreateTx(ctx, tx, booking); err != nil {
			return nil, fmt.Errorf("failed to create booking: %w", err)
		}

		// 4. Create Add-ons for this booking
		for _, addonReq := range bReq.Addons {
			product, _ := s.productRepo.GetByID(ctx, addonReq.ProductID) // Already validated above
			addon := &model.BookingAddon{
				BookingID:      booking.BookingID,
				ProductID:      addonReq.ProductID,
				Quantity:       addonReq.Quantity,
				PriceAtBooking: product.Price,
			}
			if err := s.addonRepo.CreateTx(ctx, tx, addon); err != nil {
				return nil, fmt.Errorf("failed to create addon: %w", err)
			}
		}

		createdBookings = append(createdBookings, *booking)
	}

	// 5. Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	// 6. Enqueue bookings for assignment
	for _, b := range createdBookings {
		if err := s.queueRepo.Enqueue(ctx, b.BookingID); err != nil {
			log.Printf("booking_group_service: failed to enqueue booking %d: %v", b.BookingID, err)
		}
	}

	group.Bookings = createdBookings
	return group, nil
}

// GetGroupByID retrieves a booking group with its bookings.
func (s *BookingGroupService) GetGroupByID(ctx context.Context, groupID int64) (*model.BookingGroup, error) {
	return s.groupRepo.GetByIDWithBookings(ctx, groupID)
}
