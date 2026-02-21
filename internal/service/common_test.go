package service

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

// Shared nil mocks for service tests

type nilAssignmentQueueRepo struct{}

func (n *nilAssignmentQueueRepo) Enqueue(ctx context.Context, bookingID int64) error { return nil }
func (n *nilAssignmentQueueRepo) EnqueueTx(ctx context.Context, tx pgx.Tx, bookingID int64) error {
	return nil
}
func (n *nilAssignmentQueueRepo) EnqueueManyTx(ctx context.Context, tx pgx.Tx, bookingIDs []int64) error {
	return nil
}
func (n *nilAssignmentQueueRepo) DequeueBatch(ctx context.Context, limit int) ([]repository.QueueItem, error) {
	return nil, nil
}
func (n *nilAssignmentQueueRepo) Remove(ctx context.Context, bookingID int64) error { return nil }
func (n *nilAssignmentQueueRepo) IncrementAttempt(ctx context.Context, bookingID int64, attempts int, nextAttempt time.Time) error {
	return nil
}
func (n *nilAssignmentQueueRepo) UpdateWorkflowState(ctx context.Context, bookingID int64, state string, data map[string]interface{}) error {
	return nil
}

type nilPromoRepo struct{}

func (n *nilPromoRepo) Create(ctx context.Context, p *model.Promotion) error { return nil }
func (n *nilPromoRepo) ListActive(ctx context.Context, now time.Time) ([]model.Promotion, error) {
	return nil, nil
}
func (n *nilPromoRepo) GetByCode(ctx context.Context, code string) (*model.Promotion, error) {
	return nil, nil
}
func (n *nilPromoRepo) TryIncrementGlobalUsageTx(ctx context.Context, tx pgx.Tx, promoID int64) (bool, error) {
	return true, nil
}
func (n *nilPromoRepo) TryIncrementUserPromoUsageTx(ctx context.Context, tx pgx.Tx, promoID, userID int64) (bool, error) {
	return true, nil
}
func (n *nilPromoRepo) ListAll(ctx context.Context) ([]model.Promotion, error) { return nil, nil }
func (n *nilPromoRepo) Update(ctx context.Context, id int64, updates map[string]interface{}) error {
	return nil
}
func (n *nilPromoRepo) Delete(ctx context.Context, id int64) error { return nil }

type nilServiceRepo struct{}

func (n *nilServiceRepo) Create(ctx context.Context, svc *model.Service) error { return nil }
func (n *nilServiceRepo) GetByID(ctx context.Context, serviceID int64) (*model.Service, error) {
	return &model.Service{ServiceID: serviceID, BasePrice: 300, DurationMinutes: 60}, nil
}
func (n *nilServiceRepo) GetByIDs(ctx context.Context, ids []int64) ([]model.Service, error) {
	return nil, nil
}
func (n *nilServiceRepo) ListActive(ctx context.Context) ([]model.Service, error) { return nil, nil }
func (n *nilServiceRepo) ListRecentByUser(ctx context.Context, userID int64) ([]model.Service, error) {
	return nil, nil
}
func (n *nilServiceRepo) ListPopular(ctx context.Context) ([]model.Service, error) { return nil, nil }
func (n *nilServiceRepo) ListUnavailable(ctx context.Context) ([]model.Service, error) {
	return nil, nil
}

type nilAddressRepo struct{}

func (n *nilAddressRepo) Create(ctx context.Context, address *model.Address) error { return nil }
func (n *nilAddressRepo) GetByID(ctx context.Context, addressID int64) (*model.Address, error) {
	return nil, nil
}
func (n *nilAddressRepo) ListByUser(ctx context.Context, userID int64) ([]model.Address, error) {
	return nil, nil
}
func (n *nilAddressRepo) Delete(ctx context.Context, addressID int64) error             { return nil }
func (n *nilAddressRepo) SetDefault(ctx context.Context, userID, addressID int64) error { return nil }
func (n *nilAddressRepo) Update(ctx context.Context, address *model.Address) error      { return nil }

type nilUserRepo struct{}

