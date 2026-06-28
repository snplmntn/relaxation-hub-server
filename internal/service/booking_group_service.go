package service

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

type groupBookingDetail struct {
	Service         *model.Service
	Req             model.CreateGroupBookingRequest
	DurationMinutes int
	ServiceSubtotal float64
	AddonsTotal     float64
	CalculatedCost  float64
	StartTime       time.Time
	AddonPrices     map[int64]float64
}

type groupPromotionResult struct {
	PromoID          *int64
	DiscountAmount   float64
	EligibleSubtotal float64
	AppliesTo        string
	Type             string
}

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
	branchRepo      repository.BranchRepository
	promoRepo       repository.PromotionRepository
	userRepo        voucherUserStore
	blocks          blockChecker
}

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
	branchRepo repository.BranchRepository,
	promoRepo repository.PromotionRepository,
	userRepo ...voucherUserStore,
) *BookingGroupService {
	var users voucherUserStore
	if len(userRepo) > 0 {
		users = userRepo[0]
	}
	// The injected user store is backed by the full UserRepository, which
	// supports block detection; assert to the narrow blockChecker surface.
	blocks, _ := users.(blockChecker)
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
		branchRepo:      branchRepo,
		promoRepo:       promoRepo,
		userRepo:        users,
		blocks:          blocks,
	}
}

