Booking events & booking response spec

Summary

- Added `booking_events` table to record timeline events (created, assigned, payment_succeeded, therapist_arrived, confirm_start, no_show, cancelled, etc.)
- Bookings now include `assigned_at`, `therapist_arrived_at`, `no_show_at`, `cancelled_by`, `cancelled_at`, `cancellation_reason`.
- Handlers should surface these fields in booking responses and APIs should return `server_time` and `grace_period` where relevant.

New: Offer-to-therapists-first

- The platform SHOULD attempt to offer an eligible booking to qualified on-call therapists before persisting a final `assigned` state to any therapist. This introduces a short "offer" lifecycle prior to assignment and is reflected in `booking_events`.
- New events: `offered_to_therapist`, `offer_accepted`, `offer_declined`, `offer_expired`.
- Offer behavior summary:
  - When a booking is created and assignment is appropriate (see `AssignTherapist` gating), the service will create one or more short-lived "offers" for therapists rather than immediately writing an `assigned` event.
  - Each offer is written as a `booking_events` row `offered_to_therapist` with `actor_id = system` and `metadata` identifying the therapist(s) targeted and `expires_at`.
  - The targeted therapist may respond `accept` or `decline` via the appropriate API/action. Accept -> emit `offer_accepted` (actor = therapist) and transition to `assigned` (set `assigned_at`). Decline -> emit `offer_declined` (actor = therapist) and try next candidate or expire.
  - When an offer reaches `expires_at` without response, emit `offer_expired` and continue the offering process according to policy (try next therapist, open market, or fallback to admin assignment).
  - If a therapist accepts, set `assigned_at`, emit `assigned` with `actor_id` = therapist, and cancel other outstanding offers for the booking (emit `offer_expired`/`offer_cancelled` for those offers as needed).
  - Admins creating bookings may opt-out of the offer process and request a direct assignment; admin direct assignments are still subject to assignment gating and will emit `admin_created_booking` + `assigned` with `actor_id` = admin when accepted.

Service changes required

- CreateBooking
  - Persist booking with chosen `payment_method` and create a payment intent when prepay is required.
  - Emit `booking_events` row: `created`.
  - When an admin creates a booking for a client via the admin API (`POST /api/v1/admin/bookings`), the service should also emit an `admin_created_booking` event with `actor_id` set to the admin's user id. This provides an auditable record for manual administrative intervention.
  - Admin-created bookings: an admin may optionally assign a therapist at creation time. Assignment at creation is subject to the normal assignment gating (see "AssignTherapist" below) — the service should only accept and persist an `assigned` state if the booking meets the assignment rules (booking is `pending` OR `payment_method` is `cash`). When an admin assigns during creation, set `assigned_at` and emit both `admin_created_booking` and `assigned` `booking_events` with `actor_id` set to the admin's user id.
  - Offer-to-therapists behavior during creation: when the platform policy is to offer to therapists first (default for many markets), `CreateBooking` should enqueue and create initial `offered_to_therapist` `booking_events` rather than immediately persisting `assigned`. Admins may opt to bypass offers and request direct assignment.
- Payment flow
  - On payment intent success (sync or webhook) emit `payment_succeeded` event.
  - On payment failure emit `payment_failed` event and surface `payment_status` via `payments` table (no separate `awaiting_payment` booking status used).
  - Note: the distinct booking status `awaiting_payment` has been removed. All payment state is recorded in the `payments` table and reflected via `payment_status` and events. Handlers should rely on `payments` records and emitted `booking_events` (e.g., `payment_succeeded`, `payment_failed`) rather than an `awaiting_payment` booking state.
