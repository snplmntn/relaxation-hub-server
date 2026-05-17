package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/broadcaster"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

// AssignmentWorker picks unassigned bookings from a durable queue and attempts
// to match them to available therapists. It's designed to be resilient and
// idempotent so it can be run concurrently or restarted.
const (
	minAssignmentPollDelay = 5 * time.Second
	maxAssignmentPollDelay = 60 * time.Second
)

type AssignmentWorker struct {
	db                  db.DBTX
	queueRepo           repository.AssignmentQueueRepository
	bookingRepo         repository.BookingRepository
	paymentRepo         repository.PaymentRepository
	offerRepo           repository.BookingOfferRepository
	serviceRepo         repository.ServiceRepository
	areaRepo            repository.ServiceAreaRepository
	therapistRepo       repository.TherapistRepository // Injected
	matchService        TherapistMatchingService
	notificationService *NotificationService
	// opsNotifier is an optional hook to surface critical failures to ops.
	opsNotifier  func(ctx context.Context, subject string, details map[string]string) error
	pollInterval time.Duration
	batchSize    int
	maxAttempts  int
	baseBackoff  time.Duration
}

func NewAssignmentWorker(db db.DBTX, qr repository.AssignmentQueueRepository, br repository.BookingRepository, pr repository.PaymentRepository, or repository.BookingOfferRepository, sr repository.ServiceRepository, ar repository.ServiceAreaRepository, tr repository.TherapistRepository, ms TherapistMatchingService, ns *NotificationService, opsNotifier func(ctx context.Context, subject string, details map[string]string) error) *AssignmentWorker {
	return &AssignmentWorker{
		db:                  db,
		queueRepo:           qr,
		bookingRepo:         br,
		paymentRepo:         pr,
		offerRepo:           or,
		serviceRepo:         sr,
		areaRepo:            ar,
		therapistRepo:       tr,
		matchService:        ms,
		notificationService: ns,
		opsNotifier:         opsNotifier,
		pollInterval:        5 * time.Second,
		batchSize:           10,
		maxAttempts:         5,
		baseBackoff:         30 * time.Second,
	}
}

func (w *AssignmentWorker) Start(ctx context.Context) {
	go func() {
		// recover from panics to ensure worker doesn't silently die
		defer func() {
			if r := recover(); r != nil {
				slog.Error("assignment worker: panic recovered", "panic", r)
				if w.opsNotifier != nil {
					_ = w.opsNotifier(ctx, "assignment_worker: panic", map[string]string{"panic": fmt.Sprint(r)})
				}
			}
		}()

		w.run(ctx)
	}()
}

func (w *AssignmentWorker) run(ctx context.Context) {
	slog.Info("assignment worker started")
	delay := w.pollInterval
	if delay <= 0 {
		delay = minAssignmentPollDelay
	}

	for {
		if !waitForNextAssignmentPoll(ctx, delay) {
			slog.Info("assignment worker stopping")
			return
		}

		processed := w.processOnce(ctx)
		delay = w.nextPollDelay(processed, delay)
	}
}

