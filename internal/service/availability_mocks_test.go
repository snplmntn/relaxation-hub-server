package service

import (
	"context"
	"time"
)

// Stubs for the availability additions to TherapistRepository / matching service.
// No existing test exercises these paths, so constant returns suffice — they only
// exist to keep the many local mocks implementing the interfaces.

func (m *mockTherapistRepoState) HasAvailableTherapists(ctx context.Context, windowStart, windowEnd time.Time, quantity int) (bool, error) {
	return false, nil
}
func (m *mockTherapistRepoForTest) HasAvailableTherapists(ctx context.Context, windowStart, windowEnd time.Time, quantity int) (bool, error) {
	return true, nil
}
func (m *mockTherapistRepoAdmin) HasAvailableTherapists(ctx context.Context, windowStart, windowEnd time.Time, quantity int) (bool, error) {
	return false, nil
}
func (m *mockTherapistRepoUnassign) HasAvailableTherapists(ctx context.Context, windowStart, windowEnd time.Time, quantity int) (bool, error) {
	return false, nil
}
func (m *MockTherapistRepository) HasAvailableTherapists(ctx context.Context, windowStart, windowEnd time.Time, quantity int) (bool, error) {
	return false, nil
}
func (n *noTher) HasAvailableTherapists(ctx context.Context, windowStart, windowEnd time.Time, quantity int) (bool, error) {
	return false, nil
}
func (n *nilTherapistRepo) HasAvailableTherapists(ctx context.Context, windowStart, windowEnd time.Time, quantity int) (bool, error) {
	return false, nil
}
func (r *lifecycleTherapistRepo) HasAvailableTherapists(ctx context.Context, windowStart, windowEnd time.Time, quantity int) (bool, error) {
	return false, nil
}
func (f *fakeTherapistRepoForLogistics) HasAvailableTherapists(ctx context.Context, windowStart, windowEnd time.Time, quantity int) (bool, error) {
	return false, nil
}

func (m *mockMatch) IsSlotAvailable(ctx context.Context, scheduledStart time.Time, durationMinutes, quantity int) (bool, error) {
	return true, nil
}
func (m *mockMatch) FindAlternativeSlots(ctx context.Context, scheduledStart time.Time, durationMinutes, quantity, limit int) ([]AvailabilitySlot, error) {
	return nil, nil
}
func (m *mockMatchState) IsSlotAvailable(ctx context.Context, scheduledStart time.Time, durationMinutes, quantity int) (bool, error) {
	return true, nil
}
func (m *mockMatchState) FindAlternativeSlots(ctx context.Context, scheduledStart time.Time, durationMinutes, quantity, limit int) ([]AvailabilitySlot, error) {
	return nil, nil
}
