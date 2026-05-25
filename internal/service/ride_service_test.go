package service

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type rideRequestTestRepo struct {
	repository.RideRepository
	createdRide             *model.Ride
	createdProfileUserID    int64
	createdProfileType      string
	createdProfilePlate     string
	nextCreatedProfileID    int64
	claimedRideID           int64
	claimedRiderID          int64
	assignedRideID          int64
	assignedRiderID         int64
	updatedRideID           int64
	updatedStatus           string
	statusUpdateRideID      int64
	statusUpdateRiderID     int64
	statusUpdateStatus      string
	statusUpdateShouldError bool
	ride                    *model.Ride
	riderProfiles           map[int64]*model.RiderProfile
	profilesByRider         map[int64]*model.RiderProfile
}

func (r *rideRequestTestRepo) Create(ctx context.Context, ride *model.Ride) error {
	copy := *ride
	copy.RideID = 99
	ride.RideID = copy.RideID
	r.createdRide = &copy
	return nil
}

func (r *rideRequestTestRepo) ClaimRide(ctx context.Context, rideID, riderID int64) error {
	r.claimedRideID = rideID
	r.claimedRiderID = riderID
	return nil
}

func (r *rideRequestTestRepo) GetRiderProfile(ctx context.Context, userID int64) (*model.RiderProfile, error) {
	if profile, ok := r.riderProfiles[userID]; ok {
		return profile, nil
	}
	return nil, pgx.ErrNoRows
}

func (r *rideRequestTestRepo) CreateRiderProfile(ctx context.Context, userID int64, vehicleType, licensePlate string) error {
	if r.riderProfiles == nil {
		r.riderProfiles = make(map[int64]*model.RiderProfile)
	}
	profileID := r.nextCreatedProfileID
	if profileID == 0 {
		profileID = userID + 1000
	}
	profile := &model.RiderProfile{
		RiderID:      profileID,
		UserID:       userID,
		VehicleType:  vehicleType,
		LicensePlate: licensePlate,
	}
	r.riderProfiles[userID] = profile
	r.createdProfileUserID = userID
	r.createdProfileType = vehicleType
	r.createdProfilePlate = licensePlate
	return nil
}

func (r *rideRequestTestRepo) GetProfileByRiderID(ctx context.Context, riderID int64) (*model.RiderProfile, error) {
	if profile, ok := r.profilesByRider[riderID]; ok {
		return profile, nil
	}
	return nil, pgx.ErrNoRows
}

func (r *rideRequestTestRepo) GetByID(ctx context.Context, rideID int64) (*model.Ride, error) {
	if r.ride != nil {
		copy := *r.ride
		return &copy, nil
	}
	return &model.Ride{RideID: rideID, PassengerID: 22}, nil
}

func (r *rideRequestTestRepo) AssignRider(ctx context.Context, rideID, riderID int64) error {
	r.assignedRideID = rideID
	r.assignedRiderID = riderID
	return nil
}

func (r *rideRequestTestRepo) UpdateStatus(ctx context.Context, rideID int64, status string) error {
	r.updatedRideID = rideID
	r.updatedStatus = status
	return nil
}

func (r *rideRequestTestRepo) UpdateStatusForRider(ctx context.Context, rideID, riderID int64, status string) error {
	r.statusUpdateRideID = rideID
	r.statusUpdateRiderID = riderID
	r.statusUpdateStatus = status
	if r.statusUpdateShouldError {
		return repository.ErrRideNotFound
	}
	return nil
}

func TestRideServiceForceAssignRiderCreatesMissingRiderProfileForUserID(t *testing.T) {
	ctx := context.Background()
	riderUserID := int64(7892)
	riderProfileID := int64(9001)
	repo := &rideRequestTestRepo{
		nextCreatedProfileID: riderProfileID,
		riderProfiles:        map[int64]*model.RiderProfile{},
		ride:                 &model.Ride{RideID: 44, PassengerID: 22},
	}
	svc := NewRideService(
		repo,
		nil,
		NewRidePricingService(&missingPricingConfigDB{}),
		NewRideMatchingService(&missingPricingConfigDB{}),
		&missingPricingConfigDB{},
	)

	err := svc.ForceAssignRider(ctx, 44, riderUserID)

	require.NoError(t, err)
	assert.Equal(t, riderUserID, repo.createdProfileUserID)
	assert.Equal(t, "Unspecified", repo.createdProfileType)
	assert.Equal(t, "PENDING", repo.createdProfilePlate)
	assert.Equal(t, int64(44), repo.assignedRideID)
	assert.Equal(t, riderProfileID, repo.assignedRiderID)
	assert.Equal(t, int64(44), repo.updatedRideID)
	assert.Equal(t, "accepted", repo.updatedStatus)
}

type missingPricingConfigDB struct{}

func (d *missingPricingConfigDB) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (d *missingPricingConfigDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return &mockRows{}, nil
}

func (d *missingPricingConfigDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if strings.Contains(sql, "FROM ride_pricing_config") {
		return rowWithError{err: pgx.ErrNoRows}
	}
	if strings.Contains(sql, "ST_Distance") {
		return distanceRow{distanceKm: 4.25}
	}
	return rowWithError{err: pgx.ErrNoRows}
}

func (d *missingPricingConfigDB) Begin(ctx context.Context) (pgx.Tx, error) { return &mockTx{}, nil }

func (d *missingPricingConfigDB) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	return 0, nil
}

func (d *missingPricingConfigDB) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults {
	return &mockBatchResults{}
}