func (n *nilUserRepo) Create(ctx context.Context, user *model.User) error { return nil }
func (n *nilUserRepo) GetByID(ctx context.Context, userID int64) (*model.User, error) {
	return nil, nil
}
func (n *nilUserRepo) GetByPhone(ctx context.Context, phone string) (*model.User, error) {
	return nil, nil
}
func (n *nilUserRepo) Update(ctx context.Context, user *model.User) error { return nil }
func (n *nilUserRepo) Delete(ctx context.Context, userID int64) error     { return nil }
func (n *nilUserRepo) CreateUserAndIdentity(ctx context.Context, user model.User, identity model.UserAuthIdentity) error {
	return nil
}
func (n *nilUserRepo) FindIdentityByKey(ctx context.Context, provider, key string) (*model.UserAuthIdentity, error) {
	return nil, nil
}
func (n *nilUserRepo) FindUserByID(ctx context.Context, userID int) (*model.User, error) {
	return nil, nil
}
func (n *nilUserRepo) UpdateUser(ctx context.Context, userID int64, updates map[string]interface{}) error {
	return nil
}
func (n *nilUserRepo) ListUsers(ctx context.Context, roleFilter string) ([]model.User, error) {
	return nil, nil
}
func (n *nilUserRepo) ListUsersPaginated(ctx context.Context, roleFilter string, limit, offset int) ([]model.User, int, error) {
	return nil, 0, nil
}
func (n *nilUserRepo) BlockUser(ctx context.Context, blockerID, blockedID int64) error   { return nil }
func (n *nilUserRepo) UnblockUser(ctx context.Context, blockerID, blockedID int64) error { return nil }
func (n *nilUserRepo) IsBlocked(ctx context.Context, userA, userB int64) (bool, error) {
	return false, nil
}
func (n *nilUserRepo) GetBlockList(ctx context.Context, userID int64) ([]repository.BlockedUserEntry, error) {
	return nil, nil
}
func (n *nilUserRepo) UpdateFCMToken(ctx context.Context, userID int64, token string) error {
	return nil
}
func (n *nilUserRepo) GetFCMToken(ctx context.Context, userID int64) (*string, error) {
	return nil, nil
}
func (n *nilUserRepo) GetUserInfoBatch(ctx context.Context, userIDs []int64) (map[int64]*repository.UserInfo, error) {
	return nil, nil
}
func (n *nilUserRepo) GetTherapistInfoBatch(ctx context.Context, therapistIDs []int64) (map[int64]*repository.TherapistInfo, error) {
	return nil, nil
}
func (n *nilUserRepo) AddFavoriteTherapist(ctx context.Context, userID, therapistID int64) error {
	return nil
}
func (n *nilUserRepo) RemoveFavoriteTherapist(ctx context.Context, userID, therapistID int64) error {
	return nil
}
func (n *nilUserRepo) ListFavoriteTherapists(ctx context.Context, userID int64) ([]model.User, error) {
	return nil, nil
}
func (n *nilUserRepo) IsTherapistFavorite(ctx context.Context, userID, therapistID int64) (bool, error) {
	return false, nil
}
func (n *nilUserRepo) BanUserSystem(ctx context.Context, userID int64, reason string) error {
	return nil
}
func (n *nilUserRepo) SuspendUserSystem(ctx context.Context, userID int64, reason string) error {
	return nil
}
func (n *nilUserRepo) SetOneSignalPlayerID(ctx context.Context, userID int64, playerID string) error {
	return nil
}

type nilTherapistRepo struct{}

