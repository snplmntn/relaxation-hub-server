package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"github.com/stretchr/testify/assert"
)

type fakeBookingRepoForDispatch struct {
	repository.BookingRepository
	candidates []repository.RiderDispatchCandidate
	prev       map[int64]repository.DispatchLatLong
	branches   map[int64]repository.DispatchLatLong
	addresses  map[int64]repository.DispatchLatLong

	lastLimit    int
	listCalls    int
	prevCalls    int
	branchCalls  int
	addressCalls int
}

func (f *fakeBookingRepoForDispatch) ListRiderDispatchCandidates(ctx context.Context, start, end time.Time, limit int) ([]repository.RiderDispatchCandidate, error) {
	f.listCalls++
	f.lastLimit = limit
	return f.candidates, nil
}

func (f *fakeBookingRepoForDispatch) GetPreviousBookingDropoffs(ctx context.Context, lookups []repository.PreviousDropoffLookup) (map[int64]repository.DispatchLatLong, error) {
	f.prevCalls++
	return f.prev, nil
}

func (f *fakeBookingRepoForDispatch) GetBranchLocations(ctx context.Context, branchIDs []int64) (map[int64]repository.DispatchLatLong, error) {
	f.branchCalls++
	return f.branches, nil
}

func (f *fakeBookingRepoForDispatch) GetAddressLocations(ctx context.Context, addressIDs []int64) (map[int64]repository.DispatchLatLong, error) {
	f.addressCalls++
	return f.addresses, nil
}

type fakeRideDispatcher struct {
	rides []model.Ride
}

func (f *fakeRideDispatcher) RequestRide(ctx context.Context, ride *model.Ride) (*model.Ride, error) {
	copy := *ride
	f.rides = append(f.rides, copy)
	return &copy, nil
}

func (f *fakeRideDispatcher) ExpireStaleOffers(ctx context.Context)   {}
func (f *fakeRideDispatcher) RetryUnmatchedRides(ctx context.Context) {}

type fakeRoutingForDispatch struct {
	calls int
}

func (f *fakeRoutingForDispatch) GetRoute(ctx context.Context, originLat, originLong, destLat, destLong float64, vehicleType string) (*RouteResult, error) {
	f.calls++
	return &RouteResult{DistanceMeters: 2400, DurationSeconds: 600}, nil
}

func TestRiderDispatchWorkerProcessOnce_UsesBoundedBatchedLookupsAndDedupesBookingRides(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, time.May, 11, 9, 0, 0, 0, time.UTC)
	branchID := int64(44)
	homeAddressID := int64(55)
	scheduledStart := now.Add(2 * time.Hour)
	bookingRepo := &fakeBookingRepoForDispatch{
		candidates: []repository.RiderDispatchCandidate{
			{BookingID: 100, TherapistID: 20, ScheduledStart: scheduledStart, ClientLat: 14.55, ClientLong: 121.02, BranchID: &branchID},
			{BookingID: 101, TherapistID: 21, ScheduledStart: scheduledStart.Add(time.Hour), ClientLat: 14.56, ClientLong: 121.03, HomeAddressID: &homeAddressID},
			{BookingID: 101, TherapistID: 21, ScheduledStart: scheduledStart.Add(time.Hour), ClientLat: 14.56, ClientLong: 121.03, HomeAddressID: &homeAddressID},
		},
		prev: map[int64]repository.DispatchLatLong{
			100: {Lat: 14.50, Long: 121.00},
		},
		branches: map[int64]repository.DispatchLatLong{
			branchID: {Lat: 14.60, Long: 121.06},
		},
		addresses: map[int64]repository.DispatchLatLong{
			homeAddressID: {Lat: 14.61, Long: 121.07},
		},
	}
	dispatcher := &fakeRideDispatcher{}
	routing := &fakeRoutingForDispatch{}
	worker := &RiderDispatchWorker{
		bookingRepo:    bookingRepo,
		rideDispatcher: dispatcher,
		routingService: routing,
		db:             &dispatchConfigDB{vehicleTypes: []string{"bicycle"}},
		now:            func() time.Time { return now },
	}

	worker.processOnce(ctx)

	assert.Equal(t, riderDispatchCandidateLimit, bookingRepo.lastLimit)
	assert.Equal(t, 1, bookingRepo.listCalls)
	assert.Equal(t, 1, bookingRepo.prevCalls)
	assert.Equal(t, 1, bookingRepo.branchCalls)
	assert.Equal(t, 1, bookingRepo.addressCalls)
	assert.Len(t, dispatcher.rides, 2)
	assert.Equal(t, 2, routing.calls)
	assert.Equal(t, int64(100), *dispatcher.rides[0].BookingID)
	assert.Equal(t, "outbound", dispatcher.rides[0].RideType)
	assert.Equal(t, "Previous Client Location", dispatcher.rides[0].PickupAddress)
	assert.Equal(t, int64(101), *dispatcher.rides[1].BookingID)
	assert.Equal(t, "Therapist Home", dispatcher.rides[1].PickupAddress)
}

