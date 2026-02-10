package model

import "time"

// RiderProfile represents the rider_profiles table.
type RiderProfile struct {
	RiderID            int64      `db:"rider_id" json:"rider_id"`
	UserID             int64      `db:"user_id" json:"user_id"`
	VehicleType        string     `db:"vehicle_type" json:"vehicle_type"`
	LicensePlate       string     `db:"license_plate" json:"license_plate"`
	LicenseNumber      string     `db:"license_number" json:"license_number"`
	IsOnline           bool       `db:"is_online" json:"is_online"`
	CurrentLat         *float64   `db:"-" json:"current_lat,omitempty"` // Derived from PostGIS geography
	CurrentLong        *float64   `db:"-" json:"current_long,omitempty"` // Derived from PostGIS geography
	LastLocationUpdate *time.Time `db:"last_location_update" json:"last_location_update,omitempty"`
	Rating             float64    `db:"rating" json:"rating"`
	TotalTrips         int        `db:"total_trips" json:"total_trips"`
	CreatedAt          time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt          time.Time  `db:"updated_at" json:"updated_at"`
}

// Ride represents the rides table.
type Ride struct {
	RideID             int64        `db:"ride_id" json:"ride_id"`
	RiderID            *int64       `db:"rider_id" json:"rider_id,omitempty"`
	PassengerID        int64        `db:"passenger_id" json:"passenger_id"`
	BookingID          *int64       `db:"booking_id" json:"booking_id,omitempty"`
	RideType           string       `db:"ride_type" json:"ride_type"` // "outbound" or "return"
	PickupLat          float64      `db:"pickup_lat" json:"pickup_lat"`
	PickupLong         float64      `db:"pickup_long" json:"pickup_long"`
	PickupAddress      string       `db:"pickup_address" json:"pickup_address"`
	DropoffLat         float64      `db:"dropoff_lat" json:"dropoff_lat"`
	DropoffLong        float64      `db:"dropoff_long" json:"dropoff_long"`
	DropoffAddress     string       `db:"dropoff_address" json:"dropoff_address"`
	DistanceKm         *float64     `db:"distance_km" json:"distance_km,omitempty"`
	PricingSnapshot    []byte       `db:"pricing_snapshot" json:"-"`
	Pricing            *RidePricing `db:"-" json:"pricing,omitempty"`
	Status             string       `db:"status" json:"status"`
	CreatedAt          time.Time    `db:"created_at" json:"created_at"`
	OfferedAt          *time.Time   `db:"offered_at" json:"offered_at,omitempty"`
	AcceptedAt         *time.Time   `db:"accepted_at" json:"accepted_at,omitempty"`
	ArrivedAt          *time.Time   `db:"arrived_at" json:"arrived_at,omitempty"`
	StartedAt          *time.Time   `db:"started_at" json:"started_at,omitempty"`
	CompletedAt        *time.Time   `db:"completed_at" json:"completed_at,omitempty"`
	CancelledAt        *time.Time   `db:"cancelled_at" json:"cancelled_at,omitempty"`
	CancellationReason *string      `db:"cancellation_reason" json:"cancellation_reason,omitempty"`
	UpdatedAt          time.Time    `db:"updated_at" json:"updated_at"`

    // Enriched fields
    RiderName    string `db:"-" json:"rider_name,omitempty"`
    RiderPhone   string `db:"-" json:"rider_phone,omitempty"`
    VehicleType  string `db:"-" json:"vehicle_type,omitempty"`
    LicensePlate string `db:"-" json:"license_plate,omitempty"`
}

// RidePricing represents the pricing structure snapshot.
type RidePricing struct {
	BaseRate        float64 `json:"base_rate"`
	PerKmRate       float64 `json:"per_km_rate"`
	SurgeMultiplier float64 `json:"surge_multiplier"`
	FinalFare       float64 `json:"final_fare"`
}

// PricingConfig represents the ride_pricing_config table.
type PricingConfig struct {
	ConfigID        int64     `db:"config_id" json:"config_id"`
	ConfigKey       string    `db:"config_key" json:"config_key"`
	BaseDistanceKm  float64   `db:"base_distance_km" json:"base_distance_km"`
	BaseRate        float64   `db:"base_rate" json:"base_rate"`
	PerKmRate       float64   `db:"per_km_rate" json:"per_km_rate"`
	Per100mRate     float64   `db:"per_100m_rate" json:"per_100m_rate"`
	MinFare         float64   `db:"min_fare" json:"min_fare"`
	MaxFare         float64   `db:"max_fare" json:"max_fare"`
	SurgeEnabled    bool      `db:"surge_enabled" json:"surge_enabled"`
	SurgeMultiplier float64   `db:"surge_multiplier" json:"surge_multiplier"`
	UpdatedAt       time.Time `db:"updated_at" json:"updated_at"`
}

// RideOffer represents a pending offer to a rider.
// Note: In the simplified model, offers might not be persisted in a separate table if we just broadcast them,
// but usually we track offers to prevent double booking.
// For now, we'll assume offers are ephemeral or tracked via Ride status 'offered' + assignments.