func (n *nilTherapistRepo) CreateProfile(ctx context.Context, therapistID int64) error { return nil }
func (n *nilTherapistRepo) GetProfile(ctx context.Context, therapistID int64) (*model.TherapistProfile, error) {
	return nil, nil
}
func (n *nilTherapistRepo) UpdateProfile(ctx context.Context, therapistID int64, updates map[string]interface{}) error {
	return nil
}
func (n *nilTherapistRepo) List(ctx context.Context, availableOnly bool) ([]model.TherapistProfile, error) {
	return nil, nil
}
func (n *nilTherapistRepo) UploadDocument(ctx context.Context, doc *model.TherapistDocument) error {
	return nil
}
func (n *nilTherapistRepo) GetDocuments(ctx context.Context, therapistID int64) ([]model.TherapistDocument, error) {
	return nil, nil
}
func (n *nilTherapistRepo) VerifyDocument(ctx context.Context, documentID, verifierID int64, status string) error {
	return nil
}
func (n *nilTherapistRepo) AddService(ctx context.Context, ts *model.TherapistService) error {
	return nil
}
func (n *nilTherapistRepo) RemoveService(ctx context.Context, therapistID, serviceID int64) error {
	return nil
}
func (n *nilTherapistRepo) GetServices(ctx context.Context, therapistID int64) ([]int64, error) {
	return nil, nil
}
func (n *nilTherapistRepo) SetServicePressures(ctx context.Context, therapistID, serviceID int64, pressures []string) error {
	return nil
}
func (n *nilTherapistRepo) GetServicesWithPressures(ctx context.Context, therapistID int64) (map[int64][]string, error) {
	return nil, nil
}
func (n *nilTherapistRepo) FindAvailableByService(ctx context.Context, clientID int64, serviceID int64, genderPreference string, pressurePreference string) ([]model.TherapistProfile, error) {
	return nil, nil
}
func (n *nilTherapistRepo) FindAvailableByServiceWithTime(ctx context.Context, clientID int64, serviceID int64, genderPreference string, pressurePreference string, scheduledStart time.Time, durationMinutes int, lat *float64, lng *float64) ([]model.TherapistProfile, error) {
	return nil, nil
}
func (n *nilTherapistRepo) FindNearbyByService(ctx context.Context, clientID int64, serviceID int64, latitude float64, longitude float64, radiusKm float64, genderPreference string, pressurePreference string) ([]model.TherapistProfile, error) {
	return nil, nil
}
func (n *nilTherapistRepo) GetProfiles(ctx context.Context, therapistIDs []int64) ([]model.TherapistProfile, error) {
	return nil, nil
}
func (n *nilTherapistRepo) SetAtBranch(ctx context.Context, therapistID int64, atBranch bool) error {
	return nil
}
func (n *nilTherapistRepo) SetBatchServices(ctx context.Context, therapistID int64, serviceIDs []model.AddServiceWithPressuresRequest) error {
	return nil
}
func (n *nilTherapistRepo) TryLockTherapist(ctx context.Context, therapistID int64) (bool, error) {
	return true, nil
}
func (n *nilTherapistRepo) TryLockTherapistTx(ctx context.Context, tx pgx.Tx, therapistID int64) (bool, error) {
	return true, nil
}

type nilOfferRepo struct{}

func (n *nilOfferRepo) CreateOffer(ctx context.Context, offer *model.BookingOffer) error { return nil }
func (n *nilOfferRepo) CreateOfferTx(ctx context.Context, tx pgx.Tx, offer *model.BookingOffer) error {
	return nil
}
func (n *nilOfferRepo) GetOffer(ctx context.Context, offerID int64) (*model.BookingOffer, error) {
	return nil, nil
}
func (n *nilOfferRepo) GetActiveOffersForTherapist(ctx context.Context, therapistID int64) ([]model.BookingOffer, error) {
	return nil, nil
}
func (n *nilOfferRepo) AcceptOfferTx(ctx context.Context, tx pgx.Tx, offerID int64) error { return nil }
func (n *nilOfferRepo) ExpireOffersTx(ctx context.Context, tx pgx.Tx, bookingID int64) error {
	return nil
}
func (n *nilOfferRepo) ListByBooking(ctx context.Context, bookingID int64) ([]model.BookingOffer, error) {
	return nil, nil
}
func (n *nilOfferRepo) MarkAsRejected(ctx context.Context, offerID int64, reason string) error {
	return nil
}
func (n *nilOfferRepo) ListRecentEvents(ctx context.Context, limit int) ([]model.BookingOffer, error) {
	return nil, nil
}

type nilExtensionRequestRepo struct{}

func (n *nilExtensionRequestRepo) Create(ctx context.Context, req *model.ExtensionRequest) error {
	return nil
}
func (n *nilExtensionRequestRepo) GetByID(ctx context.Context, id int64) (*model.ExtensionRequest, error) {
	return nil, nil
}
func (n *nilExtensionRequestRepo) GetActiveByBookingID(ctx context.Context, bookingID int64) (*model.ExtensionRequest, error) {
	return nil, nil
}
func (n *nilExtensionRequestRepo) UpdateStatus(ctx context.Context, id int64, status string, responseAt *time.Time) error {
	return nil
}
func (n *nilExtensionRequestRepo) ListByBookingID(ctx context.Context, bookingID int64) ([]model.ExtensionRequest, error) {
	return nil, nil
}

// --- Helpers shared across service tests ---

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