- AssignTherapist
  - Only assign when booking is `pending` OR payment method is `cash` (business rule enforceable in service layer).
  - Offer-first assignment: where configured, the service SHOULD create therapist offers (see "Offer-to-therapists-first" above) and only set `assigned_at` + write `assigned` when a therapist accepts an offer.
  - Direct assignment: allowed for admins or when policy dictates; direct assignment must still enforce gating (`pending` OR `cash`). On direct assignment set `assigned_at` and write `assigned` with `actor_id` = admin/therapist.
  - Admins assigning at creation: when an admin includes an assignment as part of `POST /api/v1/admin/bookings`, the same gating rules apply; the API should validate assignment eligibility and either persist `assigned_at` + `assigned` event or reject the assignment and create the booking without an assignment.
- UpdateStatus
  - When transitioning to `in_progress`, set `therapist_arrived_at` if not already set and write event `therapist_arrived` or `session_started` depending on confirmation handshake.
  - When marking `cancelled`, set `cancelled_by`, `cancelled_at`, `cancellation_reason` and write `cancelled` event.
- No-show / reschedule
  - Support explicit `no_show` status and `no_show_at` timestamp; emit `no_show` event.

Offer events & data modeling

- Each offer SHOULD be referenceable in `booking_events.metadata` by an `offer_id` so that accept/decline/expiry map cleanly to a single offer lifecycle. Example metadata: `{ "offer_id": "uuid", "target_therapist_id": 123, "expires_at": "2025-12-27T12:34:56Z" }`.
- For auditing, preserve all offers as events; do not sweep historical offer events when assignment completes.

Handler/API changes

- Booking responses (GET/list/create/update) MUST include:
  - `assigned_at`, `therapist_arrived_at`, `no_show_at`, `cancelled_by`, `cancelled_at`, `cancellation_reason`.
  - `server_time` (optional top-level in response or separate endpoint) and `grace_period_minutes` (from configuration) to drive client countdown timers.
  - Enriched `client` object: `{ "client_id", "name", "phone", "photo", "gender" }`
  - Enriched `therapist` object (when assigned): `{ "therapist_id", "name", "phone", "photo", "gender", "rating" }`
- When offering-to-therapists-first is used, handlers SHOULD also surface the current `offers` state in booking responses (or in a separate `timeline`/`offers` endpoint). Minimal offer data: `offer_id`, `target_therapist_id`, `status` (pending/accepted/declined/expired), `expires_at`.
- Expose recent `booking_events` or a short `timeline` in `GET /bookings/{id}` for detailed UX (optional, recommended).

Admin Intervention Endpoints

- `GET /admin/bookings/pending` - List all pending bookings without assigned therapist.
- `GET /admin/bookings/{id}/offers` - List all offers for a booking (for admin to see which therapists declined/expired).
- `GET /admin/bookings/{id}/candidates` - List available therapist candidates for manual assignment.

Notes on countdown & anti-cheat

- Server provides authoritative `scheduled_start` and `server_time` so clients compute identical countdown timers.
- Use a handshake: therapist marks `therapist_arrived` (writes `therapist_arrived_at`), a party confirms `confirm_start`; server sets `actual_start` when both confirmations present or when grace_period expires (policy).
- All confirmations and state changes are recorded in `booking_events` for audit and dispute resolution.

Additional start behavior:

- The `StartSession` flow allows a `client`, `therapist`, or an `admin` to initiate the booking start via `POST /bookings/{id}/start` when appropriate. The server enforces that the therapist has arrived (or `therapist_arrived_at` is set) before transitioning to `in_progress`, and it records the confirmation event (`confirm_start`) in `booking_events` for audit.

Migration

- Migration SQL files were removed from this repository. Ensure your database provisioning process includes the schema needed for `booking_events` and related booking columns.

Testing

- Add unit tests for:
  - Assignment gating (prepay vs cash)
  - Timeline event writes
  - State transition validation and race conditions (assignment worker + manual assign)
  - Offer lifecycle tests: offer creation, accept, decline, expiry, concurrent accepts (ensure single-assignment), and offer audit logging.

Roadmap

1. Ensure dev DB schema includes booking events/timeline columns.
2. Wire service flows (payment webhook -> event + transition).
3. Update handlers (done) and add `timeline` endpoint.
4. Add tests and CI checks.
