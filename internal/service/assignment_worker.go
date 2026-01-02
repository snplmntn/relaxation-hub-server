package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/broadcaster"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

// AssignmentWorker picks unassigned bookings from a durable queue and attempts
// to match them to available therapists. It's designed to be resilient and
// idempotent so it can be run concurrently or restarted.
type AssignmentWorker struct {
    db                  db.DBTX
    queueRepo           repository.AssignmentQueueRepository
    bookingRepo         repository.BookingRepository
    paymentRepo         repository.PaymentRepository
    offerRepo           repository.BookingOfferRepository
    matchService        TherapistMatchingService
    notificationService *NotificationService
    // opsNotifier is an optional hook to surface critical failures to ops.
    opsNotifier         func(ctx context.Context, subject string, details map[string]string) error
    pollInterval        time.Duration
    batchSize           int
    maxAttempts         int
    baseBackoff         time.Duration
}
func NewAssignmentWorker(db db.DBTX, qr repository.AssignmentQueueRepository, br repository.BookingRepository, pr repository.PaymentRepository, or repository.BookingOfferRepository, ms TherapistMatchingService, ns *NotificationService, opsNotifier func(ctx context.Context, subject string, details map[string]string) error) *AssignmentWorker {
    return &AssignmentWorker{
        db:                  db,
        queueRepo:           qr,
        bookingRepo:         br,
        paymentRepo:         pr,
        offerRepo:           or,
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
                log.Printf("assignment worker: panic recovered: %v", r)
                if w.opsNotifier != nil {
                    _ = w.opsNotifier(ctx, "assignment_worker: panic", map[string]string{"panic": fmt.Sprint(r)})
                }
            }
        }()

        ticker := time.NewTicker(w.pollInterval)
        defer ticker.Stop()
        for {
            select {
            case <-ctx.Done():
                return
            case <-ticker.C:
                w.processOnce(ctx)
            }
        }
    }()
}

