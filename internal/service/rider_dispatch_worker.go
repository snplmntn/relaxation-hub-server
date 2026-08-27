package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

const (
	riderDispatchCandidateLimit = 50
	riderDispatchConfigTTL      = 5 * time.Minute
)

type rideDispatcher interface {
	RequestRide(ctx context.Context, ride *model.Ride) (*model.Ride, error)
	ExpireStaleOffers(ctx context.Context)
	RetryUnmatchedRides(ctx context.Context)
}

type RiderDispatchBookingRepository interface {
	ListRiderDispatchCandidates(ctx context.Context, start, end time.Time, limit int) ([]repository.RiderDispatchCandidate, error)
	GetPreviousBookingDropoffs(ctx context.Context, lookups []repository.PreviousDropoffLookup) (map[int64]repository.DispatchLatLong, error)
	GetBranchLocations(ctx context.Context, branchIDs []int64) (map[int64]repository.DispatchLatLong, error)
	GetAddressLocations(ctx context.Context, addressIDs []int64) (map[int64]repository.DispatchLatLong, error)
}

type riderDispatchConfig struct {
	dispatchBufferMinutes int
	vehicleType           string
	expiresAt             time.Time
}

// RiderDispatchWorker automates the dispatching of riders for scheduled bookings.
// It ensures riders are requested with enough lead time (Travel Time + Buffer).
type RiderDispatchWorker struct {
	bookingRepo    RiderDispatchBookingRepository
	rideService    *RideService
	rideDispatcher rideDispatcher
	routingService RoutingService
	db             db.DBTX // Direct DB access needed for config and efficient polling if not in repo
	pollInterval   time.Duration
	configCache    riderDispatchConfig
	now            func() time.Time
}

// NewRiderDispatchWorker creates a new RiderDispatchWorker.
func NewRiderDispatchWorker(br RiderDispatchBookingRepository, rs *RideService, routing RoutingService, db db.DBTX) *RiderDispatchWorker {
	return &RiderDispatchWorker{
		bookingRepo:    br,
		rideService:    rs,
		rideDispatcher: rs,
		routingService: routing,
		db:             db,
		pollInterval:   1 * time.Minute,
		now:            time.Now,
	}
}

// Start begins the background dispatch loop.
func (w *RiderDispatchWorker) Start(ctx context.Context) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("rider dispatch worker panic recovered", "error", r)
			}
		}()

		slog.Info("rider dispatch worker started")
		ticker := time.NewTicker(w.pollInterval)
		defer ticker.Stop()

		w.processOnce(ctx)

		for {
			select {
			case <-ctx.Done():
				slog.Info("rider dispatch worker stopping")
				return
			case <-ticker.C:
				w.processOnce(ctx)
				w.dispatcher().ExpireStaleOffers(ctx)
				w.dispatcher().RetryUnmatchedRides(ctx)
			}
		}
	}()
}

func (w *RiderDispatchWorker) Stop() {
	slog.Info("rider dispatch worker stopped")
}

func (w *RiderDispatchWorker) processOnce(ctx context.Context) {
	config := w.getConfig(ctx)

	// dispatchBufferMinutes is fetched but not used for triggering anymore
	// (we use 24h rule). We could use it for filtering too-tight bookings,
	// but for now, we just proceed.
	// dispatchBuffer := time.Duration(dispatchBufferMinutes) * time.Minute // Unused
	now := w.currentTime()

	// 2. Find Candidate Bookings
	// Look ahead: 12 hours (Same-Day Dispatch Rule)
	lookAhead := 12 * time.Hour
	startWindow := now
	endWindow := now.Add(lookAhead)

	candidates, err := w.bookingRepo.ListRiderDispatchCandidates(ctx, startWindow, endWindow, riderDispatchCandidateLimit)
	if err != nil {
		attrs := append([]any{"error", err}, db.PoolLogAttrs(w.db)...)
		slog.Error("rider dispatch worker: failed to query bookings", attrs...)
		return
	}

	prevDropoffs, branchLocations, addressLocations := w.loadDispatchLocations(ctx, candidates)
	processedBookings := make(map[int64]struct{}, len(candidates))

	for _, candidate := range candidates {
		if _, exists := processedBookings[candidate.BookingID]; exists {
			continue
		}
		processedBookings[candidate.BookingID] = struct{}{}

		var originLat, originLong float64
		var originAddress string

		if prevDropoff, ok := prevDropoffs[candidate.BookingID]; ok {
			originLat = prevDropoff.Lat
			originLong = prevDropoff.Long
			originAddress = "Previous Client Location"
		} else {
			if candidate.BranchID != nil {
				if pickup, ok := branchLocations[*candidate.BranchID]; ok {
					originLat = pickup.Lat
					originLong = pickup.Long
					originAddress = "Branch"
				}
			} else if candidate.HomeAddressID != nil {
				if pickup, ok := addressLocations[*candidate.HomeAddressID]; ok {
					originLat = pickup.Lat
					originLong = pickup.Long
					originAddress = "Therapist Home"
				}
			}
		}

		if originLat == 0 && originLong == 0 {
			slog.Warn("rider dispatch worker: cannot determine pickup for therapist", "therapist_id", candidate.TherapistID)
			continue
		}

		// 4. Calculate Travel Time (Still needed for ride data, even if not for trigger)
		route, err := w.routingService.GetRoute(ctx, originLat, originLong, candidate.ClientLat, candidate.ClientLong, config.vehicleType)
		if err != nil {
			slog.Error("rider dispatch worker: routing failed", "booking_id", candidate.BookingID, "error", err)
			continue
		}

		travelDuration := time.Duration(route.DurationSeconds) * time.Second

		// 5. Determine Dispatch Time
		// OLD LOGIC: Dispatch Time = ScheduledStart - (Travel Time + Buffer)
		// NEW LOGIC (Same-Day Rule): Dispatch if within 12 hours.

		timeUntilStart := candidate.ScheduledStart.Sub(now)
		if timeUntilStart <= 12*time.Hour {
			slog.Info("rider dispatch worker: triggering dispatch (within 12h)",
				"booking_id", candidate.BookingID,
				"time_until_start", timeUntilStart,
				"travel_duration", travelDuration, // Use variable to fix lint
				"vehicle", config.vehicleType,
				"scheduled", candidate.ScheduledStart,
			)

			req := &model.Ride{
				BookingID:      &candidate.BookingID,
				PassengerID:    candidate.TherapistID,
				RideType:       "outbound",
				PickupLat:      originLat,
				PickupLong:     originLong,
				PickupAddress:  originAddress,
				DropoffLat:     candidate.ClientLat,
				DropoffLong:    candidate.ClientLong,
				DropoffAddress: "Client Location",
				DistanceKm:     &[]float64{route.DistanceMeters / 1000.0}[0],
				Status:         "pending",
			}

			_, err = w.dispatcher().RequestRide(ctx, req)
			if err != nil {
				slog.Error("rider dispatch worker: failed to request ride", "booking_id", candidate.BookingID, "error", err)
			}
		} else {
			// This branch should theoretically not be reached due to query filter, but safe to keep
			// slog.Debug("rider dispatch worker: too early to dispatch", "booking_id", bookingID)
		}
	}
}

