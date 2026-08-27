package service

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type bookingGroupTestGroupRepo struct {
	created *model.BookingGroup
}

type bookingGroupTestUserStore struct {
	user *model.User
}

func (r *bookingGroupTestUserStore) FindUserByID(_ context.Context, _ int) (*model.User, error) {
	return r.user, nil
}

func TestApplyGroupVIPDiscountUsesOnlyTheLargerDiscount(t *testing.T) {
	result := &groupPromotionResult{
		DiscountAmount: 80,
		AppliesTo:      model.PromotionAppliesToServicesOnly,
		Type:           "fixed",
	}
	vipDiscount := 100.0

	applyGroupVIPDiscount(result, &vipDiscount, 1000)

	assert.InDelta(t, 100, result.DiscountAmount, 0.0001)
	assert.Equal(t, model.PromotionAppliesToFullBasket, result.AppliesTo)
	assert.Equal(t, "vip", result.Type)

	result = &groupPromotionResult{DiscountAmount: 150, Type: "fixed"}
	applyGroupVIPDiscount(result, &vipDiscount, 1000)
	assert.InDelta(t, 150, result.DiscountAmount, 0.0001)
	assert.Equal(t, "fixed", result.Type)
}

func TestBookingGroupServiceGroupVIPDiscount(t *testing.T) {
	svc := &BookingGroupService{
		userRepo: &bookingGroupTestUserStore{
			user: &model.User{UserID: 9, Role: model.RoleClient, IsVIP: true},
		},
	}

	discount, err := svc.groupVIPDiscount(context.Background(), 9, 1250)

	require.NoError(t, err)
	require.NotNil(t, discount)
	assert.InDelta(t, 125, *discount, 0.0001)
}

func (r *bookingGroupTestGroupRepo) CreateTx(_ context.Context, _ pgx.Tx, g *model.BookingGroup) error {
	g.GroupID = 77
	r.created = cloneBookingGroup(g)
	return nil
}

func (r *bookingGroupTestGroupRepo) GetByID(_ context.Context, _ int64) (*model.BookingGroup, error) {
	return nil, nil
}

func (r *bookingGroupTestGroupRepo) GetByIDWithBookings(_ context.Context, _ int64) (*model.BookingGroup, error) {
	return nil, nil
}

func (r *bookingGroupTestGroupRepo) UpdateStatus(_ context.Context, _ int64, _ string) error {
	return nil
}

func (r *bookingGroupTestGroupRepo) ListByClient(_ context.Context, _ int64) ([]model.BookingGroup, error) {
	return nil, nil
}

type bookingGroupTestProductRepo struct {
	products []model.Product
}

func (r *bookingGroupTestProductRepo) Create(_ context.Context, _ *model.Product) error { return nil }
func (r *bookingGroupTestProductRepo) GetByID(_ context.Context, _ int64) (*model.Product, error) {
	return nil, nil
}
func (r *bookingGroupTestProductRepo) GetByIDs(_ context.Context, _ []int64) ([]model.Product, error) {
	return r.products, nil
}
func (r *bookingGroupTestProductRepo) ListActive(_ context.Context) ([]model.Product, error) {
	return r.products, nil
}
func (r *bookingGroupTestProductRepo) ListAll(_ context.Context) ([]model.Product, error) {
	return r.products, nil
}
func (r *bookingGroupTestProductRepo) Update(_ context.Context, _ *model.Product) error {
	return nil
}
func (r *bookingGroupTestProductRepo) Delete(_ context.Context, _ int64) error { return nil }

type bookingGroupTestAddonRepo struct{}

func (r *bookingGroupTestAddonRepo) Create(_ context.Context, _ *model.BookingAddon) error {
	return nil
}
func (r *bookingGroupTestAddonRepo) CreateTx(_ context.Context, _ pgx.Tx, _ *model.BookingAddon) error {
	return nil
}
func (r *bookingGroupTestAddonRepo) CreateManyTx(_ context.Context, _ pgx.Tx, _ []model.BookingAddon) error {
	return nil
}
func (r *bookingGroupTestAddonRepo) ListByBookingID(_ context.Context, _ int64) ([]model.BookingAddon, error) {
	return nil, nil
}
func (r *bookingGroupTestAddonRepo) ListByBookingIDWithProducts(_ context.Context, _ int64) ([]model.BookingAddon, error) {
	return nil, nil
}
func (r *bookingGroupTestAddonRepo) Delete(_ context.Context, _ int64) error { return nil }

type bookingGroupTestBranchRepo struct {
	branches []model.Branch
}

