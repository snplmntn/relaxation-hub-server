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

type fakeRideRepoForLogistics struct {
	repository.RideRepository
	createdRide *model.Ride
}

type fakeBookingRepoForLogistics struct {
	repository.BookingRepository
	nextDestination *repository.BookingDetailsResult
	nextErr         error
}

func (f *fakeBookingRepoForLogistics) FindNextReturnDestinationBooking(ctx context.Context, therapistID, excludeBookingID int64, after time.Time) (*repository.BookingDetailsResult, error) {
	if f.nextErr != nil {
		return nil, f.nextErr
	}
	return f.nextDestination, nil
}

func (f *fakeRideRepoForLogistics) Create(ctx context.Context, ride *model.Ride) error {
	copy := *ride
	f.createdRide = &copy
	ride.RideID = 777
	ride.CreatedAt = time.Now().UTC()
	ride.UpdatedAt = ride.CreatedAt
	return nil
}

type fakeTherapistRepoForLogistics struct {
	repository.TherapistRepository
	profile *model.TherapistProfile
}

func (f *fakeTherapistRepoForLogistics) GetProfile(ctx context.Context, therapistID int64) (*model.TherapistProfile, error) {
	return f.profile, nil
}

type fakeAddressRepoForLogistics struct {
	repository.AddressRepository
	addresses map[int64]*model.Address
}

func (f *fakeAddressRepoForLogistics) GetByIDUnsafe(ctx context.Context, addressID int64) (*model.Address, error) {
	return f.addresses[addressID], nil
}

func TestLogisticsServiceScheduleReturnRide_SetsScheduledFor(t *testing.T) {
	ctx := context.Background()
	bookingID := int64(11)
	therapistID := int64(22)
	clientAddressID := int64(33)
	homeAddressID := int64(44)
	scheduledStart := time.Date(2026, time.May, 10, 15, 0, 0, 0, time.UTC)
	clientLat, clientLng := 14.55, 121.02
	homeLat, homeLng := 14.60, 121.04
	rideRepo := &fakeRideRepoForLogistics{}
	rideService := NewRideService(
		rideRepo,
		nil,
		NewRidePricingService(&mockDB{}),
		NewRideMatchingService(&mockDB{}),
		&mockDB{},
	)
	svc := &LogisticsService{
		rideService: rideService,
		bookingRepo: &fakeBookingRepoForLogistics{},
		therapistRepo: &fakeTherapistRepoForLogistics{profile: &model.TherapistProfile{
			TherapistID:   therapistID,
			HomeAddressID: &homeAddressID,
		}},
		addressRepo: &fakeAddressRepoForLogistics{addresses: map[int64]*model.Address{
			clientAddressID: {AddressID: clientAddressID, Street: "Client", City: "Makati", Latitude: &clientLat, Longitude: &clientLng},
			homeAddressID:   {AddressID: homeAddressID, Street: "Home", City: "Pasig", Latitude: &homeLat, Longitude: &homeLng},
		}},
	}

	err := svc.scheduleReturnRide(ctx, &model.Booking{
		BookingID:       bookingID,
		TherapistID:     &therapistID,
		AddressID:       &clientAddressID,
		ScheduledStart:  &scheduledStart,
		DurationMinutes: 90,
	})

	assert.NoError(t, err)
	if assert.NotNil(t, rideRepo.createdRide) && assert.NotNil(t, rideRepo.createdRide.ScheduledFor) {
		expected := scheduledStart.Add(120 * time.Minute)
		assert.Equal(t, expected, *rideRepo.createdRide.ScheduledFor)
	}
}