func waitForNextAssignmentPoll(ctx context.Context, delay time.Duration) bool {
	if delay <= 0 {
		delay = minAssignmentPollDelay
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func nextAssignmentPollDelay(processed int, current time.Duration) time.Duration {
	return nextPollDelay(processed, current, minAssignmentPollDelay)
}

func (w *AssignmentWorker) nextPollDelay(processed int, current time.Duration) time.Duration {
	minimum := w.pollInterval
	if minimum <= 0 || minimum == minAssignmentPollDelay {
		return nextAssignmentPollDelay(processed, current)
	}
	return nextPollDelay(processed, current, minimum)
}

func nextPollDelay(processed int, current, minimum time.Duration) time.Duration {
	if minimum <= 0 {
		minimum = minAssignmentPollDelay
	}
	if processed > 0 || current < minimum {
		return minimum
	}

	next := current * 2
	if next > maxAssignmentPollDelay {
		return maxAssignmentPollDelay
	}
	return next
}

func (w *AssignmentWorker) Stop() {
	// Context cancellation is handled by the caller (main.go) via the context passed to Start
	// But providing a explicit Stop method is good practice if we need custom cleanup.
	slog.Info("assignment worker stopped")
}

func (w *AssignmentWorker) processOnce(ctx context.Context) int {
	items, err := w.queueRepo.DequeueBatch(ctx, w.batchSize)
	if err != nil {
		slog.Error("assignment worker: fail dequeue batch", "error", err)
		if w.opsNotifier != nil {
			_ = w.opsNotifier(ctx, "assignment_worker: dequeue_failed", map[string]string{"error": err.Error()})
		}
		return 0
	}
	if len(items) == 0 {
		return 0
	}

	// 1. Pre-fetch Contextual Data in Batch
	bookingIDs := make([]int64, len(items))
	groupIDsMap := make(map[int64]bool)
	for i, it := range items {
		bookingIDs[i] = it.BookingID
	}

	// Fetch booking details
	detailsMap, err := w.bookingRepo.GetBookingWithDetailsBatch(ctx, bookingIDs)
	if err != nil {
		slog.Warn("assignment worker: failed to batch-fetch booking details", "error", err)
		detailsMap = make(map[int64]*repository.BookingDetailsResult)
	}

	// Identify groups and cities for further batching
	cityMap := make(map[string]bool)
	for _, d := range detailsMap {
		if d.Booking.GroupID != nil {
			groupIDsMap[*d.Booking.GroupID] = true
		}
		if d.Address != nil && (d.Address.Latitude == nil || d.Address.Longitude == nil) && d.Address.City != "" {
			cityMap[d.Address.City] = true
		}
	}

	// Fetch siblings for all groups in the batch
	var groupIDs []int64
	for gid := range groupIDsMap {
		groupIDs = append(groupIDs, gid)
	}
	siblingsMap, err := w.bookingRepo.GetByGroupIDs(ctx, groupIDs)
	if err != nil {
		slog.Warn("assignment worker: failed to batch-fetch siblings", "error", err)
		siblingsMap = make(map[int64][]model.Booking)
	}

	// Fetch ServiceAreas for cities missing coords
	areasByCity := make(map[string]*model.ServiceArea)
	if w.areaRepo != nil && len(cityMap) > 0 {
		for city := range cityMap {
			area, err := w.areaRepo.GetByName(ctx, city, model.ServiceAreaLevelCity)
			if err == nil && area != nil {
				areasByCity[city] = area
			}
		}
	}

	// Fetch active offers
	activeOffersByBooking, err := w.offerRepo.GetActiveOffersBatch(ctx, bookingIDs)
	if err != nil {
		slog.Warn("assignment worker: failed to batch-fetch active offers", "error", err)
		activeOffersByBooking = make(map[int64][]model.BookingOffer)
	}

	// 2. Process Items
	for _, it := range items {
		bid := it.BookingID
		details := detailsMap[bid]
		if details == nil || details.Booking == nil {
			slog.Warn("assignment worker: booking detail not found", "booking_id", bid)
			if err := w.queueRepo.Remove(ctx, bid); err != nil {
				slog.Warn("assignment worker: failed to remove booking from queue", "booking_id", bid, "error", err)
			}
			continue
		}
		b := details.Booking

		if it.WorkflowState == "" {
			it.WorkflowState = "init"
		}

		// Check for expired offers first
		if w.offerRepo != nil {
			expired, err := w.offerRepo.ExpireOffers(ctx, bid)
			if err != nil {
				slog.Warn("assignment worker: failed to expire offers", "booking_id", bid, "error", err)
			} else if len(expired) > 0 {
				for _, off := range expired {
					if err := broadcaster.BroadcastToUser(off.TherapistID, "offer_expired", map[string]interface{}{
						"offer_id":     off.OfferID,
						"booking_id":   off.BookingID,
						"therapist_id": off.TherapistID,
					}); err != nil {
						slog.Warn("assignment worker: failed to broadcast offer expiry", "offer_id", off.OfferID, "therapist_id", off.TherapistID, "error", err)
					}
				}
				// Refresh active offers if some expired
				if offers, err := w.offerRepo.GetActiveOffers(ctx, bid); err != nil {
					slog.Warn("assignment worker: failed to refresh active offers", "booking_id", bid, "error", err)
				} else {
					activeOffersByBooking[bid] = offers
				}
			}
		}

		// State Machine Loop
		for {
			transitioned := false
			transition := func(newState string, data map[string]interface{}) {
				if err := w.queueRepo.UpdateWorkflowState(ctx, bid, newState, data); err != nil {
					slog.Warn("assignment worker: failed to transition", "booking_id", bid, "new_state", newState, "error", err)
				} else {
					it.WorkflowState = newState
					transitioned = true
				}
			}

			switch it.WorkflowState {
			case "init":
				if b.GroupID != nil {
					siblings := siblingsMap[*b.GroupID]
					isParallel := true
					if len(siblings) > 0 {
						firstStart := siblings[0].ScheduledStart
						for _, sib := range siblings {
							if sib.ScheduledStart == nil || firstStart == nil || !sib.ScheduledStart.Equal(*firstStart) {
								isParallel = false
								break
							}
						}
					}
					if isParallel {
						transition("group_locking", nil)
					} else {
						transition("sequence_bundling", nil)
					}
				} else {
					transition("matching", nil)
				}

			case "group_locking":
				if b.GroupID == nil {
					transition("matching", nil)
					break
				}
				siblings := siblingsMap[*b.GroupID]
				allMatchable := true
				for _, sib := range siblings {
					if sib.Status != "pending" {
						continue
					}
					// Check matchability (simplified/cached check might be needed if matchService is too slow)
					start := time.Now()
					if sib.ScheduledStart != nil {
						start = *sib.ScheduledStart
					}
					// Note: matchService currently doesn't have batching, so this is still individual
					// but we've avoided repo calls to get sib details.
					if sib.ServiceID == nil {
						allMatchable = false
						break
					}
					cands, err := w.matchService.FindAvailableTherapistsForServiceWithTime(
						ctx, sib.ClientID, *sib.ServiceID, sib.GenderPref, sib.PressurePref, start, sib.DurationMinutes, nil, nil,
					)
					if err != nil || len(cands) == 0 {
						allMatchable = false
						break
					}
				}
				if !allMatchable {
					if err := w.queueRepo.IncrementAttempt(ctx, bid, it.Attempts, time.Now().Add(5*time.Minute)); err != nil {
						slog.Warn("assignment worker: failed to increment attempt", "booking_id", bid, "error", err)
					}
					break
				}
				transition("matching", nil)

			case "matching":
				// DEBUG Panic Tracking
				slog.Info("assignment worker: MATCHING step start", "booking_id", bid)

				var lat, lng *float64
				if details.Address != nil {
					slog.Info("assignment worker: using booking address", "booking_id", bid)
					lat, lng = details.Address.Latitude, details.Address.Longitude
				}
				if lat == nil && details.Address != nil && details.Address.City != "" {
					slog.Info("assignment worker: attempting city fallback", "booking_id", bid, "city", details.Address.City)
					if area := areasByCity[details.Address.City]; area != nil {
						slog.Info("assignment worker: city found", "booking_id", bid)
						lat, lng = area.Lat, area.Lng
					}
				}

				start := time.Now()
				if b.ScheduledStart != nil {
					start = *b.ScheduledStart
				}

				if b.ServiceID == nil {
					slog.Warn("assignment worker: MATCHING invalid serviceID", "booking_id", bid)
					_ = w.queueRepo.Remove(ctx, bid)
					break
				}
				if w.matchService == nil {
					slog.Error("assignment worker: matchService is NIL")
					break
				}

				slog.Info("assignment worker: calling FindAvailableTherapistsForServiceWithTime",
					"booking_id", bid, "service_id", *b.ServiceID)

				therapists, err := w.matchService.FindAvailableTherapistsForServiceWithTime(
					ctx, b.ClientID, *b.ServiceID, b.GenderPref, b.PressurePref, start, b.DurationMinutes, lat, lng,
				)
				if err != nil {
					slog.Error("assignment worker: FindAvailable... returned error", "error", err)
					if err := w.queueRepo.IncrementAttempt(ctx, bid, it.Attempts, time.Now().Add(1*time.Minute)); err != nil {
						slog.Warn("assignment worker: failed to increment attempt on match error", "booking_id", bid, "error", err)
					}
					break
				}
				slog.Info("assignment worker: matching result", "count", len(therapists))

				if len(therapists) == 0 {
					if len(activeOffersByBooking[bid]) > 0 {
						transition("offering", nil)
					} else {
						attempts := it.Attempts + 1
						if attempts > w.maxAttempts {
							if w.notificationService != nil && b.ClientID != 0 {
								_, _ = w.notificationService.Create(ctx, &model.CreateNotificationRequest{
									UserID: b.ClientID, Type: "assignment_failed", Title: "Assignment failed", Message: "No therapist found.",
								})
							}
							_ = w.queueRepo.Remove(ctx, bid)
							// Notify admin/ops about exhausted assignment
							if w.opsNotifier != nil {
								_ = w.opsNotifier(ctx, "offer_exhausted", map[string]string{
									"booking_id": fmt.Sprint(bid),
									"attempts":   fmt.Sprint(attempts),
									"message":    "All therapist assignment attempts exhausted. Manual intervention required.",
								})
							}
						} else {
							backoff := time.Duration(1<<uint(attempts-1)) * w.baseBackoff
							_ = w.queueRepo.IncrementAttempt(ctx, bid, attempts, time.Now().Add(backoff))
						}
					}
					break
				}

				// Limit candidates
				if len(therapists) > 3 {
					therapists = therapists[:3]
				}

				offersMade := 0
				for _, t := range therapists {
					// Check already offered
					already := false
					for _, ao := range activeOffersByBooking[bid] {
						if ao.TherapistID == t.TherapistID {
							already = true
							break
						}
					}
					if already || t.TherapistID == 0 {
						continue
					}

					tx, err := w.db.Begin(ctx)
					if err != nil {
						continue
					}
					locked, err := w.therapistRepo.TryLockTherapistTx(ctx, tx, t.TherapistID)
					if err != nil || !locked {
						_ = tx.Rollback(ctx)
						continue
					}

					// Calculate earnings
					finalTotal := 0.0
					if b.FinalTotal != nil {
						finalTotal = *b.FinalTotal
					}
					earnings := CalculateCommission(0.0, finalTotal, b.DurationMinutes, b.DurationMinutes)
					if details.Service != nil && details.Service.TherapistCommission != nil {
						earnings = CalculateCommission(*details.Service.TherapistCommission, details.Service.BasePrice, details.Service.DurationMinutes, b.DurationMinutes)
					}

					offer := &model.BookingOffer{
						BookingID:         bid,
						TherapistID:       t.TherapistID,
						Status:            "pending",
						ExpiresAt:         time.Now().Add(5 * time.Minute),
						EstimatedEarnings: &earnings,
						Items: []model.BookingOfferItem{
							{BookingID: bid, EstimatedEarnings: earnings},
						},
					}

					if err := w.offerRepo.CreateTx(ctx, tx, offer); err == nil {
						if err := tx.Commit(ctx); err == nil {
							offersMade++
							w.notifyAndBroadcastOffer(ctx, t.TherapistID, offer, details)
						}
					} else {
						_ = tx.Rollback(ctx)
					}
				}

				if offersMade > 0 {
					transition("offering", nil)
				} else {
					_ = w.queueRepo.IncrementAttempt(ctx, bid, it.Attempts, time.Now().Add(1*time.Minute))
				}

			case "offering":
				if len(activeOffersByBooking[bid]) == 0 {
					transition("matching", nil)
				} else {
					_ = w.queueRepo.IncrementAttempt(ctx, bid, it.Attempts, time.Now().Add(5*time.Minute))
				}

			case "sequence_bundling":
				if b.GroupID == nil {
					transition("matching", nil)
					break
				}
				siblings := siblingsMap[*b.GroupID]
				if len(siblings) == 0 || siblings[0].BookingID != bid {
					_ = w.queueRepo.IncrementAttempt(ctx, bid, it.Attempts, time.Now().Add(30*time.Second))
					break
				}

				// Leader Logic: Bundling
				candidateCounts := make(map[int64]int)
				candidateProfiles := make(map[int64]model.TherapistProfile)

				for _, sib := range siblings {
					if sib.Status != "pending" {
						continue
					}
					start := time.Now()
					if sib.ScheduledStart != nil {
						start = *sib.ScheduledStart
					}
					if sib.ServiceID == nil {
						slog.Warn("assignment worker: sibling has no service_id", "booking_id", sib.BookingID)
						continue
					}
					cands, err := w.matchService.FindAvailableTherapistsForServiceWithTime(
						ctx, sib.ClientID, *sib.ServiceID, sib.GenderPref, sib.PressurePref, start, sib.DurationMinutes, nil, nil,
					)
					if err == nil {
						for _, c := range cands {
							candidateCounts[c.TherapistID]++
							candidateProfiles[c.TherapistID] = c
						}
					}
				}

				validTherapists := []model.TherapistProfile{}
				for id, count := range candidateCounts {
					if count == len(siblings) {
						validTherapists = append(validTherapists, candidateProfiles[id])
					}
				}

				if len(validTherapists) == 0 {
					_ = w.queueRepo.IncrementAttempt(ctx, bid, it.Attempts, time.Now().Add(5*time.Minute))
					break
				}

				// Limit and Offer
				if len(validTherapists) > 3 {
					validTherapists = validTherapists[:3]
				}

				offersMade := 0
				for _, t := range validTherapists {
					tx, err := w.db.Begin(ctx)
					if err != nil {
						continue
					}
					locked, err := w.therapistRepo.TryLockTherapistTx(ctx, tx, t.TherapistID)
					if err != nil || !locked {
						_ = tx.Rollback(ctx)
						continue
					}

					var totalEarnings float64
					offerItems := []model.BookingOfferItem{}
					for _, sib := range siblings {
						ft := 0.0
						if sib.FinalTotal != nil {
							ft = *sib.FinalTotal
						}
						// Note: for siblings we don't have details pre-fetched in detailMap (only leader is there)
						// So we use CalculateCommission with default or fetch if needed.
						// To keep it simple and batch-friendly, we'd need sib services.
						earn := CalculateCommission(0.0, ft, sib.DurationMinutes, sib.DurationMinutes)
						totalEarnings += earn
						offerItems = append(offerItems, model.BookingOfferItem{BookingID: sib.BookingID, EstimatedEarnings: earn})
					}

					offer := &model.BookingOffer{
						BookingID:         bid,
						TherapistID:       t.TherapistID,
						Status:            "pending",
						ExpiresAt:         time.Now().Add(5 * time.Minute),
						EstimatedEarnings: &totalEarnings,
						Items:             offerItems,
						IsBundle:          true,
					}

					if err := w.offerRepo.CreateTx(ctx, tx, offer); err == nil {
						if err := tx.Commit(ctx); err == nil {
							offersMade++
							// For bundle, pass first sibling details (leader) as representative
							// We could fetch others if needed, but UI likely just shows summary or first one
							// details for 'bid' (leader) should be in detailsMap
							sibDetails := detailsMap[bid]
							w.notifyAndBroadcastOffer(ctx, t.TherapistID, offer, sibDetails)
						}
					} else {
						_ = tx.Rollback(ctx)
					}
				}

				if offersMade > 0 {
					for _, sib := range siblings {
						_ = w.queueRepo.UpdateWorkflowState(ctx, sib.BookingID, "offering", nil)
					}
				} else {
					_ = w.queueRepo.IncrementAttempt(ctx, bid, it.Attempts, time.Now().Add(1*time.Minute))
				}

			default:
				transition("init", nil)
			}

			if !transitioned || it.WorkflowState == "offering" {
				break
			}
		}
	}
	return len(items)
}
func (w *AssignmentWorker) notifyAndBroadcastOffer(ctx context.Context, therapistID int64, offer *model.BookingOffer, details *repository.BookingDetailsResult) {
	expiresAt := offer.ExpiresAt
	bid := offer.BookingID

	payload := map[string]interface{}{
		"booking_id":          bid,
		"offer_id":            offer.OfferID,
		"target_therapist_id": therapistID,
		"expires_at":          expiresAt,
		"created_at":          offer.CreatedAt,
		"is_bundle":           offer.IsBundle,
	}
	if offer.EstimatedEarnings != nil {
		payload["estimated_earnings"] = *offer.EstimatedEarnings
		// Client expects "price" which is usually the total or earnings.
		// Contextually, "price" in the offer dialog usually refers to what the therapist earns or the booking value.
		// Given the dialog shows "price" next to money icon, let's map EstimatedEarnings to "price" for now,
		// or if we have the booking total, maybe that.
		// But for a therapist offer, they care about their earnings.
		// Let's check what the client does. valid price = offerData['price'] ?? 0;
		// If we send 0, it shows 0.
		payload["price"] = *offer.EstimatedEarnings
	}

	// Enrich with details if available
	if details != nil {
		if details.ClientName != "" {
			payload["client_name"] = details.ClientName
		} else {
			payload["client_name"] = "Client"
		}

		if details.Service != nil {
			payload["service_name"] = details.Service.Name
		} else {
			payload["service_name"] = "Service"
		}

		if details.Address != nil {
			// Format address
			addr := details.Address.Street
			if details.Address.Label != "" {
				addr = fmt.Sprintf("%s (%s)", details.Address.Label, addr)
			}
			payload["address"] = addr
		} else {
			payload["address"] = "Location provided in details"
		}
	} else {
		// Fallback values when details is nil
		payload["client_name"] = "Client"
		payload["service_name"] = "Service"
		payload["address"] = "Location provided in details"
	}

	if w.notificationService != nil {
		// Use enriched payload for notification data too
		notif, err := w.notificationService.Create(ctx, &model.CreateNotificationRequest{
			UserID: therapistID, Type: "booking_offer", Title: "New Offer", Message: "You have a new booking offer!", Data: payload,
		})
		if err != nil {
			slog.Error("assignment worker: failed to create notification", "therapist_id", therapistID, "error", err)
		} else {
			slog.Info("assignment worker: created notification", "notification_id", notif.NotificationID, "therapist_id", therapistID)
		}
	}

	slog.Info("🔔 Broadcasting offer to therapist", "therapist_id", therapistID, "booking_id", offer.BookingID, "payload", payload)
	_ = broadcaster.BroadcastToUser(therapistID, "offered_to_therapist", payload)
}
