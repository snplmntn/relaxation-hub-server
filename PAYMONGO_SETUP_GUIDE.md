# PayMongo Online Payment — Setup Guide

How to enable online payment for the Hiraya / Kalinga customer web booking flow.

> **Status:** the integration is planned, not yet built. This guide covers the
> PayMongo-side setup and configuration, which can be done in parallel with (or
> ahead of) the server work.

---

## What this enables

The booking page's payment dropdown is replaced by a card picker with two
groups. **Pay online** sits on top; the routes where a person settles up with
you directly sit below.

| Group | Route | Booking created | Vouchers | VIP 10% | Confirmed by |
|---|---|---|---|---|---|
| **Pay online** (new) | GCash, Maya, Card, QR Ph | only after payment clears | no | **yes** | PayMongo, automatically |
| **Pay us directly** | Cash | immediately | yes | yes | therapist collects |
| **Pay us directly** | GCash transfer, BDO transfer | immediately | yes | yes | staff verify the receipt |

Only **vouchers** are blocked online. The VIP 10% applies on every route, so a
VIP customer pays the same total whichever they pick — there is no penalty for
choosing the convenient option.

Cancelling or abandoning the PayMongo page charges nothing and creates no
booking. The customer returns to the booking form with their selections intact.

The manual GCash/BDO flow (QR code, transfer, upload receipt, staff verifies) is
unchanged in every respect — same `payment_method` values, same receipt panel,
same verification. It simply becomes reachable from the web booking page instead
of only from staff and mobile bookings.

---

## Payment channels

Four channels ship. The customer picks one **on our page**, and that single
value is what we pass to PayMongo as `payment_method_types` — so their hosted
page opens straight into that channel instead of presenting the menu a second
time.

| Channel | API value | Notes |
|---|---|---|
| GCash | `gcash` | Highest volume in PH. |
| Maya | `paymaya` | Note the API value is `paymaya`, not `maya`. |
| Card | `card` | Visa/Mastercard. Highest fee tier. |
| QR Ph | `qrph` | The BSP national QR standard. |

PayMongo also supports `grab_pay`, `shopee_pay`, `billease`, `brankas`, and
`dob` (online banking direct debit) — none of which are activated on the
account. Adding one later is a one-line change plus activation; until it is
activated, naming it would make session creation fail.

### Why QR Ph is worth having alongside the wallet buttons

It looks partly redundant — QR Ph settles through InstaPay to GCash and Maya,
which have dedicated buttons already. The value is the **40+ participating
banks**: a customer paying from the BPI, BDO, UnionBank or Landbank app has no
other route on this list, and it also reaches GrabPay and ShopeePay, which are
not activated as channels of their own. It is the single broadest-reach channel
per unit of setup effort.

Two operational facts worth knowing:

- A dynamic QR Ph code **expires 30 minutes** after it is generated if unscanned.
  Because the booking is only created on payment confirmation, an expired QR
  simply means no booking and no charge — nothing to clean up.
- Minimum transaction is ₱1.00. There is no PayMongo-side maximum; the ceiling
  is whatever outward limit the customer's own bank sets.

### Fees

All PayMongo fees are **absorbed**, not passed to the customer. There is no
surcharge line on the booking summary. This is the reason **vouchers** are
switched off for online payment: the margin already gives up the processing fee,
so it does not also give up a stacked promotion. The VIP 10% is deliberately
exempt from that logic — it is a standing customer benefit, not a campaign, and
withdrawing it would punish your best customers for using the cheapest-to-handle
route.

Card is the most expensive channel and QR Ph among the cheapest; check current
rates at <https://www.paymongo.com/pricing>, they change.

---

## Setup

### Step 1 — Activate the channels

In the PayMongo Dashboard, activate each of: **GCash, Maya, Card, QR Ph**.

Activation is per-channel and not instant — several require business documents
and PayMongo review. Card and QR Ph in particular tend to take longest. Start
this first; it is the long pole in the whole project.

> **This gates everything else.** The integration pins an explicit channel list,
> so a channel that is not activated on the account makes checkout session
> creation **fail loudly** rather than silently disappear from the page. That is
> deliberate — a silent drop is how you ship a payment page missing the method
> half your customers wanted. It does mean the pinned list must match what is
> actually live before deploying.

If a channel's approval is still pending at deploy time, ship without it and add
it after. Do not deploy a list containing an inactive channel.

### Step 2 — Get the API keys

Dashboard → **Developers → API Keys**. Two pairs exist:

- `sk_test_...` / `pk_test_...` — test mode, no real money
- `sk_live_...` / `pk_live_...` — live mode

Only the **secret key** (`sk_`) is needed; the integration is server-side and
never renders card fields, so the public key is unused.

Treat `sk_live_` like a database password. It goes in the deployment
environment, never in the repo, never in the web app.

### Step 3 — Register the webhook endpoint

Dashboard → **Developers → Webhooks → Add endpoint**.

**URL:** `https://<your-api-host>/api/v1/webhooks/paymongo`

**Events:** subscribe to the payment-completed and payment-failed events. The
exact event names to select are `payment.paid` and `payment.failed`; if the
dashboard also offers `checkout_session.payment.paid`, subscribe to that too —
the handler accepts either and resolves the draft booking from the checkout
session reference, so subscribing to both is safe and not double-counted
(delivery is idempotent on the event id).

Saving the endpoint produces a **webhook signing secret**. This is a *different*
value from the API secret key above. Copy it — depending on the dashboard it may
only be shown once.

Register the endpoint twice: once in test mode pointing at staging, once in live
mode pointing at production. They have separate signing secrets.

### Step 4 — Set environment variables

Add to the server environment (see `ENVIRONMENT_SETUP_GUIDE.md` for how this
repo handles env):

```bash
# PayMongo secret API key — sk_test_... in staging, sk_live_... in production
PAYMONGO_SECRET_KEY=sk_test_xxxxxxxxxxxxxxxxxxxxx

# Webhook signing secret from Step 3 — NOT the same as the key above
PAYMONGO_WEBHOOK_SECRET=whsk_xxxxxxxxxxxxxxxxxxxxx

# Where PayMongo sends the customer after paying / after cancelling
PAYMONGO_SUCCESS_URL=https://kalingaspa.com/book?checkout={reference}
PAYMONGO_CANCEL_URL=https://kalingaspa.com/book?checkout={reference}&cancelled=1
```

Both redirect URLs must be on an origin already in the server's CORS allowlist
(`internal/app/routes.go`). `kalingaspa.com`, `www.kalingaspa.com` and
`bookhiraya.com` are already there.

If `PAYMONGO_SECRET_KEY` is unset, the "Pay online" option is not offered and
the booking page behaves exactly as it does today. That is the intended
fallback — an unconfigured environment degrades to cash-only rather than
erroring.

### Step 5 — Run the migration

```bash
make migrate   # or your usual migration command
```

Migration `034` adds `online` to the allowed payment methods on `bookings` and
`booking_groups`, and creates the `booking_checkouts` table that holds the
un-submitted booking draft between "customer pressed Pay" and "PayMongo
confirmed". Additive only; existing bookings are untouched.

### Step 6 — Test before going live

With test keys, run one booking end to end per channel:

1. Pick a card under **Pay online** → confirm the voucher field disappears,
   and that on a VIP account the 10% is still shown and still in the total.
2. Press Pay → you land on PayMongo's page → confirm it opens **straight into
   the channel you picked**, with no channel menu. Repeat for all four.