func TestRiderDispatchWorkerProcessOnce_CachesConfigForFiveMinutesAndFallsBackOnErrors(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, time.May, 11, 9, 0, 0, 0, time.UTC)
	branchID := int64(44)
	scheduledStart := now.Add(time.Hour)
	bookingRepo := &fakeBookingRepoForDispatch{
		candidates: []repository.RiderDispatchCandidate{{BookingID: 100, TherapistID: 20, ScheduledStart: scheduledStart, ClientLat: 14.55, ClientLong: 121.02, BranchID: &branchID}},
		prev:       map[int64]repository.DispatchLatLong{},
		branches:   map[int64]repository.DispatchLatLong{branchID: {Lat: 14.60, Long: 121.06}},
		addresses:  map[int64]repository.DispatchLatLong{},
	}
	configDB := &dispatchConfigDB{vehicleTypes: []string{"bicycle", "van"}, scanErrs: []error{nil, errors.New("config unavailable")}}
	routing := &recordingRoutingForDispatch{}
	worker := &RiderDispatchWorker{
		bookingRepo:    bookingRepo,
		rideDispatcher: &fakeRideDispatcher{},
		routingService: routing,
		db:             configDB,
		now:            func() time.Time { return now },
	}

	worker.processOnce(ctx)
	now = now.Add(4 * time.Minute)
	worker.processOnce(ctx)
	now = now.Add(2 * time.Minute)
	worker.processOnce(ctx)

	assert.Equal(t, 2, configDB.scanCount)
	assert.Equal(t, []string{"bicycle", "bicycle", "motorcycle"}, routing.vehicleTypes)
}

type recordingRoutingForDispatch struct {
	vehicleTypes []string
}

func (r *recordingRoutingForDispatch) GetRoute(ctx context.Context, originLat, originLong, destLat, destLong float64, vehicleType string) (*RouteResult, error) {
	r.vehicleTypes = append(r.vehicleTypes, vehicleType)
	return &RouteResult{DistanceMeters: 1000, DurationSeconds: 300}, nil
}

type dispatchConfigDB struct {
	vehicleTypes []string
	scanErrs     []error
	scanCount    int
}

func (d *dispatchConfigDB) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (d *dispatchConfigDB) Query(ctx context.Context, sql string, arguments ...any) (pgx.Rows, error) {
	return nil, nil
}

func (d *dispatchConfigDB) QueryRow(ctx context.Context, sql string, arguments ...any) pgx.Row {
	return &dispatchConfigRow{db: d}
}

func (d *dispatchConfigDB) Begin(ctx context.Context) (pgx.Tx, error) { return nil, nil }

func (d *dispatchConfigDB) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	return 0, nil
}

func (d *dispatchConfigDB) SendBatch(ctx context.Context, batch *pgx.Batch) pgx.BatchResults {
	return nil
}

type dispatchConfigRow struct {
	db *dispatchConfigDB
}

func (r *dispatchConfigRow) Scan(dest ...any) error {
	index := r.db.scanCount
	r.db.scanCount++
	if index < len(r.db.scanErrs) && r.db.scanErrs[index] != nil {
		return r.db.scanErrs[index]
	}
	vehicleType := "motorcycle"
	if index < len(r.db.vehicleTypes) {
		vehicleType = r.db.vehicleTypes[index]
	}
	*dest[0].(*int) = 30
	*dest[1].(*string) = vehicleType
	return nil
}
