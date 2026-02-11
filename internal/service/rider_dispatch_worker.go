package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

// RiderDispatchWorker automates the dispatching of riders for scheduled bookings.
// It ensures riders are requested with enough lead time (Travel Time + Buffer).
type RiderDispatchWorker struct {
	bookingRepo   repository.BookingRepository
	rideService   *RideService
	routingService RoutingService
	db            db.DBTX // Direct DB access needed for config and efficient polling if not in repo
	pollInterval  time.Duration
}

// NewRiderDispatchWorker creates a new RiderDispatchWorker.
func NewRiderDispatchWorker(br repository.BookingRepository, rs *RideService, routing RoutingService, db db.DBTX) *RiderDispatchWorker {
	return &RiderDispatchWorker{
		bookingRepo:    br,
		rideService:    rs,
		routingService: routing,
		db:             db,
		pollInterval:   1 * time.Minute,
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
				w.rideService.ExpireStaleOffers(ctx)
				w.rideService.RetryUnmatchedRides(ctx)
			}
		}
	}()
}

func (w *RiderDispatchWorker) Stop() {
	slog.Info("rider dispatch worker stopped")
}

func (w *RiderDispatchWorker) processOnce(ctx context.Context) {
	// 1. Get Config
	var dispatchBufferMinutes int
	var vehicleType string
	
	// Default values
	dispatchBufferMinutes = 30
	vehicleType = "motorcycle"

	// Fetch unified config
	err := w.db.QueryRow(ctx, `
		SELECT dispatch_buffer_minutes, default_vehicle_type 
		FROM ride_pricing_config 
		ORDER BY config_id DESC LIMIT 1
	`).Scan(&dispatchBufferMinutes, &vehicleType)
	
	if err != nil && err != pgx.ErrNoRows {
		slog.Warn("rider dispatch worker: failed to fetch config, using defaults", "error", err)
	}

	// dispatchBufferMinutes is fetched but not used for triggering anymore 
	// (we use 24h rule). We could use it for filtering too-tight bookings, 
	// but for now, we just proceed.
	// dispatchBuffer := time.Duration(dispatchBufferMinutes) * time.Minute // Unused
	now := time.Now()

	// 2. Find Candidate Bookings
	// Look ahead: 12 hours (Same-Day Dispatch Rule)
	lookAhead := 12 * time.Hour
	startWindow := now
	endWindow := now.Add(lookAhead)

	query := `
		SELECT 
			b.booking_id, b.client_id, b.therapist_id, b.scheduled_start, 
			a.latitude, a.longitude,
			tp.branch_id, tp.home_address_id
		FROM bookings b
		JOIN addresses a ON b.address_id = a.address_id
		LEFT JOIN therapist_profiles tp ON b.therapist_id = tp.therapist_id
		LEFT JOIN rides r ON b.booking_id = r.booking_id
		JOIN services s ON b.service_id = s.service_id
		WHERE 
			b.status IN ('confirmed', 'assigned')
			AND s.category = 'home_service'
			AND b.scheduled_start BETWEEN $1 AND $2
			AND r.ride_id IS NULL -- No ride yet
			AND b.therapist_id IS NOT NULL
	`

	rows, err := w.db.Query(ctx, query, startWindow, endWindow)
	if err != nil {
		slog.Error("rider dispatch worker: failed to query bookings", "error", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var (
			bookingID     int64
			clientID      int64
			therapistID   int64
			scheduledStart time.Time
			clientLat     float64
			clientLong    float64
			branchID      *int64
			homeAddressID *int64
		)

		if err := rows.Scan(&bookingID, &clientID, &therapistID, &scheduledStart, &clientLat, &clientLong, &branchID, &homeAddressID); err != nil {
			slog.Error("rider dispatch worker: scan error", "error", err)
			continue
		}

		// 3. Determine Origin
		prevDropoff, errPrev := w.getPreviousBookingDropoff(ctx, therapistID, scheduledStart)
		if errPrev != nil {
			slog.Warn("rider dispatch worker: failed to get previous booking", "error", errPrev)
		}
		
		var originLat, originLong float64
		var originAddress string
		
		if prevDropoff != nil {
			originLat = prevDropoff.Lat
			originLong = prevDropoff.Long
			originAddress = "Previous Client Location"
		} else {
			// First ride: Branch or Home
			if branchID != nil {
				pickup, err := w.getLocationFromID(ctx, "branches", "branch_id", *branchID)
				if err == nil {
					originLat = pickup.Lat
					originLong = pickup.Long
					originAddress = "Branch"
				}
			} else if homeAddressID != nil {
				pickup, err := w.getLocationFromID(ctx, "addresses", "address_id", *homeAddressID)
				if err == nil {
					originLat = pickup.Lat
					originLong = pickup.Long
					originAddress = "Therapist Home"
				}
			}
		}

		if originLat == 0 && originLong == 0 {
			slog.Warn("rider dispatch worker: cannot determine pickup for therapist", "therapist_id", therapistID)
			continue
		}

		// 4. Calculate Travel Time (Still needed for ride data, even if not for trigger)
		route, err := w.routingService.GetRoute(ctx, originLat, originLong, clientLat, clientLong, vehicleType)
		if err != nil {
			slog.Error("rider dispatch worker: routing failed", "booking_id", bookingID, "error", err)
			continue
		}

		travelDuration := time.Duration(route.DurationSeconds) * time.Second
		
		// 5. Determine Dispatch Time
		// OLD LOGIC: Dispatch Time = ScheduledStart - (Travel Time + Buffer)
		// NEW LOGIC (Same-Day Rule): Dispatch if within 12 hours.
		
		timeUntilStart := time.Until(scheduledStart)
		if timeUntilStart <= 12*time.Hour {
			slog.Info("rider dispatch worker: triggering dispatch (within 12h)", 
				"booking_id", bookingID, 
				"time_until_start", timeUntilStart,
				"travel_duration", travelDuration, // Use variable to fix lint
				"vehicle", vehicleType,
				"scheduled", scheduledStart,
			)
			
			req := &model.Ride{
				BookingID:      &bookingID,
				PassengerID:    therapistID,
				PickupLat:      originLat,
				PickupLong:     originLong,
				PickupAddress:  originAddress,
				DropoffLat:     clientLat,
				DropoffLong:    clientLong,
				DropoffAddress: "Client Location",
				DistanceKm:     &[]float64{route.DistanceMeters / 1000.0}[0],
				Status:         "pending",
			}
			
			_, err = w.rideService.RequestRide(ctx, req)
			if err != nil {
				slog.Error("rider dispatch worker: failed to request ride", "booking_id", bookingID, "error", err)
			}
		} else {
			// This branch should theoretically not be reached due to query filter, but safe to keep
			// slog.Debug("rider dispatch worker: too early to dispatch", "booking_id", bookingID)
		}
	}
}

type LatLong struct {
	Lat  float64
	Long float64
}

func (w *RiderDispatchWorker) getPreviousBookingDropoff(ctx context.Context, therapistID int64, before time.Time) (*LatLong, error) {
	// Find the last completed/assigned booking for this therapist before the current one
	// Returns location of THAT booking
	query := `
		SELECT a.latitude, a.longitude
		FROM bookings b
		JOIN addresses a ON b.address_id = a.address_id
		WHERE b.therapist_id = $1
		  AND b.scheduled_start < $2
		ORDER BY b.scheduled_start DESC
		LIMIT 1
	`
	var l LatLong
	err := w.db.QueryRow(ctx, query, therapistID, before).Scan(&l.Lat, &l.Long)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &l, nil
}

func (w *RiderDispatchWorker) getLocationFromID(ctx context.Context, table, idCol string, id int64) (*LatLong, error) {
	var query string
	if table == "branches" {
		query = `SELECT latitude, longitude FROM branches WHERE branch_id = $1`
	} else if table == "addresses" {
		query = `SELECT latitude, longitude FROM addresses WHERE address_id = $1`
	} else {
		return nil, fmt.Errorf("unknown table %s", table)
	}
	
	var l LatLong
	err := w.db.QueryRow(ctx, query, id).Scan(&l.Lat, &l.Long)
	if err != nil {
		return nil, err
	}
	return &l, nil
}
