# Booking Announcement Variations Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add unlimited optional message variations to booking-announcement campaigns, keep one stable selection per booking/group, and include it in Day View copies and booking-completed emails.

**Architecture:** PostgreSQL stores campaigns, ordered variations, the optional winner, and consistency-only assignment snapshots. A single Go service owns validation and resolution; both the admin endpoint and completed-email service call it. React Query exposes campaign CRUD/winner/resolution operations, while Day View resolves on copy instead of filtering campaigns client-side.

**Tech Stack:** Go 1.24, chi, pgx/PostgreSQL, React 19, TypeScript, TanStack Query/Router, Vitest, Tailwind CSS.

**Spec:** `docs/superpowers/specs/2026-09-19-booking-announcement-variations-design.md`

## Global Constraints

- One or unlimited variations; no variation-mode toggle.
- A selected winner applies only to future unassigned bookings.
- No conversion counts, impressions, analytics, customer audit, or automatic winner.
- Only Day View copied details and booking-completed email receive announcements.
- No database or external-service integration tests.
- Preserve unrelated dirty work and commit only feature files/hunks.

---

### Task 1: Server campaign model and validation

**Files:**
- Create: `internal/model/booking_announcement.go`
- Create: `internal/service/booking_announcement_service.go`
- Create: `internal/service/booking_announcement_service_test.go`
- Create: `internal/db/migrations/042_create_booking_announcements.sql`
- Create: `internal/db/migrations/044_add_booking_announcement_variations.sql`

**Interfaces:**
- Produces: `BookingAnnouncement`, `BookingAnnouncementVariation`, `ResolvedBookingAnnouncement`, `BookingAnnouncementRequest`, and `BookingAnnouncementService`.
- Repository contract: `List`, `Create`, `Update`, `Delete`, `ChooseWinner`, and `ResolveForBooking`.

- [ ] **Step 1: Write failing validation and resolution service tests**

Cover trimmed unlimited variations, empty variation rejection, invalid date ranges, winner ownership, stable repeated resolution, group subject reuse, and winner use for new assignments with literal expected messages.

- [ ] **Step 2: Run the focused test and verify RED**

Run: `go test ./internal/service -run 'TestBookingAnnouncement' -count=1`

Expected: compilation/test failure because the variation request and resolver contracts do not exist.

- [ ] **Step 3: Add the minimum model, service, and migrations**

Use these request/response shapes:

```go
type BookingAnnouncementVariationRequest struct {
    VariationID *int64 `json:"variation_id,omitempty"`
    Message string `json:"message"`
}
type BookingAnnouncementRequest struct {
    Variations []BookingAnnouncementVariationRequest `json:"variations"`
    StartDate string `json:"start_date"`
    EndDate string `json:"end_date"`
}
type WinnerRequest struct { VariationID int64 `json:"variation_id"` }
```

Migration `042` creates the legacy campaign table. Migration `044` creates ordered variations, backfills `message`, adds `winner_variation_id`, and creates assignment snapshots keyed uniquely by campaign plus booking or group.

- [ ] **Step 4: Run the focused test and verify GREEN**

Run: `go test ./internal/service -run 'TestBookingAnnouncement' -count=1`

Expected: PASS.

### Task 2: Server persistence, routes, and completed email

**Files:**
- Create: `internal/repository/booking_announcement_repo.go`
- Create: `internal/handler/booking_announcement.go`
- Modify: `internal/app/dependencies.go`
- Modify: `internal/app/routes.go`
- Modify: `internal/service/booking_email_service.go`
- Modify: `internal/service/booking_email_service_test.go`

**Interfaces:**
- Consumes: campaign service and repository contract from Task 1.
- Produces: `GET/POST/PUT/DELETE /api/v1/booking-announcements`, `PUT /{id}/winner`, and `GET /resolve/{bookingID}`.

- [ ] **Step 1: Write failing completed-email tests**

Add a fake announcement resolver and assert literal announcement text appears in both completed plain-text and HTML output, while advanced-confirmation output excludes it.

- [ ] **Step 2: Run the focused email test and verify RED**

Run: `go test ./internal/service -run 'Test.*BookingEmail.*Announcement' -count=1`

Expected: FAIL because the email service does not accept or render resolved announcements.