func (w *AssignmentWorker) processOnce(ctx context.Context) {
    items, err := w.queueRepo.DequeueBatch(ctx, w.batchSize)
    if err != nil {
        log.Printf("assignment worker: fail dequeue batch: %v", err)
        if w.opsNotifier != nil {
            _ = w.opsNotifier(ctx, "assignment_worker: dequeue_failed", map[string]string{"error": err.Error()})
        }
        return
    }
    if len(items) == 0 {
        return
    }

    for _, it := range items {
        bid := it.BookingID
        // Load booking (admin scope)
        b, err := w.bookingRepo.GetByBookingID(ctx, bid)
        if err != nil {
            log.Printf("assignment worker: failed to load booking %d: %v", bid, err)
            // remove from queue if missing
            _ = w.queueRepo.Remove(ctx, bid)
            continue
        }

        // If service is not present, skip for now
        if b.ServiceID == nil {
            continue
        }

        // Payment gating removed as per requirements.

        // Check for active offers
        activeOffers, err := w.offerRepo.GetActiveOffers(ctx, bid)
        if err != nil {
            log.Printf("assignment worker: failed to get active offers for booking %d: %v", bid, err)
            continue
        }

        shouldExpand := false
        if len(activeOffers) > 0 {
            // Check age of oldest active offer
            oldest := activeOffers[0].CreatedAt
            for _, o := range activeOffers {
                if o.CreatedAt.Before(oldest) {
                    oldest = o.CreatedAt
                }
            }
            
            if time.Since(oldest) < 5*time.Minute {
                // Wait until 5 minutes have passed
                nextCheck := oldest.Add(5 * time.Minute)
                _ = w.queueRepo.IncrementAttempt(ctx, bid, it.Attempts, nextCheck)
                continue
            }
            // >= 5 minutes passed, expand pool
            shouldExpand = true
        } else {
            // No active offers. Expire any old pending offers.
            expired, err := w.offerRepo.ExpireOffers(ctx, bid)
            if err != nil {
                 log.Printf("assignment worker: failed to expire offers for booking %d: %v", bid, err)
            } else {
                // Broadcast expiration
                for _, o := range expired {
                    _ = broadcaster.BroadcastToUser(o.TherapistID, "offer_expired", map[string]any{
                        "offer_id":   o.OfferID,
                        "booking_id": o.BookingID,
                    })
                }
            }
        }

        // Find available therapists for the service
        therapists, err := w.matchService.FindAvailableTherapistsForService(ctx, b.ClientID, *b.ServiceID, b.GenderPref, b.PressurePref)
        if err != nil {
            log.Printf("assignment worker: matching error for booking %d: %v", bid, err)
            if w.opsNotifier != nil {
                _ = w.opsNotifier(ctx, "assignment_worker: matching_error", map[string]string{"booking_id": fmt.Sprint(bid), "error": err.Error()})
            }
            continue
        }
        log.Printf("assignment worker: Found %d total available therapists for booking %d. Filtered from initial pool.", len(therapists), bid)

        // Identify struggling therapists (cancellations/no-shows OR low booking volume)
        tids := make([]int64, len(therapists))
        for i, t := range therapists {
            tids[i] = t.TherapistID
        }
        struggleMap, _ := w.bookingRepo.GetRecentTherapistStruggleFlags(ctx, tids, time.Now().Add(-24*time.Hour))
        if struggleMap == nil {
            struggleMap = make(map[int64]bool)
        }

        // Also identify low-volume therapists (those with significantly fewer bookings)
        countsSince := time.Now().Add(-24 * time.Hour)
        bookingCounts, _ := w.bookingRepo.GetTherapistBookingCounts(ctx, tids, countsSince)
        if bookingCounts == nil {
            bookingCounts = make(map[int64]int)
        }

        // Calculate average bookings
        totalBookings := 0
        for _, tid := range tids {
            totalBookings += bookingCounts[tid]
        }
        avgBookings := 0.0
        if len(tids) > 0 {
            avgBookings = float64(totalBookings) / float64(len(tids))
        }
        lowVolumeThreshold := avgBookings * 0.5

        // Mark low-volume therapists as struggling too
        for _, tid := range tids {
            if float64(bookingCounts[tid]) <= lowVolumeThreshold {
                struggleMap[tid] = true
            }
        }

        // Filter out therapists who have already been offered (rejected or expired or currently active)
        pastOffers, err := w.offerRepo.GetOffersByBookingID(ctx, bid)
        if err != nil {
             log.Printf("assignment worker: failed to get past offers for booking %d: %v", bid, err)
             continue
        }
        
        offeredMap := make(map[int64]bool)
        for _, o := range pastOffers {
            offeredMap[o.TherapistID] = true
        }

        candidates := make([]model.TherapistProfile, 0)
        for _, t := range therapists {
            if !offeredMap[t.TherapistID] {
                candidates = append(candidates, t)
            } else {
                log.Printf("assignment worker: skipping therapist %d (already offered)", t.TherapistID)
            }
        }

        if len(candidates) == 0 {
            if len(activeOffers) > 0 {
                 // Wait for current offers
                 minExpiresAt := activeOffers[0].ExpiresAt
                 for _, o := range activeOffers {
                     if o.ExpiresAt.Before(minExpiresAt) {
                         minExpiresAt = o.ExpiresAt
                     }
                 }
                 _ = w.queueRepo.IncrementAttempt(ctx, bid, it.Attempts, minExpiresAt)
                 continue
            }
            // no match found this round — schedule retry with backoff
            attempts := it.Attempts + 1
            backoff := time.Duration(1<<uint(attempts-1)) * w.baseBackoff
            if attempts > w.maxAttempts {
                // notify client/admin and remove from queue
                log.Printf("assignment worker: booking %d exhausted attempts", bid)
                if w.opsNotifier != nil {
                    _ = w.opsNotifier(ctx, "assignment_worker: exhausted_attempts", map[string]string{"booking_id": fmt.Sprint(bid)})
                }
                // notify client if possible
                if w.notificationService != nil && b.ClientID != 0 {
                    _, _ = w.notificationService.Create(ctx, &model.CreateNotificationRequest{
                        UserID:  b.ClientID,
                        Type:    "assignment_failed",
                        Title:   "Assignment failed",
                        Message: "We couldn't find an available therapist for your booking; our team will try to find an available therapist and manually assign it to you.",
                    })
                }
                _ = w.queueRepo.Remove(ctx, bid)
                continue
            }
            next := time.Now().Add(backoff)
            _ = w.queueRepo.IncrementAttempt(ctx, bid, attempts, next)
            continue
        }

        targetCandidates := make([]model.TherapistProfile, 0)
        
        if !shouldExpand && len(activeOffers) == 0 {
            log.Printf("assignment worker: Stage 1 (Struggling Therapists) for booking %d", bid)
            // Stage 1: Struggling only
            for _, t := range candidates {
                if struggleMap[t.TherapistID] {
                    targetCandidates = append(targetCandidates, t)
                    log.Printf("assignment worker: candidate %d selected (struggling)", t.TherapistID)
                } else {
                    log.Printf("assignment worker: candidate %d skipped (not struggling)", t.TherapistID)
                }
            }
            if len(targetCandidates) == 0 {
                log.Printf("assignment worker: no struggling candidates found, falling back to all candidates")
                // No struggling candidates available. Proceed to offer to wider group immediately.
                targetCandidates = candidates
            }
        } else {
            log.Printf("assignment worker: Stage 2 (Expansion) for booking %d", bid)
            // Stage 2: Expanding -> Offer to remaining candidates
            targetCandidates = candidates
        }

        // Limit for wider group (Stage 2 or fallback)
        // If we are in Stage 1 (struggling), we take ALL of them (no limit).
        // If we are in Stage 2, we limit to 5.
        isStage1 := (!shouldExpand && len(activeOffers) == 0 && len(targetCandidates) > 0 && struggleMap[targetCandidates[0].TherapistID])
        
        if !isStage1 {
            limit := 5
            if len(targetCandidates) > limit {
                log.Printf("assignment worker: limiting candidates from %d to %d", len(targetCandidates), limit)
                targetCandidates = targetCandidates[:limit]
            }
        }

        // Create offers
        // Expiration: 30 mins to give time, but we check back in 5 mins to expand.
        expiresAt := time.Now().Add(30 * time.Minute)
        for _, t := range targetCandidates {
            offer := &model.BookingOffer{
                BookingID:   bid,
                TherapistID: t.TherapistID,
                Status:      model.BookingOfferStatusPending,
                CreatedAt:   time.Now(),
                ExpiresAt:   expiresAt,
            }
            if err := w.offerRepo.Create(ctx, offer); err != nil {
                log.Printf("assignment worker: failed to create offer for booking %d therapist %d: %v", bid, t.TherapistID, err)
                continue
            }
            
            // Log the offer as requested
            log.Printf("assignment worker: OFFER MADE: BookingID=%d, TherapistID=%d, OfferID=%d", bid, t.TherapistID, offer.OfferID)

            // Notify therapist
            if w.notificationService != nil {
                _, _ = w.notificationService.Create(ctx, &model.CreateNotificationRequest{
                    UserID:  t.TherapistID,
                    Type:    "booking_offer",
                    Title:   "New Booking Offer",
                    Message: "You have a new booking offer. Please accept or decline.",
                    Data:    map[string]any{"booking_id": bid, "offer_id": offer.OfferID, "expires_at": expiresAt},
                })
            }

            // Broadcast real-time event to therapist
            payload := map[string]interface{}{
                "booking_id":          bid,
                "offer_id":            offer.OfferID,
                "target_therapist_id": t.TherapistID,
                "expires_at":          offer.ExpiresAt,
                "created_at":          offer.CreatedAt,
            }
            _ = broadcaster.BroadcastToUser(t.TherapistID, "offered_to_therapist", payload)
        }

        // Update queue to check back in 5 minutes (to expand if needed)
        _ = w.queueRepo.IncrementAttempt(ctx, bid, it.Attempts, time.Now().Add(5*time.Minute))
    }
}