func TestLogisticsServiceBuildReturnRideOptions(t *testing.T) {
	ctx := context.Background()
	therapistID := int64(22)
	bookingID := int64(11)
	clientAddressID := int64(33)
	branchID := int64(44)
	homeAddressID := int64(55)
	returnTime := time.Date(2026, time.May, 10, 17, 0, 0, 0, time.UTC)
	branchAddressLine := "Branch Street"
	clientLat, clientLng := 14.55, 121.02
	nextLat, nextLng := 14.57, 121.04
	branchLat, branchLng := 14.60, 121.05
	homeLat, homeLng := 14.62, 121.06

	tests := []struct {
		name                string
		nextDestination     *repository.BookingDetailsResult
		profile             *model.TherapistProfile
		addresses           map[int64]*model.Address
		branch              *model.Branch
		expectedDestination model.ReturnRideDestination
		expectedLabel       string
		disabled            map[model.ReturnRideDestination]string
	}{
		{
			name: "defaults to next booking when it has coordinates",
			nextDestination: &repository.BookingDetailsResult{
				Booking: &model.Booking{BookingID: 99},
				Address: &model.Address{AddressID: 88, Label: "Next Client", Street: "Next Street", City: "Makati", Latitude: &nextLat, Longitude: &nextLng},
			},
			profile:             &model.TherapistProfile{TherapistID: therapistID, BranchID: &branchID, HomeAddressID: &homeAddressID},
			addresses:           map[int64]*model.Address{homeAddressID: {AddressID: homeAddressID, Street: "Home Street", City: "Pasig", Latitude: &homeLat, Longitude: &homeLng}},
			branch:              &model.Branch{BranchID: branchID, BranchName: "Central Branch", AddressLine: &branchAddressLine, Latitude: &branchLat, Longitude: &branchLng},
			expectedDestination: model.ReturnRideDestinationNextBooking,
			expectedLabel:       "Next booking",
		},
		{
			name:                "falls back to branch when no next booking is available",
			profile:             &model.TherapistProfile{TherapistID: therapistID, BranchID: &branchID, HomeAddressID: &homeAddressID},
			addresses:           map[int64]*model.Address{homeAddressID: {AddressID: homeAddressID, Street: "Home Street", City: "Pasig", Latitude: &homeLat, Longitude: &homeLng}},
			branch:              &model.Branch{BranchID: branchID, BranchName: "Central Branch", AddressLine: &branchAddressLine, Latitude: &branchLat, Longitude: &branchLng},
			expectedDestination: model.ReturnRideDestinationBranch,
			expectedLabel:       "Branch",
			disabled:            map[model.ReturnRideDestination]string{model.ReturnRideDestinationNextBooking: "No later booking with a mapped address"},
		},
		{
			name:                "falls back to home when next booking and branch are unavailable",
			profile:             &model.TherapistProfile{TherapistID: therapistID, BranchID: &branchID, HomeAddressID: &homeAddressID},
			addresses:           map[int64]*model.Address{homeAddressID: {AddressID: homeAddressID, Label: "Home", Street: "Home Street", City: "Pasig", Latitude: &homeLat, Longitude: &homeLng}},
			branch:              &model.Branch{BranchID: branchID, BranchName: "Central Branch"},
			expectedDestination: model.ReturnRideDestinationHome,
			expectedLabel:       "Home",
			disabled: map[model.ReturnRideDestination]string{
				model.ReturnRideDestinationNextBooking: "No later booking with a mapped address",
				model.ReturnRideDestinationBranch:      "Branch address has no coordinates",
			},
		},
		{
			name:                "keeps invalid options disabled with visible reasons",
			profile:             &model.TherapistProfile{TherapistID: therapistID, BranchID: &branchID, HomeAddressID: &homeAddressID},
			addresses:           map[int64]*model.Address{homeAddressID: {AddressID: homeAddressID, Street: "Home Street", City: "Pasig"}},
			branch:              &model.Branch{BranchID: branchID, BranchName: "Central Branch"},
			expectedDestination: "",
			disabled: map[model.ReturnRideDestination]string{
				model.ReturnRideDestinationNextBooking: "No later booking with a mapped address",
				model.ReturnRideDestinationBranch:      "Branch address has no coordinates",
				model.ReturnRideDestinationHome:        "Home address has no coordinates",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &LogisticsService{
				bookingRepo:   &fakeBookingRepoForLogistics{nextDestination: tt.nextDestination},
				therapistRepo: &fakeTherapistRepoForLogistics{profile: tt.profile},
				addressRepo: &fakeAddressRepoForLogistics{addresses: map[int64]*model.Address{
					clientAddressID: {AddressID: clientAddressID, Street: "Client Street", City: "Makati", Latitude: &clientLat, Longitude: &clientLng},
				}},
				db: &fakeBranchDBForLogistics{branch: tt.branch},
			}
			for id, address := range tt.addresses {
				svc.addressRepo.(*fakeAddressRepoForLogistics).addresses[id] = address
			}

			state, err := svc.buildReturnRideOptions(ctx, &model.Booking{
				BookingID:       bookingID,
				TherapistID:     &therapistID,
				AddressID:       &clientAddressID,
				ScheduledStart:  &returnTime,
				DurationMinutes: 90,
			}, returnTime)

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedDestination, state.Destination)
			assert.Equal(t, tt.expectedLabel, state.DestinationLabel)
			assert.Len(t, state.Options, 3)
			for _, option := range state.Options {
				if reason, ok := tt.disabled[option.Destination]; ok {
					assert.False(t, option.Available)
					assert.Equal(t, reason, option.DisabledReason)
				} else {
					assert.True(t, option.Available)
					assert.Empty(t, option.DisabledReason)
				}
			}
		})
	}
}