func (w *RiderDispatchWorker) dispatcher() rideDispatcher {
	if w.rideDispatcher != nil {
		return w.rideDispatcher
	}
	return w.rideService
}

func (w *RiderDispatchWorker) currentTime() time.Time {
	if w.now != nil {
		return w.now()
	}
	return time.Now()
}

func (w *RiderDispatchWorker) getConfig(ctx context.Context) riderDispatchConfig {
	now := w.currentTime()
	if w.configCache.vehicleType != "" && now.Before(w.configCache.expiresAt) {
		return w.configCache
	}

	config := riderDispatchConfig{dispatchBufferMinutes: 30, vehicleType: "motorcycle", expiresAt: now.Add(riderDispatchConfigTTL)}
	err := w.db.QueryRow(ctx, `
		SELECT dispatch_buffer_minutes, default_vehicle_type 
		FROM ride_pricing_config 
		ORDER BY config_id DESC LIMIT 1
	`).Scan(&config.dispatchBufferMinutes, &config.vehicleType)
	if err != nil {
		if err != pgx.ErrNoRows {
			slog.Warn("rider dispatch worker: failed to fetch config, using defaults", "error", err)
		}
		config.vehicleType = "motorcycle"
		config.dispatchBufferMinutes = 30
	}
	w.configCache = config
	return config
}

func (w *RiderDispatchWorker) loadDispatchLocations(ctx context.Context, candidates []repository.RiderDispatchCandidate) (map[int64]repository.DispatchLatLong, map[int64]repository.DispatchLatLong, map[int64]repository.DispatchLatLong) {
	lookups := make([]repository.PreviousDropoffLookup, 0, len(candidates))
	branchIDs := make([]int64, 0)
	addressIDs := make([]int64, 0)
	seenBranches := map[int64]struct{}{}
	seenAddresses := map[int64]struct{}{}

	for _, candidate := range candidates {
		lookups = append(lookups, repository.PreviousDropoffLookup{BookingID: candidate.BookingID, TherapistID: candidate.TherapistID, ScheduledStart: candidate.ScheduledStart})
		if candidate.BranchID != nil {
			if _, ok := seenBranches[*candidate.BranchID]; !ok {
				seenBranches[*candidate.BranchID] = struct{}{}
				branchIDs = append(branchIDs, *candidate.BranchID)
			}
		} else if candidate.HomeAddressID != nil {
			if _, ok := seenAddresses[*candidate.HomeAddressID]; !ok {
				seenAddresses[*candidate.HomeAddressID] = struct{}{}
				addressIDs = append(addressIDs, *candidate.HomeAddressID)
			}
		}
	}

	prevDropoffs, err := w.bookingRepo.GetPreviousBookingDropoffs(ctx, lookups)
	if err != nil {
		slog.Warn("rider dispatch worker: failed to batch previous booking lookup", "error", err)
		prevDropoffs = map[int64]repository.DispatchLatLong{}
	}
	branches, err := w.bookingRepo.GetBranchLocations(ctx, branchIDs)
	if err != nil {
		slog.Warn("rider dispatch worker: failed to batch branch lookup", "error", err)
		branches = map[int64]repository.DispatchLatLong{}
	}
	addresses, err := w.bookingRepo.GetAddressLocations(ctx, addressIDs)
	if err != nil {
		slog.Warn("rider dispatch worker: failed to batch address lookup", "error", err)
		addresses = map[int64]repository.DispatchLatLong{}
	}
	return prevDropoffs, branches, addresses
}