func (s *BookingGroupService) CreateBookingGroup(ctx context.Context, clientID, actorID int64, req *model.CreateBookingGroupRequest) (*model.BookingGroup, error) {
	if req == nil || len(req.Bookings) == 0 {
		return nil, fmt.Errorf("at least one booking is required")
	}

	scheduledStart, err := parseGroupScheduledStart(req.ScheduledStart)
	if err != nil {
		return nil, err
	}

	if err := s.validateGroupLocation(ctx, clientID, req.AddressID, req.Bookings); err != nil {
		return nil, err
	}

	bookingDetails, rawTotal, servicesSubtotal, err := s.prepareBookingGroupDetails(ctx, *scheduledStart, req.Bookings)
	if err != nil {
		return nil, err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	promotionResult, err := s.resolveGroupPromotion(ctx, tx, clientID, req.VoucherCode, rawTotal, servicesSubtotal, true)
	if err != nil {
		return nil, err
	}

	totalDiscount := roundCurrency(promotionResult.DiscountAmount)
	allocatedDiscounts := allocateGroupDiscounts(bookingDetails, totalDiscount, promotionResult.AppliesTo)

	// The group's scheduled_start reflects the earliest child start. With per-child
	// (tandem) start times each booking can begin at a different moment, so the
	// top-level group time is the start of the overall visit window.
	groupStart := *scheduledStart
	for i := range bookingDetails {
		if bookingDetails[i].StartTime.Before(groupStart) {
			groupStart = bookingDetails[i].StartTime
		}
	}

	paymentMethod, pmErr := normalizePaymentMethod(req.PaymentMethod)
	if pmErr != nil {
		return nil, pmErr
	}

	group := &model.BookingGroup{
		ClientID:       clientID,
		AddressID:      req.AddressID,
		ScheduledStart: &groupStart,
		RawTotal:       rawTotal,
		Discount:       totalDiscount,
		FinalTotal:     roundCurrency(rawTotal - totalDiscount),
		PaymentMethod:  paymentMethod,
		Status:         "pending",
	}

	if err := s.groupRepo.CreateTx(ctx, tx, group); err != nil {
		return nil, fmt.Errorf("failed to create booking group: %w", err)
	}

	var createdBookings []model.Booking
	var allAddons []model.BookingAddon
	// Only children left without a pre-selected therapist are queued for
	// auto-assignment; children pinned to a chosen therapist (tandem) are
	// assigned directly below and must not be enqueued.
	unassignedIDs := make([]int64, 0, len(bookingDetails))

	for i := range bookingDetails {
		detail := bookingDetails[i]
		allocatedDiscount := allocatedDiscounts[i]
		finalTotal := roundCurrency(detail.CalculatedCost - allocatedDiscount)

		booking := &model.Booking{
			ClientID:        clientID,
			ServiceID:       &detail.Req.ServiceID,
			AddressID:       req.AddressID,
			PromoID:         promotionResult.PromoID,
			GenderPref:      strings.TrimSpace(detail.Req.GenderPref),
			PressurePref:    strings.TrimSpace(detail.Req.PressurePref),
			Notes:           strings.TrimSpace(detail.Req.Notes),
			DurationMinutes: detail.DurationMinutes,
			ScheduledStart:  &detail.StartTime,
			RawTotal:        float64Ptr(detail.CalculatedCost),
			Discount:        float64Ptr(allocatedDiscount),
			FinalTotal:      float64Ptr(finalTotal),
			Status:          "pending",
			GroupID:         &group.GroupID,
			GuestName:       detail.Req.GuestName,
			SequenceNumber:  detail.Req.SequenceNumber,
			StartCondition:  detail.Req.StartCondition,
			PaymentMethod:   paymentMethod,
		}

		if err := s.bookingRepo.CreateTx(ctx, tx, booking); err != nil {
			return nil, fmt.Errorf("failed to create booking: %w", err)
		}

		if detail.Req.TherapistID != nil {
			// Reject pinning a therapist that is blocked for this client.
			if berr := checkAssignmentBlock(ctx, s.blocks, clientID, *detail.Req.TherapistID); berr != nil {
				return nil, berr
			}
			// Pin the chosen therapist in-transaction. The repository performs
			// the guarded assign (active/accepting, offers the service, no
			// overlapping booking) so a conflict rolls back the whole group.
			if err := s.bookingRepo.AssignTherapistWithActorTx(ctx, tx, booking.BookingID, *detail.Req.TherapistID, actorID); err != nil {
				return nil, mapAssignError(err)
			}
			booking.TherapistID = detail.Req.TherapistID
			booking.Status = model.BookingStatusAssigned
		} else {
			unassignedIDs = append(unassignedIDs, booking.BookingID)
		}

		for _, addonReq := range detail.Req.Addons {
			priceAtBooking, ok := detail.AddonPrices[addonReq.ProductID]
			if !ok {
				return nil, fmt.Errorf("invalid product %d", addonReq.ProductID)
			}
			allAddons = append(allAddons, model.BookingAddon{
				BookingID:      booking.BookingID,
				ProductID:      addonReq.ProductID,
				Quantity:       addonReq.Quantity,
				PriceAtBooking: priceAtBooking,
			})
		}

		createdBookings = append(createdBookings, *booking)
	}

	if len(allAddons) > 0 {
		if err := s.addonRepo.CreateManyTx(ctx, tx, allAddons); err != nil {
			return nil, fmt.Errorf("failed to create addons: %w", err)
		}
	}

	if len(unassignedIDs) > 0 {
		if err := s.queueRepo.EnqueueManyTx(ctx, tx, unassignedIDs); err != nil {
			return nil, fmt.Errorf("failed to enqueue bookings: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	group.Bookings = createdBookings
	return group, nil
}

func (s *BookingGroupService) PreviewVoucher(ctx context.Context, clientID int64, req *model.CreateBookingGroupRequest) (*model.GroupVoucherPreviewResponse, error) {
	if req == nil || len(req.Bookings) == 0 {
		return nil, fmt.Errorf("at least one booking is required")
	}

	code := strings.TrimSpace(req.VoucherCode)
	if code == "" {
		return &model.GroupVoucherPreviewResponse{
			Valid:   false,
			Message: "Code required",
		}, nil
	}

	bookingDetails, rawTotal, servicesSubtotal, err := s.prepareBookingGroupDetails(ctx, time.Now().UTC(), req.Bookings)
	if err != nil {
		return nil, err
	}

	_ = bookingDetails

	promo, err := s.resolveGroupPromotion(ctx, nil, clientID, code, rawTotal, servicesSubtotal, false)
	if err != nil {
		if ve, ok := err.(*ValidationError); ok && (ve.Code == "invalid_voucher" || ve.Code == "vip_required") {
			return &model.GroupVoucherPreviewResponse{
				Valid:    false,
				Code:     code,
				RawTotal: rawTotal,
				Message:  ve.Message,
			}, nil
		}
		return nil, err
	}

	return &model.GroupVoucherPreviewResponse{
		Valid:            true,
		Code:             code,
		PromoID:          derefInt64(promo.PromoID),
		DiscountAmount:   roundCurrency(promo.DiscountAmount),
		EligibleSubtotal: roundCurrency(promo.EligibleSubtotal),
		RawTotal:         roundCurrency(rawTotal),
		FinalTotal:       roundCurrency(rawTotal - promo.DiscountAmount),
		AppliesTo:        promo.AppliesTo,
		Message:          "Promotion applied",
		Type:             promo.Type,
	}, nil
}

func (s *BookingGroupService) GetGroupByID(ctx context.Context, groupID int64) (*model.BookingGroup, error) {
	return s.groupRepo.GetByIDWithBookings(ctx, groupID)
}

func (s *BookingGroupService) prepareBookingGroupDetails(ctx context.Context, scheduledStart time.Time, bookings []model.CreateGroupBookingRequest) ([]groupBookingDetail, float64, float64, error) {
	svcIDs := make([]int64, 0, len(bookings))
	prodIDs := make([]int64, 0)
	for _, b := range bookings {
		svcIDs = append(svcIDs, b.ServiceID)
		for _, addon := range b.Addons {
			prodIDs = append(prodIDs, addon.ProductID)
		}
	}

	services, err := s.serviceRepo.GetByIDs(ctx, svcIDs)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to fetch services: %w", err)
	}
	svcMap := make(map[int64]*model.Service, len(services))
	for i := range services {
		svc := services[i]
		svcMap[svc.ServiceID] = &svc
	}

	products, err := s.productRepo.GetByIDs(ctx, prodIDs)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to fetch products: %w", err)
	}
	prodMap := make(map[int64]*model.Product, len(products))
	for i := range products {
		product := products[i]
		prodMap[product.ProductID] = &product
	}

	details := make([]groupBookingDetail, len(bookings))
	currentStartTime := scheduledStart
	var rawTotal float64
	var servicesSubtotal float64

	for i, req := range bookings {
		svc, ok := svcMap[req.ServiceID]
		if !ok {
			return nil, 0, 0, fmt.Errorf("invalid service %d", req.ServiceID)
		}

		duration := req.DurationMinutes
		if duration <= 0 {
			duration = svc.DurationMinutes
		}

		extraCost := 0.0
		if duration > svc.DurationMinutes && svc.DurationMinutes > 0 {
			diff := duration - svc.DurationMinutes
			ratePerMinute := svc.BasePrice / float64(svc.DurationMinutes)
			extraCost = ratePerMinute * float64(diff)
		}
		serviceSubtotal := roundCurrency(svc.BasePrice + extraCost)

		addonsTotal := 0.0
		for _, addon := range req.Addons {
			product, ok := prodMap[addon.ProductID]
			if !ok {
				return nil, 0, 0, fmt.Errorf("invalid product %d", addon.ProductID)
			}
			addonsTotal += product.Price * float64(addon.Quantity)
		}
		calculatedCost := roundCurrency(serviceSubtotal + addonsTotal)

		startTime := currentStartTime
		if strings.TrimSpace(req.ScheduledStart) != "" {
			// Tandem: each child carries its own explicit start, overriding the
			// fixed_time / after_previous sequencing.
			childStart, perr := parseGroupScheduledStart(req.ScheduledStart)
			if perr != nil {
				return nil, 0, 0, fmt.Errorf("invalid scheduled_start for booking %d: %w", i, perr)
			}
			startTime = *childStart
		} else if req.StartCondition == "after_previous" && i > 0 {
			prev := details[i-1]
			startTime = prev.StartTime.Add(time.Duration(prev.DurationMinutes) * time.Minute)
		}

		details[i] = groupBookingDetail{
			Service:         svc,
			Req:             req,
			DurationMinutes: duration,
			ServiceSubtotal: serviceSubtotal,
			AddonsTotal:     roundCurrency(addonsTotal),
			CalculatedCost:  calculatedCost,
			StartTime:       startTime,
			AddonPrices:     make(map[int64]float64, len(req.Addons)),
		}
		for _, addon := range req.Addons {
			details[i].AddonPrices[addon.ProductID] = prodMap[addon.ProductID].Price
		}

		rawTotal += calculatedCost
		servicesSubtotal += serviceSubtotal
	}

	return details, roundCurrency(rawTotal), roundCurrency(servicesSubtotal), nil
}

func (s *BookingGroupService) validateGroupLocation(ctx context.Context, clientID int64, addressID *int64, bookings []model.CreateGroupBookingRequest) error {
	if addressID == nil || s.locationService == nil || s.addressRepo == nil {
		return nil
	}

	address, err := s.addressRepo.GetByIDUnsafe(ctx, *addressID)
	if err != nil {
		return fmt.Errorf("invalid address: %w", err)
	}

	locationResult, err := s.locationService.CheckLocationByName(ctx, clientID, address.City, address.Barangay)
	if err != nil {
		slog.Warn("booking_group_service: location check failed", "error", err, "address_id", *addressID)
	} else if !locationResult.IsAllowed {
		return fmt.Errorf("%s", locationResult.Message)
	}

	if address.Latitude == nil || address.Longitude == nil {
		slog.Warn("booking_group_service: missing address coordinates, skipping distance rules", "address_id", *addressID)
		return nil
	}

	branchLat, branchLng, ok, err := s.getNearestActiveBranchCoordinates(ctx, *address.Latitude, *address.Longitude)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	distanceKm := s.locationService.GetDistanceKm(*address.Latitude, *address.Longitude, branchLat, branchLng)
	totalDurationMinutes := 0
	for _, b := range bookings {
		if b.DurationMinutes > 0 {
			totalDurationMinutes += b.DurationMinutes
		} else {
			totalDurationMinutes += 60
		}
	}

	if distanceKm > 10 && totalDurationMinutes < 180 {
		return fmt.Errorf("bookings over 10km away require a minimum of 3 hours total duration")
	}
	if distanceKm > 5 && totalDurationMinutes < 120 {
		return fmt.Errorf("bookings over 5km away require a minimum of 2 hours total duration")
	}

	return nil
}

func (s *BookingGroupService) getNearestActiveBranchCoordinates(ctx context.Context, customerLat, customerLng float64) (float64, float64, bool, error) {
	if s.branchRepo == nil || s.locationService == nil {
		slog.Warn("booking_group_service: branch repository unavailable, skipping distance rules")
		return 0, 0, false, nil
	}

	branches, err := s.branchRepo.List(ctx, true)
	if err != nil {
		return 0, 0, false, fmt.Errorf("failed to list branches: %w", err)
	}

	shortestDistance := math.MaxFloat64
	var nearestLat float64
	var nearestLng float64
	found := false

	for _, branch := range branches {
		if branch.Latitude == nil || branch.Longitude == nil {
			continue
		}
		distance := s.locationService.GetDistanceKm(customerLat, customerLng, *branch.Latitude, *branch.Longitude)
		if distance < shortestDistance {
			shortestDistance = distance
			nearestLat = *branch.Latitude
			nearestLng = *branch.Longitude
			found = true
		}
	}

	if !found {
		slog.Warn("booking_group_service: no active branches with coordinates found, skipping distance rules")
		return 0, 0, false, nil
	}

	return nearestLat, nearestLng, true, nil
}

func (s *BookingGroupService) resolveGroupPromotion(ctx context.Context, tx pgx.Tx, clientID int64, voucherCode string, rawTotal, servicesSubtotal float64, incrementUsage bool) (*groupPromotionResult, error) {
	code := strings.TrimSpace(voucherCode)
	if code == "" {
		return &groupPromotionResult{
			AppliesTo: model.PromotionAppliesToFullBasket,
		}, nil
	}
	if s.promoRepo == nil {
		return nil, NewValidationError("invalid_voucher", "voucher support is unavailable", map[string]string{"voucher_code": "promotion repository unavailable"})
	}
	if err := validateVoucherClient(ctx, s.userRepo, clientID); err != nil {
		return nil, err
	}

	promo, err := s.promoRepo.GetByCode(ctx, code)
	if err != nil {
		return nil, NewValidationError("invalid_voucher", "invalid voucher code", map[string]string{"voucher_code": "not found or expired"})
	}

	now := time.Now()
	if promo.ValidFrom != nil && promo.ValidFrom.After(now) {
		return nil, NewValidationError("invalid_voucher", "voucher not yet active", map[string]string{"voucher_code": "not yet active"})
	}
	if promo.ValidUntil != nil && promo.ValidUntil.Before(now) {
		return nil, NewValidationError("invalid_voucher", "voucher expired", map[string]string{"voucher_code": "expired"})
	}
	if promo.UsageLimit > 0 && promo.CurrentUses >= promo.UsageLimit {
		return nil, NewValidationError("invalid_voucher", "voucher fully redeemed", map[string]string{"voucher_code": "redemption limit reached"})
	}

	appliesTo := promo.AppliesTo
	if appliesTo == "" {
		appliesTo = model.PromotionAppliesToFullBasket
	}

	eligibleSubtotal := rawTotal
	if appliesTo == model.PromotionAppliesToServicesOnly {
		eligibleSubtotal = servicesSubtotal
	}

	discountAmount := 0.0
	promoType := ""
	if promo.DiscountAmount != nil && *promo.DiscountAmount > 0 {
		discountAmount = *promo.DiscountAmount
		promoType = "fixed"
	} else if promo.DiscountPct != nil && *promo.DiscountPct > 0 {
		discountAmount = eligibleSubtotal * float64(*promo.DiscountPct) / 100.0
		promoType = "percentage"
	}
	if discountAmount > eligibleSubtotal {
		discountAmount = eligibleSubtotal
	}

	if incrementUsage {
		ok, err := s.promoRepo.TryIncrementGlobalUsageTx(ctx, tx, promo.PromoID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, NewValidationError("invalid_voucher", "voucher fully redeemed", map[string]string{"voucher_code": "redemption limit reached"})
		}
		if _, err := s.promoRepo.TryIncrementUserPromoUsageTx(ctx, tx, promo.PromoID, clientID); err != nil {
			return nil, err
		}
	}

	return &groupPromotionResult{
		PromoID:          &promo.PromoID,
		DiscountAmount:   roundCurrency(discountAmount),
		EligibleSubtotal: roundCurrency(eligibleSubtotal),
		AppliesTo:        appliesTo,
		Type:             promoType,
	}, nil
}

func allocateGroupDiscounts(details []groupBookingDetail, totalDiscount float64, appliesTo string) []float64 {
	allocations := make([]float64, len(details))
	if len(details) == 0 || totalDiscount <= 0 {
		return allocations
	}

	type candidate struct {
		index      int
		base       float64
		floorCents int
		remainder  float64
	}

	candidates := make([]candidate, 0, len(details))
	totalBase := 0.0
	for i, detail := range details {
		base := detail.CalculatedCost
		if appliesTo == model.PromotionAppliesToServicesOnly {
			base = detail.ServiceSubtotal
		}
		totalBase += base
		candidates = append(candidates, candidate{index: i, base: base})
	}

	if totalBase <= 0 {
		return allocations
	}

	totalCents := int(math.Round(totalDiscount * 100))
	allocatedCents := 0
	for i := range candidates {
		exactShare := (float64(totalCents) * candidates[i].base) / totalBase
		candidates[i].floorCents = int(math.Floor(exactShare))
		candidates[i].remainder = exactShare - float64(candidates[i].floorCents)
		allocatedCents += candidates[i].floorCents
	}

	remainingCents := totalCents - allocatedCents
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].remainder == candidates[j].remainder {
			return candidates[i].index < candidates[j].index
		}
		return candidates[i].remainder > candidates[j].remainder
	})
	for i := 0; i < remainingCents && i < len(candidates); i++ {
		candidates[i].floorCents++
	}

	for _, candidate := range candidates {
		allocations[candidate.index] = float64(candidate.floorCents) / 100
	}
	return allocations
}