func (r *bookingGroupTestBranchRepo) Create(_ context.Context, _ *model.Branch) error { return nil }
func (r *bookingGroupTestBranchRepo) GetByID(_ context.Context, _ int64) (*model.Branch, error) {
	return nil, nil
}
func (r *bookingGroupTestBranchRepo) List(_ context.Context, _ bool) ([]model.Branch, error) {
	return r.branches, nil
}
func (r *bookingGroupTestBranchRepo) Update(_ context.Context, _ int64, _ map[string]interface{}) error {
	return nil
}

type bookingGroupTestServiceAreaRepo struct{}

func (r *bookingGroupTestServiceAreaRepo) GetByKey(_ context.Context, _ string) (*model.ServiceArea, error) {
	return nil, repository.ErrAreaNotFound
}
func (r *bookingGroupTestServiceAreaRepo) GetByName(_ context.Context, _ string, level model.ServiceAreaLevel) (*model.ServiceArea, error) {
	return &model.ServiceArea{
		AreaKey:           "city:test",
		Name:              "Test City",
		Level:             level,
		Status:            model.ServiceAreaStatusCovered,
		MinBookingMinutes: 60,
	}, nil
}
func (r *bookingGroupTestServiceAreaRepo) GetStatusByKey(_ context.Context, _ string) (model.ServiceAreaStatus, error) {
	return model.ServiceAreaStatusCovered, nil
}
func (r *bookingGroupTestServiceAreaRepo) ListByStatus(_ context.Context, _ model.ServiceAreaStatus) ([]model.ServiceArea, error) {
	return nil, nil
}
func (r *bookingGroupTestServiceAreaRepo) ListAll(_ context.Context) ([]model.ServiceArea, error) {
	return nil, nil
}
func (r *bookingGroupTestServiceAreaRepo) ListTopDemand(_ context.Context, _ int) ([]model.ServiceArea, error) {
	return nil, nil
}
func (r *bookingGroupTestServiceAreaRepo) UpdateStatus(_ context.Context, _ string, _ model.ServiceAreaStatus) error {
	return nil
}
func (r *bookingGroupTestServiceAreaRepo) UpsertArea(_ context.Context, _ *model.ServiceArea) error {
	return nil
}
func (r *bookingGroupTestServiceAreaRepo) RecordInterest(_ context.Context, _ int64, _ string) error {
	return nil
}
func (r *bookingGroupTestServiceAreaRepo) GetInterestCount(_ context.Context, _ string) (int, error) {
	return 0, nil
}
func (r *bookingGroupTestServiceAreaRepo) ListInterestedUsers(_ context.Context, _ string) ([]int64, error) {
	return nil, nil
}
func (r *bookingGroupTestServiceAreaRepo) ListInterestedUsersPage(_ context.Context, _ string, _ int, _ int) ([]model.AreaInterestedUser, int, error) {
	return nil, 0, nil
}

