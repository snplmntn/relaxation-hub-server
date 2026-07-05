package service

import (
	"context"
	"time"
)

// Stubs for the availability additions to TherapistRepository / matching service.
// No existing test exercises these paths, so constant returns suffice — they only
// exist to keep the many local mocks implementing the interfaces.

func (m *mockTherapistRepoState) HasAvailableTherapist(ctx context.Context, windowStart, windowEnd time.Time) (bool, error) {
	return false, nil
}
func (m *mockTherapistRepoForTest) HasAvailableTherapist(ctx context.Context, windowStart, windowEnd time.Time) (bool, error) {
	return true, nil
}
func (m *mockTherapistRepoAdmin) HasAvailableTherapist(ctx context.Context, windowStart, windowEnd time.Time) (bool, error) {
	return false, nil
}
func (m *mockTherapistRepoUnassign) HasAvailableTherapist(ctx context.Context, windowStart, windowEnd time.Time) (bool, error) {
	return false, nil
}
func (m *MockTherapistRepository) HasAvailableTherapist(ctx context.Context, windowStart, windowEnd time.Time) (bool, error) {
	return false, nil
}
func (n *noTher) HasAvailableTherapist(ctx context.Context, windowStart, windowEnd time.Time) (bool, error) {
	return false, nil
}
func (n *nilTherapistRepo) HasAvailableTherapist(ctx context.Context, windowStart, windowEnd time.Time) (bool, error) {
	return false, nil
}
func (r *lifecycleTherapistRepo) HasAvailableTherapist(ctx context.Context, windowStart, windowEnd time.Time) (bool, error) {
	return false, nil
}
func (f *fakeTherapistRepoForLogistics) HasAvailableTherapist(ctx context.Context, windowStart, windowEnd time.Time) (bool, error) {
	return false, nil
}

func (m *mockMatch) IsSlotAvailable(ctx context.Context, scheduledStart time.Time) (bool, error) {
	return true, nil
}
func (m *mockMatchState) IsSlotAvailable(ctx context.Context, scheduledStart time.Time) (bool, error) {
	return true, nil
}