func TestLogisticsServiceBuildReturnRideOptions_PropagatesNextBookingLookupError(t *testing.T) {
	ctx := context.Background()
	therapistID := int64(22)
	bookingID := int64(11)
	clientAddressID := int64(33)
	returnTime := time.Date(2026, time.May, 10, 17, 0, 0, 0, time.UTC)
	expectedErr := errors.New("next booking lookup failed")
	clientLat, clientLng := 14.55, 121.02
	svc := &LogisticsService{
		bookingRepo: &fakeBookingRepoForLogistics{nextErr: expectedErr},
		therapistRepo: &fakeTherapistRepoForLogistics{profile: &model.TherapistProfile{
			TherapistID: therapistID,
		}},
		addressRepo: &fakeAddressRepoForLogistics{addresses: map[int64]*model.Address{
			clientAddressID: {AddressID: clientAddressID, Street: "Client Street", City: "Makati", Latitude: &clientLat, Longitude: &clientLng},
		}},
	}

	state, err := svc.buildReturnRideOptions(ctx, &model.Booking{
		BookingID:       bookingID,
		TherapistID:     &therapistID,
		AddressID:       &clientAddressID,
		ScheduledStart:  &returnTime,
		DurationMinutes: 90,
	}, returnTime)

	assert.Nil(t, state)
	assert.ErrorIs(t, err, expectedErr)
}

type fakeBranchDBForLogistics struct {
	branch *model.Branch
}

func (f *fakeBranchDBForLogistics) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (f *fakeBranchDBForLogistics) Query(ctx context.Context, sql string, arguments ...any) (pgx.Rows, error) {
	return nil, nil
}

func (f *fakeBranchDBForLogistics) QueryRow(ctx context.Context, sql string, arguments ...any) pgx.Row {
	return &fakeBranchRowForLogistics{branch: f.branch}
}

func (f *fakeBranchDBForLogistics) Begin(ctx context.Context) (pgx.Tx, error) { return nil, nil }

func (f *fakeBranchDBForLogistics) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	return 0, nil
}

func (f *fakeBranchDBForLogistics) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults {
	return nil
}

type fakeBranchRowForLogistics struct {
	branch *model.Branch
}

func (r *fakeBranchRowForLogistics) Scan(dest ...any) error {
	if r.branch == nil {
		return errors.New("branch not found")
	}
	*dest[0].(*int64) = r.branch.BranchID
	*dest[1].(*string) = r.branch.BranchName
	*dest[2].(**string) = r.branch.AddressLine
	*dest[3].(**string) = r.branch.City
	*dest[4].(**float64) = r.branch.Latitude
	*dest[5].(**float64) = r.branch.Longitude
	return nil
}