- [ ] **Step 3: Implement transactional repository and HTTP handlers**

Resolve the authoritative booking business date/group inside a transaction using `business_day(scheduled_start)`, reuse existing snapshots, choose `(subjectID + announcementID) % len(variations)` when no winner exists, insert with conflict protection, and return campaign-ordered messages. Campaign writes upsert ordered variations and preserve assignment snapshots.

- [ ] **Step 4: Wire the resolver into completed email only**

Extend `BookingEmailData` with `Announcements []string`. Resolve in `send` only when `template == "booking_completed_success"`; log resolution errors and continue the existing best-effort delivery behavior. Render escaped HTML and plain text after booking details.

- [ ] **Step 5: Run server unit tests and verify GREEN**

Run: `go test ./internal/...`

Expected: PASS with no database/external integration tests.

### Task 3: Web contracts and copy formatting

**Files:**
- Modify: `src/types/api.ts`
- Modify: `src/hooks/use-api.ts`
- Modify: `src/lib/booking-announcements.ts`
- Modify: `src/lib/booking-announcements.test.ts`

**Interfaces:**
- Produces: campaign/variation/resolved TypeScript types, CRUD/winner hooks, `useResolveBookingAnnouncements`, and `appendBookingAnnouncements(details, resolved)`.

- [ ] **Step 1: Replace helper tests with failing variation/resolution cases**

Assert malformed campaigns are rejected, unlimited ordered versions parse, and all resolved campaign messages append as separate `Announcement:` paragraphs.

- [ ] **Step 2: Run focused Vitest and verify RED**

Run: `npm test -- --run src/lib/booking-announcements.test.ts`

Expected: FAIL because the new API shapes are unsupported.

- [ ] **Step 3: Implement minimum types, parsers, hooks, and formatter**

Campaign payloads contain `variations`, date fields, and optional winner. Resolution calls `GET /api/v1/booking-announcements/resolve/{bookingID}` and validates the returned messages before copy.

- [ ] **Step 4: Run focused Vitest and verify GREEN**

Run: `npm test -- --run src/lib/booking-announcements.test.ts`

Expected: PASS.

### Task 4: Administration UI and Day View integration

**Files:**
- Create: `src/components/booking-announcements/BookingAnnouncementsPage.tsx`
- Create: `src/routes/booking-announcements.tsx`
- Modify: `src/components/day-view/DayView.tsx`
- Modify: `src/components/layout/Sidebar.tsx`
- Modify: `src/lib/permissions.ts`
- Modify: `src/routeTree.gen.ts`

**Interfaces:**
- Consumes: hooks and types from Task 3.
- Produces: Administration page with unlimited version fields and winner actions; async Day View copy with failure toast.

- [ ] **Step 1: Build the campaign editor and cards**

Start with one required textarea; add/remove fields; submit trimmed versions; label by order; show the selected winner; allow winner selection only for multi-version campaigns; retain date inputs and campaign deletion.

- [ ] **Step 2: Resolve before Day View clipboard writes**

Pass one `resolveBookingAnnouncements(bookingID)` callback through existing Day View context. Make copy async, append resolved messages, and show a danger toast without writing to clipboard when resolution fails.

- [ ] **Step 3: Format and run web verification**

Run: `npx prettier --write` on changed feature files, then `npm test -- --run`, `npm run lint`, and `npm run build`.

Expected: all commands exit 0; pre-existing Vite CSS optimizer warnings may remain informational.

### Task 5: Review, stage, and commit default branches

**Files:**
- Review all feature files in both repositories.

- [ ] **Step 1: Re-read the spec and inspect diffs**

Confirm unlimited variations, manual winner only, stable snapshots, group sharing, Day View copy, and completed email are present; confirm analytics and other emails are absent.

- [ ] **Step 2: Run fresh final verification**

Server: `go test ./internal/...` and formatting/static checks available in the repository.

Web: `npm test -- --run`, `npm run lint`, `npm run build`, and scoped Prettier check.

- [ ] **Step 3: Stage only feature paths/hunks and commit**

Server default `main`: `feat(announcements): add booking message variations`.

Web default `master`: `feat(announcements): add booking campaign variations`.

Do not stage `.env.production`, `output/`, transportation-fee, hotel-booking, or other unrelated changes.