3. Pay with a [test card](https://docs.paymongo.com/docs/testing) → you return
   to the booking page, it shows "confirming your payment", then resolves to the
   normal booking-submitted screen with a reference code.
4. Check the booking exists, `payment_method` is `online`, and the payment row
   is `paid` with gateway `paymongo`.
5. Repeat, but **cancel** on the PayMongo page → confirm no booking was created
   and nothing was charged.
6. Repeat, but **close the tab** after paying → confirm the booking still gets
   created. This is the important one: it proves the webhook, not the browser
   redirect, is what creates the booking.

Dashboard → **Developers → Webhooks → \<your endpoint\>** shows every delivery
attempt and its response. A `401` there means the signing secret is wrong; a
`500` is a bug in the handler.

### Step 7 — Go live

- Swap `PAYMONGO_SECRET_KEY` to `sk_live_...`
- Swap `PAYMONGO_WEBHOOK_SECRET` to the **live** endpoint's signing secret
- Point `PAYMONGO_SUCCESS_URL` / `PAYMONGO_CANCEL_URL` at production
- Confirm every pinned channel shows **Activated** in the dashboard
- Run one small real transaction (₱1 service or a test booking you refund)

---

## How it works, in one page

```
Customer picks "Pay online", presses Pay
        │
        ▼
POST /api/v1/bookings/checkout          ← validates everything a normal booking
        │                                  validates: address, services, schedule.
        │                                  Creates NO booking.
        │                                  Stores the draft in booking_checkouts.
        ▼
POST https://api.paymongo.com/v1/checkout_sessions
        │  Basic auth: sk_ as username, empty password
        │  amount in centavos (₱4,500.00 → 450000), PHP only
        │  payment_method_types: [<the one channel the customer picked>]
        ▼
returns checkout_url  →  customer redirected to PayMongo
        │
        ▼
Customer pays
        │
        ├──────────────► browser redirected to PAYMONGO_SUCCESS_URL
        │                (cosmetic — shows "confirming payment", polls status)
        │
        └──────────────► POST /api/v1/webhooks/paymongo
                         (authoritative — this is what creates the booking)
                                │
                                ▼
                         verify signature, then replay the stored draft through
                         the normal booking-creation path, so reference codes,
                         therapist assignment, emails and live updates all fire
                         exactly as they do for a cash booking.
```

The browser redirect is **not** trusted to create anything. If the customer
closes the tab the moment payment completes, the webhook still lands and the
booking is still created. This is the standard hosted-checkout pattern and the
reason "pay first" is safe here.

### Webhook signature verification

Every incoming webhook is verified before it is processed. PayMongo sends:

```
Paymongo-Signature: t=<unix timestamp>,te=<test signature>,li=<live signature>
```

Verification is HMAC-SHA256 over the string `<t>.<raw request body>` keyed with
the **webhook signing secret**, compared against `li` in live mode or `te` in
test mode. The raw body must be hashed byte-for-byte as received — re-serialising
the parsed JSON produces a different string and the check will fail. The
timestamp is also checked to reject replayed deliveries.

An unverified request is rejected with `401` and never touches the booking
tables. Without this, anyone who learns the endpoint URL can create free
bookings by posting a fake "paid" event.

---

## Operational notes

**Refunds are manual.** Cancelling an online-paid booking does not move money —
staff refund from the PayMongo Dashboard (Payments → find the payment → Refund).
This is a deliberate, known shortcut. It is fine at current volume and will stop
being fine once online bookings are more than a trickle; the upgrade path is a
refund endpoint wired to the existing cancellation flow.

**Reporting.** Online payments land in the "other sales" bucket in booking
exports and daily sales reports, and are excluded from cash remittance — correct,
since no therapist ever handles that money. If you want a dedicated "Online"
column in the reports, say so; it is a small change to the bucket mapping in
`report_export_service.go`.

**Reconciliation.** The PayMongo checkout session id is stored on the payment
record as its transaction id, so any payout line in the PayMongo dashboard can
be traced to a booking. For a group booking, every child booking gets its own
payment row sharing the one session id — per-booking accounting stays correct
while the customer only paid once.

**Vouchers are off by design; VIP is not.** A voucher code is rejected for
online payment, enforced on the server for every path that can attach one —
customer web, and staff editing a booking in the admin. The web page hiding the
voucher field is cosmetic; the server is the actual gate. The VIP 10% is applied
to online bookings exactly as it is to cash ones, and is baked into the amount
sent to PayMongo — so the customer is charged the discounted total, not billed
full and credited later.

---

## Troubleshooting

| Symptom | Cause |
|---|---|
| Checkout session creation fails with a payment method error | A pinned channel is not activated on the account. Check Step 1. |
| Webhook deliveries show `401` in the dashboard | Wrong `PAYMONGO_WEBHOOK_SECRET`, or test secret used against live mode. They are per-endpoint and per-mode. |
| Customer paid but no booking appeared | Check the webhook delivery log first. No delivery attempt = endpoint not registered for that event. Failed attempt = server-side bug; PayMongo retries. |
| The "Pay online" group is missing from the picker | `PAYMONGO_SECRET_KEY` is unset in that environment. |
| Amounts off by 100× | Amounts are integer centavos. ₱4,500.00 is `450000`. |

---

## Reference

- [Create a Checkout Session](https://docs.paymongo.com/reference/create_checkout_sessions) — `POST https://api.paymongo.com/v1/checkout_sessions`
- [Webhook setup and management](https://docs.paymongo.com/docs/developer-tools-webhook-setup-management)
- [QR Ph overview](https://docs.paymongo.com/docs/payment-acceptance-qr-ph)
- [Pricing](https://www.paymongo.com/pricing)

A `v2` checkout sessions endpoint also exists, whose main addition is
pass-on fees — charging the processing fee to the customer. Since fees are
absorbed here, `v1` is what the integration uses. If that decision is ever
reversed, `v2` is where to look.
