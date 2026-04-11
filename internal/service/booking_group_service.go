package service

import (
	"context"
	"fmt"
	"log/slog"
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
	addressRepo     repository.AddressRepository
	locationService *LocationService
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
	addressRepo repository.AddressRepository,
	locationService *LocationService,
) *BookingGroupService {
	return &BookingGroupService{
		db:              db,
		groupRepo:       groupRepo,
		bookingRepo:     bookingRepo,
		addonRepo:       addonRepo,
		productRepo:     productRepo,
		serviceRepo:     serviceRepo,
		queueRepo:       queueRepo,
		addressRepo:     addressRepo,
		locationService: locationService,
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

	// =========================================================================
	// PHASE 2: Location Validation (Geofencing)
	// =========================================================================
	if req.AddressID != nil && s.locationService != nil && s.addressRepo != nil {
		// Step 1: Fetch the address to get city/barangay codes
		// Using GetByIDUnsafe since we're in a service context and already validated clientID
		address, err := s.addressRepo.GetByIDUnsafe(ctx, *req.AddressID)
		if err != nil {
			return nil, fmt.Errorf("invalid address: %w", err)
		}

		// Step 2: Check if location is serviceable using geocoded names.
		locationResult, err := s.locationService.CheckLocationByName(ctx, clientID, address.City, address.Barangay)
		if err != nil {
			slog.Warn("booking_group_service: location check failed", "error", err)
			// Fail open - don't block booking on location service errors
		} else if !locationResult.IsAllowed {
			return nil, fmt.Errorf("%s", locationResult.Message)
		}

		// Step 3: Distance-based minimum duration rules (if we have coordinates)
		if address.Latitude != nil && address.Longitude != nil {
			// TODO: Get nearest branch coordinates from branches table
			// For now, use a hardcoded central location (Makati center)
			branchLat, branchLng := 14.5547, 121.0244

			distanceKm := s.locationService.GetDistanceKm(*address.Latitude, *address.Longitude, branchLat, branchLng)

			// Calculate total duration across all bookings in the group
			var totalDurationMinutes int
			for _, b := range req.Bookings {
				if b.DurationMinutes > 0 {
					totalDurationMinutes += b.DurationMinutes
				} else {
					totalDurationMinutes += 60 // Default 1 hour if not specified
				}
			}

			// Enforce minimum duration based on distance
			if distanceKm > 10 && totalDurationMinutes < 180 {
				return nil, fmt.Errorf("bookings over 10km away require a minimum of 3 hours total duration")
			} else if distanceKm > 5 && totalDurationMinutes < 120 {
				return nil, fmt.Errorf("bookings over 5km away require a minimum of 2 hours total duration")
			}
		}
	}
	// =========================================================================

	// 1. Pre-fetch services and products to avoid N+1 queries
	svcIDs := make([]int64, 0, len(req.Bookings))
	prodIDs := make([]int64, 0)
	for _, b := range req.Bookings {
		svcIDs = append(svcIDs, b.ServiceID)
		for _, a := range b.Addons {
			prodIDs = append(prodIDs, a.ProductID)
		}
	}

	services, err := s.serviceRepo.GetByIDs(ctx, svcIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch services: %w", err)
	}
	svcMap := make(map[int64]*model.Service)
	for i := range services {
		svcMap[services[i].ServiceID] = &services[i]
	}

	products, err := s.productRepo.GetByIDs(ctx, prodIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch products: %w", err)
	}
	prodMap := make(map[int64]*model.Product)
	for i := range products {
		prodMap[products[i].ProductID] = &products[i]
	}

	// 2. Calculate totals and validate services
	var rawTotal float64
	bookingDetails := make([]struct {
		Service        *model.Service
		Req            model.CreateGroupBookingRequest
		CalculatedCost float64
		StartTime      time.Time
	}, len(req.Bookings))

	currentStartTime := *scheduledStart
	for i, bReq := range req.Bookings {
		svc, ok := svcMap[bReq.ServiceID]
		if !ok {
			return nil, fmt.Errorf("invalid service %d", bReq.ServiceID)
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
			product, ok := prodMap[addon.ProductID]
			if !ok {
				return nil, fmt.Errorf("invalid product %d", addon.ProductID)
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

	// Start transaction
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// 3. Create the BookingGroup
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

	// 4. Create individual Bookings and collect addons
	var createdBookings []model.Booking
	var allAddons []model.BookingAddon
	createdIDs := make([]int64, 0, len(bookingDetails))

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

		if err := s.bookingRepo.CreateTx(ctx, tx, booking); err != nil {
			return nil, fmt.Errorf("failed to create booking: %w", err)
		}
		createdIDs = append(createdIDs, booking.BookingID)

		// Collect Add-ons
		for _, addonReq := range bReq.Addons {
			product := prodMap[addonReq.ProductID]
			allAddons = append(allAddons, model.BookingAddon{
				BookingID:      booking.BookingID,
				ProductID:      addonReq.ProductID,
				Quantity:       addonReq.Quantity,
				PriceAtBooking: product.Price,
			})
		}
		createdBookings = append(createdBookings, *booking)
	}

	// 5. Bulk create addons
	if len(allAddons) > 0 {
		if err := s.addonRepo.CreateManyTx(ctx, tx, allAddons); err != nil {
			return nil, fmt.Errorf("failed to create addons: %w", err)
		}
	}

	// 6. Bulk enqueue bookings
	if err := s.queueRepo.EnqueueManyTx(ctx, tx, createdIDs); err != nil {
		return nil, fmt.Errorf("failed to enqueue bookings: %w", err)
	}

	// 7. Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	group.Bookings = createdBookings
	return group, nil
}

// GetGroupByID retrieves a booking group with its bookings.
func (s *BookingGroupService) GetGroupByID(ctx context.Context, groupID int64) (*model.BookingGroup, error) {
	return s.groupRepo.GetByIDWithBookings(ctx, groupID)
}