func TestBookingGroupServiceCreateBookingGroup_AppliesServicesOnlyVoucherAndAllocatesDiscount(t *testing.T) {
	dbtx := new(MockDBTX)
	tx := new(MockTx)
	groupRepo := &bookingGroupTestGroupRepo{}
	bookingRepo := new(MockBookingRepository)
	serviceRepo := new(MockServiceRepository)
	queueRepo := new(MockAssignmentQueueRepository)
	promoRepo := new(MockPromoRepository)

	dbtx.On("Begin", mock.Anything).Return(tx, nil).Once()
	tx.On("Rollback", mock.Anything).Return(nil).Once()
	tx.On("Commit", mock.Anything).Return(nil).Once()

	serviceRepo.On("GetByIDs", mock.Anything, []int64{1, 2}).Return([]model.Service{
		{ServiceID: 1, Name: "Swedish", BasePrice: 100, DurationMinutes: 60, IsActive: true, IsFeatured: true},
		{ServiceID: 2, Name: "Deep Tissue", BasePrice: 200, DurationMinutes: 90, IsActive: true, IsFeatured: true},
	}, nil).Once()

	productRepo := &bookingGroupTestProductRepo{
		products: []model.Product{
			{ProductID: 9, Name: "Oil", Price: 50},
		},
	}

	promoRepo.On("GetByCode", mock.Anything, "SAVE10").Return(&model.Promotion{
		PromoID:     55,
		Code:        "SAVE10",
		DiscountPct: intPtr(10),
		AppliesTo:   model.PromotionAppliesToServicesOnly,
		UsageLimit:  10,
		IsPublic:    true,
	}, nil).Once()
	promoRepo.On("TryIncrementGlobalUsageTx", mock.Anything, tx, int64(55)).Return(true, nil).Once()
	promoRepo.On("TryIncrementUserPromoUsageTx", mock.Anything, tx, int64(55), int64(999)).Return(true, nil).Once()

	var createdBookings []*model.Booking
	bookingRepo.On("CreateTx", mock.Anything, tx, mock.AnythingOfType("*model.Booking")).Return(nil).Twice().Run(func(args mock.Arguments) {
		booking := args.Get(2).(*model.Booking)
		booking.BookingID = int64(len(createdBookings) + 1)
		cloned := *booking
		createdBookings = append(createdBookings, &cloned)
	})

	queueRepo.On("EnqueueManyTx", mock.Anything, tx, []int64{1, 2}).Return(nil).Once()

	svc := NewBookingGroupService(
		dbtx,
		groupRepo,
		bookingRepo,
		&bookingGroupTestAddonRepo{},
		productRepo,
		serviceRepo,
		queueRepo,
		nil,
		nil,
		nil,
		promoRepo,
	)

	req := &model.CreateBookingGroupRequest{
		ScheduledStart: time.Date(2026, 4, 20, 9, 0, 0, 0, time.UTC).Format(time.RFC3339),
		PaymentMethod:  "cash",
		VoucherCode:    "SAVE10",
		Bookings: []model.CreateGroupBookingRequest{
			{
				ServiceID:       1,
				SequenceNumber:  0,
				StartCondition:  "fixed_time",
				DurationMinutes: 60,
				Addons: []model.CreateAddonRequest{
					{ProductID: 9, Quantity: 1},
				},
			},
			{
				ServiceID:       2,
				SequenceNumber:  1,
				StartCondition:  "after_previous",
				DurationMinutes: 90,
			},
		},
	}

	_, err := svc.CreateBookingGroup(context.Background(), 999, 1, req, true)
	require.NoError(t, err)

	require.NotNil(t, groupRepo.created)
	assert.InDelta(t, 350, groupRepo.created.RawTotal, 0.0001)
	assert.InDelta(t, 30, groupRepo.created.Discount, 0.0001)
	assert.InDelta(t, 320, groupRepo.created.FinalTotal, 0.0001)

	require.Len(t, createdBookings, 2)
	assert.Equal(t, int64(55), *createdBookings[0].PromoID)
	assert.InDelta(t, 150, *createdBookings[0].RawTotal, 0.0001)
	assert.InDelta(t, 10, *createdBookings[0].Discount, 0.0001)
	assert.InDelta(t, 140, *createdBookings[0].FinalTotal, 0.0001)

	assert.Equal(t, int64(55), *createdBookings[1].PromoID)
	assert.InDelta(t, 200, *createdBookings[1].RawTotal, 0.0001)
	assert.InDelta(t, 20, *createdBookings[1].Discount, 0.0001)
	assert.InDelta(t, 180, *createdBookings[1].FinalTotal, 0.0001)

	dbtx.AssertExpectations(t)
	tx.AssertExpectations(t)
	bookingRepo.AssertExpectations(t)
	serviceRepo.AssertExpectations(t)
	queueRepo.AssertExpectations(t)
	promoRepo.AssertExpectations(t)
}