type spatialRefMissingDB struct {
	missingPricingConfigDB
}

func (d *spatialRefMissingDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if strings.Contains(sql, "FROM ride_pricing_config") {
		return rowWithError{err: pgx.ErrNoRows}
	}
	if strings.Contains(sql, "ST_Distance") {
		return rowWithError{err: &pgconn.PgError{
			Code:    "XX000",
			Message: "Cannot find SRID (4326) in spatial_ref_sys",
		}}
	}
	return rowWithError{err: pgx.ErrNoRows}
}

type rowWithError struct {
	err error
}

func (r rowWithError) Scan(dest ...any) error {
	return r.err
}

type distanceRow struct {
	distanceKm float64
}

func (r distanceRow) Scan(dest ...any) error {
	*dest[0].(*float64) = r.distanceKm
	return nil
}

func TestRideServiceRequestRideCreatesRideWhenPricingConfigMissing(t *testing.T) {
	ctx := context.Background()
	db := &missingPricingConfigDB{}
	repo := &rideRequestTestRepo{}
	svc := NewRideService(
		repo,
		nil,
		NewRidePricingService(db),
		NewRideMatchingService(db),
		db,
	)

	ride, err := svc.RequestRide(ctx, &model.Ride{
		PassengerID:    22,
		BookingID:      ptrInt64ForRideTest(11),
		RideType:       "outbound",
		PickupLat:      14.60,
		PickupLong:     121.04,
		PickupAddress:  "Therapist pickup",
		DropoffLat:     14.55,
		DropoffLong:    121.02,
		DropoffAddress: "Client address",
	})

	require.NoError(t, err)
	require.NotNil(t, ride)
	require.NotNil(t, repo.createdRide)
	assert.Equal(t, int64(99), ride.RideID)
	assert.Equal(t, "pending", repo.createdRide.Status)
	assert.NotNil(t, repo.createdRide.DistanceKm)
	assert.Equal(t, 4.25, *repo.createdRide.DistanceKm)
	assert.NotNil(t, repo.createdRide.Pricing)
	assert.NotEmpty(t, repo.createdRide.PricingSnapshot)
}

func TestRideServiceRequestRideCreatesRideWhenPostGISSpatialRefMissing(t *testing.T) {
	ctx := context.Background()
	db := &spatialRefMissingDB{}
	repo := &rideRequestTestRepo{}
	svc := NewRideService(
		repo,
		nil,
		NewRidePricingService(db),
		NewRideMatchingService(db),
		db,
	)

	ride, err := svc.RequestRide(ctx, &model.Ride{
		PassengerID:    22,
		BookingID:      ptrInt64ForRideTest(11),
		RideType:       "outbound",
		PickupLat:      14.60,
		PickupLong:     121.04,
		PickupAddress:  "Therapist pickup",
		DropoffLat:     14.55,
		DropoffLong:    121.02,
		DropoffAddress: "Client address",
	})

	require.NoError(t, err)
	require.NotNil(t, ride)
	require.NotNil(t, repo.createdRide)
	assert.Equal(t, int64(99), ride.RideID)
	assert.Equal(t, "pending", repo.createdRide.Status)
	require.NotNil(t, repo.createdRide.DistanceKm)
	assert.InDelta(t, 5.96, *repo.createdRide.DistanceKm, 0.1)
	assert.NotNil(t, repo.createdRide.Pricing)
	assert.NotEmpty(t, repo.createdRide.PricingSnapshot)
}

func TestRideServiceAcceptRideUsesRiderProfileIDForRideClaim(t *testing.T) {
	ctx := context.Background()
	riderUserID := int64(55)
	riderProfileID := int64(555)
	repo := &rideRequestTestRepo{riderProfiles: map[int64]*model.RiderProfile{
		riderUserID: {RiderID: riderProfileID, UserID: riderUserID},
	}}
	db := &missingPricingConfigDB{}
	svc := NewRideService(
		repo,
		nil,
		NewRidePricingService(db),
		NewRideMatchingService(db),
		db,
	)

	err := svc.AcceptRide(ctx, 99, riderUserID)

	require.NoError(t, err)
	assert.Equal(t, int64(99), repo.claimedRideID)
	assert.Equal(t, riderProfileID, repo.claimedRiderID)
}

func TestRideServiceUpdateRideStatusScopesUpdateToRiderProfileID(t *testing.T) {
	ctx := context.Background()
	riderUserID := int64(55)
	riderProfileID := int64(555)
	repo := &rideRequestTestRepo{
		riderProfiles: map[int64]*model.RiderProfile{
			riderUserID: {RiderID: riderProfileID, UserID: riderUserID},
		},
		profilesByRider: map[int64]*model.RiderProfile{
			riderProfileID: {RiderID: riderProfileID, UserID: riderUserID},
		},
		ride: &model.Ride{
			RideID:      99,
			RiderID:     &riderProfileID,
			PassengerID: 22,
		},
	}
	db := &missingPricingConfigDB{}
	svc := NewRideService(
		repo,
		nil,
		NewRidePricingService(db),
		NewRideMatchingService(db),
		db,
	)

	err := svc.UpdateRideStatus(ctx, 99, riderUserID, "arrived_pickup")

	require.NoError(t, err)
	assert.Equal(t, int64(99), repo.statusUpdateRideID)
	assert.Equal(t, riderProfileID, repo.statusUpdateRiderID)
	assert.Equal(t, "arrived_pickup", repo.statusUpdateStatus)
}

func ptrInt64ForRideTest(value int64) *int64 {
	return &value
}