func parseGroupScheduledStart(value string) (*time.Time, error) {
	if value == "" {
		now := time.Now().UTC()
		return &now, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, fmt.Errorf("invalid scheduled_start: %w", err)
	}
	return &parsed, nil
}

// mapAssignError translates the guarded-assign errors returned by
// AssignTherapistWithActorTx into client-facing ValidationErrors, mirroring the
// single-booking admin path in booking_service.go.
func mapAssignError(err error) error {
	switch err {
	case repository.ErrTherapistNotFound:
		return NewValidationError("invalid_therapist", "specified therapist not found", map[string]string{"therapist_id": "not found"})
	case repository.ErrTherapistNotAccepting:
		return NewValidationError("therapist_not_accepting", "therapist is not accepting assignments", map[string]string{"therapist_id": "accept_assignments = false"})
	case repository.ErrAlreadyAssigned:
		return NewValidationError("cannot_assign", "therapist already assigned", map[string]string{"therapist_id": "already assigned"})
	case repository.ErrBookingNotAssignable:
		return NewValidationError("cannot_assign", "booking not in assignable state (status/payment)", map[string]string{"booking_id": "not assignable"})
	case repository.ErrAssignConflict:
		return NewValidationError("cannot_assign", "therapist is already booked for an overlapping time", map[string]string{"therapist_id": "overlapping booking"})
	case repository.ErrServiceNotOffered:
		return NewValidationError("service_not_offered", "therapist does not offer this service", map[string]string{"therapist_id": "does not offer service"})
	case pgx.ErrNoRows:
		return NewValidationError("cannot_assign", "therapist could not be assigned to booking", map[string]string{"therapist_id": "failed gating or already assigned"})
	default:
		return err
	}
}

func roundCurrency(value float64) float64 {
	return math.Round(value*100) / 100
}

func float64Ptr(value float64) *float64 {
	return &value
}

func derefInt64(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}