func TestBookingGroupServiceCreateBookingGroup_UsesNearestActiveBranchForDistanceRule(t *testing.T) {
	dbtx := new(MockDBTX)
	tx := new(MockTx)
	groupRepo := &bookingGroupTestGroupRepo{}
	bookingRepo := new(MockBookingRepository)
	serviceRepo := new(MockServiceRepository)
	queueRepo := new(MockAssignmentQueueRepository)
	addressRepo := new(MockAddressRepository)

	addressLat := 14.6
	addressLng := 121.0
	addressRepo.On("GetByIDUnsafe", mock.Anything, int64(44)).Return(&model.Address{
		AddressID: 44,
		City:      "Test City",
		Barangay:  "Test Barangay",
		Latitude:  &addressLat,
		Longitude: &addressLng,
	}, nil).Once()

	dbtx.On("Begin", mock.Anything).Return(tx, nil).Once()
	tx.On("Rollback", mock.Anything).Return(nil).Once()
	tx.On("Commit", mock.Anything).Return(nil).Once()

	serviceRepo.On("GetByIDs", mock.Anything, []int64{1}).Return([]model.Service{
		{ServiceID: 1, Name: "Swedish", BasePrice: 100, DurationMinutes: 60, IsActive: true, IsFeatured: true},
	}, nil).Once()

	bookingRepo.On("CreateTx", mock.Anything, tx, mock.AnythingOfType("*model.Booking")).Return(nil).Once().Run(func(args mock.Arguments) {
		booking := args.Get(2).(*model.Booking)
		booking.BookingID = 1
	})
	queueRepo.On("EnqueueManyTx", mock.Anything, tx, []int64{1}).Return(nil).Once()

	nearActive := true
	svc := NewBookingGroupService(
		dbtx,
		groupRepo,
		bookingRepo,
		&bookingGroupTestAddonRepo{},
		&bookingGroupTestProductRepo{},
		serviceRepo,
		queueRepo,
		addressRepo,
		NewLocationService(&bookingGroupTestServiceAreaRepo{}),
		&bookingGroupTestBranchRepo{
			branches: []model.Branch{
				{
					BranchID:   1,
					BranchName: "Far Branch",
					Latitude:   floatPtr(14.0),
					Longitude:  floatPtr(121.5),
					IsActive:   &nearActive,
				},
				{
					BranchID:   2,
					BranchName: "Near Branch",
					Latitude:   floatPtr(14.6005),
					Longitude:  floatPtr(121.0005),
					IsActive:   &nearActive,
				},
			},
		},
		nil,
	)

	req := &model.CreateBookingGroupRequest{
		AddressID:      int64Ptr(44),
		PaymentMethod:  "cash",
		ScheduledStart: time.Date(2026, 4, 20, 9, 0, 0, 0, time.UTC).Format(time.RFC3339),
		Bookings: []model.CreateGroupBookingRequest{
			{ServiceID: 1, SequenceNumber: 0, StartCondition: "fixed_time", DurationMinutes: 60},
		},
	}

	group, err := svc.CreateBookingGroup(context.Background(), 999, 1, req, true)
	require.NoError(t, err)
	assert.Equal(t, int64(77), group.GroupID)

	addressRepo.AssertExpectations(t)
	dbtx.AssertExpectations(t)
	tx.AssertExpectations(t)
	bookingRepo.AssertExpectations(t)
	serviceRepo.AssertExpectations(t)
	queueRepo.AssertExpectations(t)
}

func TestBookingGroupServiceCreateBookingGroup_AssignsTandemTherapistsWithPerChildStart(t *testing.T) {
	dbtx := new(MockDBTX)
	tx := new(MockTx)
	groupRepo := &bookingGroupTestGroupRepo{}
	bookingRepo := new(MockBookingRepository)
	serviceRepo := new(MockServiceRepository)
	queueRepo := new(MockAssignmentQueueRepository)

	dbtx.On("Begin", mock.Anything).Return(tx, nil).Once()
	tx.On("Rollback", mock.Anything).Return(nil).Once()
	tx.On("Commit", mock.Anything).Return(nil).Once()

	serviceRepo.On("GetByIDs", mock.Anything, []int64{1, 2}).Return([]model.Service{
		{ServiceID: 1, Name: "Swedish", BasePrice: 100, DurationMinutes: 60, IsActive: true, IsFeatured: true},
		{ServiceID: 2, Name: "Deep Tissue", BasePrice: 200, DurationMinutes: 60, IsActive: true, IsFeatured: true},
	}, nil).Once()

	var createdBookings []*model.Booking
	bookingRepo.On("CreateTx", mock.Anything, tx, mock.AnythingOfType("*model.Booking")).Return(nil).Twice().Run(func(args mock.Arguments) {
		booking := args.Get(2).(*model.Booking)
		booking.BookingID = int64(len(createdBookings) + 1)
		cloned := *booking
		createdBookings = append(createdBookings, &cloned)
	})

	// Each child is pinned to its chosen therapist; actorID (the admin) is 5.
	bookingRepo.On("AssignTherapistWithActorTx", mock.Anything, tx, int64(1), int64(10), int64(5)).Return(nil).Once()
	bookingRepo.On("AssignTherapistWithActorTx", mock.Anything, tx, int64(2), int64(20), int64(5)).Return(nil).Once()

	svc := NewBookingGroupService(
		dbtx,
		groupRepo,
		bookingRepo,
		&bookingGroupTestAddonRepo{},
		&bookingGroupTestProductRepo{},
		serviceRepo,
		queueRepo,
		nil,
		nil,
		nil,
		nil,
	)

	child0Start := time.Date(2026, 4, 20, 9, 0, 0, 0, time.UTC)
	child1Start := time.Date(2026, 4, 20, 9, 30, 0, 0, time.UTC)
	req := &model.CreateBookingGroupRequest{
		// Top-level start is intentionally later than both children to prove the
		// group start is derived from the earliest child, not this value.
		ScheduledStart: time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC).Format(time.RFC3339),
		PaymentMethod:  "cash",
		Bookings: []model.CreateGroupBookingRequest{
			{ServiceID: 1, SequenceNumber: 0, StartCondition: "fixed_time", DurationMinutes: 60, TherapistID: int64Ptr(10), IsTherapistRequested: true, ScheduledStart: child0Start.Format(time.RFC3339)},
			{ServiceID: 2, SequenceNumber: 1, StartCondition: "fixed_time", DurationMinutes: 60, TherapistID: int64Ptr(20), ScheduledStart: child1Start.Format(time.RFC3339)},
		},
	}

	group, err := svc.CreateBookingGroup(context.Background(), 999, 5, req, true)
	require.NoError(t, err)

	// Group start reflects the earliest child, and each child keeps its own start.
	require.NotNil(t, group.ScheduledStart)
	assert.True(t, group.ScheduledStart.Equal(child0Start), "group start should be earliest child start")
	require.Len(t, createdBookings, 2)
	assert.True(t, createdBookings[0].ScheduledStart.Equal(child0Start))
	assert.True(t, createdBookings[1].ScheduledStart.Equal(child1Start))
	assert.True(t, createdBookings[0].IsTherapistRequested)
	assert.True(t, createdBookings[0].IsLocked)
	assert.False(t, createdBookings[1].IsTherapistRequested)
	assert.False(t, createdBookings[1].IsLocked)

	// All children are pre-assigned, so none are queued for auto-assignment.
	queueRepo.AssertNotCalled(t, "EnqueueManyTx", mock.Anything, mock.Anything, mock.Anything)

	dbtx.AssertExpectations(t)
	tx.AssertExpectations(t)
	bookingRepo.AssertExpectations(t)
	serviceRepo.AssertExpectations(t)
}

func TestBookingGroupServiceCreateBookingGroup_AssignConflictReturnsValidationError(t *testing.T) {
	dbtx := new(MockDBTX)
	tx := new(MockTx)
	groupRepo := &bookingGroupTestGroupRepo{}
	bookingRepo := new(MockBookingRepository)
	serviceRepo := new(MockServiceRepository)
	queueRepo := new(MockAssignmentQueueRepository)

	dbtx.On("Begin", mock.Anything).Return(tx, nil).Once()
	// The conflict aborts the transaction: rollback runs, commit never does.
	tx.On("Rollback", mock.Anything).Return(nil).Once()

	serviceRepo.On("GetByIDs", mock.Anything, []int64{1}).Return([]model.Service{
		{ServiceID: 1, Name: "Swedish", BasePrice: 100, DurationMinutes: 60, IsActive: true, IsFeatured: true},
	}, nil).Once()

	bookingRepo.On("CreateTx", mock.Anything, tx, mock.AnythingOfType("*model.Booking")).Return(nil).Once().Run(func(args mock.Arguments) {
		args.Get(2).(*model.Booking).BookingID = 1
	})
	bookingRepo.On("AssignTherapistWithActorTx", mock.Anything, tx, int64(1), int64(10), int64(5)).Return(repository.ErrAssignConflict).Once()

	svc := NewBookingGroupService(
		dbtx,
		groupRepo,
		bookingRepo,
		&bookingGroupTestAddonRepo{},
		&bookingGroupTestProductRepo{},
		serviceRepo,
		queueRepo,
		nil,
		nil,
		nil,
		nil,
	)

	req := &model.CreateBookingGroupRequest{
		ScheduledStart: time.Date(2026, 4, 20, 9, 0, 0, 0, time.UTC).Format(time.RFC3339),
		PaymentMethod:  "cash",
		Bookings: []model.CreateGroupBookingRequest{
			{ServiceID: 1, SequenceNumber: 0, StartCondition: "fixed_time", DurationMinutes: 60, TherapistID: int64Ptr(10)},
		},
	}

	_, err := svc.CreateBookingGroup(context.Background(), 999, 5, req, true)
	require.Error(t, err)
	ve, ok := err.(*ValidationError)
	require.True(t, ok, "expected a ValidationError, got %T", err)
	assert.Equal(t, "cannot_assign", ve.Code)

	dbtx.AssertExpectations(t)
	tx.AssertExpectations(t)
	bookingRepo.AssertExpectations(t)
	serviceRepo.AssertExpectations(t)
}

func cloneBookingGroup(g *model.BookingGroup) *model.BookingGroup {
	if g == nil {
		return nil
	}
	cloned := *g
	return &cloned
}

func intPtr(v int) *int {
	return &v
}

func floatPtr(v float64) *float64 {
	return &v
}

func int64Ptr(v int64) *int64 {
	return &v
}
