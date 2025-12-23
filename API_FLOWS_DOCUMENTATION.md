# Relaxation Hub API - Complete User Flow Documentation

**Base URL:** `http://localhost:8080/api/v1`  
**Production URL:** `https://yourdomain.com/api/v1`

This documentation demonstrates real-world API flows from user registration through booking completion, therapist operations, and admin management.

---

## Table of Contents

1. [Authentication Flows](#authentication-flows)
2. [Client User Journey](#client-user-journey)
3. [Therapist Journey](#therapist-journey)
4. [Admin Operations](#admin-operations)
5. [Real-World Scenarios](#real-world-scenarios)
6. [Real-Time WebSocket Communication](#real-time-websocket-communication)

---

## Authentication Flows

### Flow 1: New User Registration (Email/Password)

**Scenario:** Maria wants to book a massage and creates an account.

```bash
POST /api/v1/register
Content-Type: application/json

{
  "provider": "email",
  "provider_key": "maria.santos@gmail.com",
  "password": "SecurePassword123!",
  "role": "client",
  "referral_code": "REF123" (optional)
}
```

**Response: 201 Created**

For client signups the API returns a JWT token and the created user id so the client can authenticate immediately:

```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user_id": 123,
  "expires_at": "2024-12-10T15:30:00Z"
}
```

For non-client roles (e.g. `therapist`, `admin`) the endpoint will return a success message instead:

```json
{
  "message": "User registered successfully"
}
```

### Flow 2: User Login

**Scenario:** Maria logs in to her account.

```bash
POST /api/v1/login
Content-Type: application/json

{
  "provider": "email",
  "provider_key": "maria.santos@gmail.com",
  "password": "SecurePassword123!"
}
```

**Response: 200 OK**

```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoiMTIzZTQ1NjctZTg5Yi0xMmQzLWE0NTYtNDI2NjE0MTc0MDAwIiwicm9sZSI6ImNsaWVudCIsImV4cCI6MTcwMjI1MTYwMCwiaWF0IjoxNzAyMTY1MjAwfQ.signature",
  "expires_at": "2024-12-10T15:30:00Z"
}
```

**Save the token for subsequent requests:**

```
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

### Flow 3: OAuth Login (Google)

**Scenario:** John prefers to sign in with Google.

#### Step 1: Initiate OAuth

```bash
POST /api/v1/oauth/google
```

**Response: 302 Redirect**

```
Location: https://accounts.google.com/o/oauth2/auth?client_id=...&redirect_uri=...&response_type=code&scope=profile+email
```

#### Step 2: User Authorizes on Google → Google Redirects Back

```bash
GET /api/v1/oauth/callback?code=4/0AX4XfWh...&state=...
```

**Response: 200 OK**

```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "user_id": "789e1234-e89b-12d3-a456-426614174000",
    "email": "john.doe@gmail.com",
    "first_name": "John",
    "last_name": "Doe",
    "role": "client"
  },
  "expires_at": "2024-12-10T15:30:00Z"
}
```

### Flow 4: OAuth Login (Apple)

**Scenario:** Sarah uses Sign in with Apple.

#### Step 1: Initiate OAuth

```bash
POST /api/v1/oauth/apple
```

**Response: 302 Redirect**

```
Location: https://appleid.apple.com/auth/authorize?client_id=...&redirect_uri=...&response_type=code&scope=name+email
```

#### Step 2: Apple Redirects Back

```bash
GET /api/v1/oauth/callback?code=c1234567890abcdef...
```

**Response: 200 OK**

```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "user_id": "456e7890-e89b-12d3-a456-426614174000",
    "email": "sarah.smith@icloud.com",
    "first_name": "Sarah",
    "last_name": "Smith",
    "role": "client"
  },
  "expires_at": "2024-12-10T15:30:00Z"
}
```

### Flow 5: Logout

**Scenario:** User logs out of the application.

```bash
POST /api/v1/oauth/logout
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

**Response: 200 OK**

```json
{
  "message": "Logged out successfully"
}
```

---

## Client User Journey

### Step 1: Browse Available Services

**Scenario:** Maria browses massage services (no authentication required).

```bash
GET /api/v1/services
```

**Response: 200 OK**

```json
{
  "services": [
    {
      "service_id": "svc_001",
      "name": "Swedish Massage",
      "description": "Relaxing full-body massage with light to medium pressure",
      "base_price": 1500.0,
      "duration_minutes": 60,
      "category": "massage"
    },
    {
      "service_id": "svc_002",
      "name": "Deep Tissue Massage",
      "description": "Therapeutic massage targeting deep muscle layers",
      "base_price": 1800.0,
      "duration_minutes": 90,
      "category": "massage"
    },
    {
      "service_id": "svc_003",
      "name": "Hot Stone Therapy",
      "description": "Massage with heated stones for ultimate relaxation",
      "base_price": 2200.0,
      "duration_minutes": 120,
      "category": "therapy"
    }
  ]
}
```

### Step 1a: Browse Branches

**Scenario:** Maria checks available branches.

```bash
GET /api/v1/branches
```

**Response: 200 OK**

```json
{
  "branches": [
    {
      "branch_id": "brn_001",
      "name": "Makati Main Branch",
      "address": "456 Ayala Avenue, Makati City"
    }
  ]
}
```

**Get branch details:**

```bash
GET /api/v1/branches/brn_001
```

### Step 1b: View Therapist Profile

**Scenario:** Maria views a therapist's profile and services.

```bash
GET /api/v1/therapists/thr_001
```

**Response: 200 OK**

```json
{
  "therapist_id": "thr_001",
  "name": "Anna Reyes",
  "bio": "Certified massage therapist...",
  "rating": 4.8
}
```

**View therapist's services:**

```bash
GET /api/v1/therapists/thr_001/services
```

### Step 2: Add Delivery Address

**Scenario:** Maria adds her home address for the massage therapist to visit.

```bash
POST /api/v1/addresses
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
Content-Type: application/json

{
  "label": "Home",
  "street_address": "123 Acacia Street",
  "barangay": "San Lorenzo",
  "city": "Makati",
  "province": "Metro Manila",
  "postal_code": "1223",
  "landmark": "Near St. Paul Church",
  "is_default": true
}
```

**Response: 201 Created**

```json
{
  "address_id": "addr_001",
  "label": "Home",
  "street_address": "123 Acacia Street",
  "barangay": "San Lorenzo",
  "city": "Makati",
  "province": "Metro Manila",
  "postal_code": "1223",
  "landmark": "Near St. Paul Church",
  "is_default": true,
  "created_at": "2024-12-09T10:00:00Z"
}
```

### Step 2a: Manage Addresses

**Scenario:** Maria views her saved addresses or updates one.

**List all addresses:**

```bash
GET /api/v1/addresses
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

**Response: 200 OK**

```json
{
  "addresses": [
    {
      "address_id": "addr_001",
      "label": "Home",
      "street_address": "123 Acacia Street",
      "is_default": true
    }
  ]
}
```

**Update an address:**

```bash
PATCH /api/v1/addresses/addr_001
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
Content-Type: application/json

{
  "label": "My Home",
  "landmark": "Green gate"
}
```

**Delete an address:**

```bash
DELETE /api/v1/addresses/addr_001
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

**Set default address:**

```bash
POST /api/v1/addresses/addr_001/default
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

### Step 3: Create a Booking

**Scenario:** Maria books a Swedish Massage for Saturday evening.

```bash
POST /api/v1/bookings
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
Content-Type: application/json

{
  "service_id": "svc_001",
  "address_id": "addr_001",
  "gender_preference": "any",         # one of: "male", "female", "any"
  "pressure_preference": "medium",    # one of: "soft", "medium", "hard"
  "duration_minutes": 60,               # optional, multiples of 30; defaults to 60
  "scheduled_at": "2024-12-14T18:00:00Z", # optional; if omitted server uses current date/time
  "notes": "Please bring lavender aromatherapy oil if available",
  "payment_method": "gcash",          # optional: "cash" or "gcash"
  "voucher_code": "FIRST20",          # optional promotion code
  "raw_total": 1500.00,                # price before voucher/discount
  "total": null                         # optional client-supplied final total
}
```

**Response: 201 Created**

```json
{
  "booking_id": "bkg_001",
  "service_id": "svc_001",
  "service_name": "Swedish Massage",
  "client_id": "123e4567-e89b-12d3-a456-426614174000",
  "client_name": "Maria Santos",
  "address_id": "addr_001",
  "scheduled_at": "2024-12-14T18:00:00Z",
  "status": "pending",
  "gender_preference": "any",
  "pressure_preference": "medium",
  "duration_minutes": 60,
  "raw_total": 1500.0,
  "discount": 300.0,                   # example discount applied from promo
  "total": 1200.0,
  "payment_method": "gcash",
  "voucher_code": "FIRST20",
  "notes": "Please bring lavender aromatherapy oil if available",
  "created_at": "2024-12-09T10:15:00Z"
}
```

### Step 4: Apply Promotion Code

**Scenario:** Maria checks for active promotions.

```bash
GET /api/v1/promotions
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

**Response: 200 OK**

```json
{
  "promotions": [
    {
      "promotion_id": "promo_001",
      "code": "FIRST20",
      "description": "20% off for first-time customers"
    }
  ]
}
```

**Scenario:** Maria applies the promo code.

```bash
GET /api/v1/promotions/code?code=FIRST20
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

**Response: 200 OK**

```json
{
  "promotion_id": "promo_001",
  "code": "FIRST20",
  "discount_percentage": 20,
  "discount_amount": null,
  "valid_from": "2024-12-01T00:00:00Z",
  "valid_until": "2024-12-31T23:59:59Z",
  "max_uses": 1000,
  "current_uses": 156,
  "description": "20% off for first-time customers"
}
```

Note: Instead of PATCHing the booking to add a promotion, clients may send `voucher_code` during booking creation. When `voucher_code` is provided the server will:

- validate the code (time window, availability)
- atomically increment global and per-user usage counters inside a DB transaction
- compute and apply the discount (set `promo_id`, `discount`, and `total`)

If the voucher is invalid or exhausted the booking creation will fail with a 400 and a clear error message.

Common validation error examples (400):

Invalid voucher:

```json
{
  "error": "Invalid voucher code",
  "code": "invalid_voucher",
  "details": { "voucher_code": "Code not found or already expired" }
}
```

Invalid duration (not multiple of 30):

```json
{
  "error": "Invalid duration",
  "code": "invalid_duration",
  "details": { "duration_minutes": "Duration must be a multiple of 30 minutes" }
}
```

Invalid payment method:

```json
{
  "error": "Invalid payment method",
  "code": "invalid_payment_method",
  "details": { "payment_method": "Allowed values: cash, gcash" }
}
```

**Update booking with promotion:**

```bash
PATCH /api/v1/bookings/bkg_001
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
Content-Type: application/json

{
  "promotion_id": "promo_001"
}
```

**Response: 200 OK**

```json
{
  "booking_id": "bkg_001",
  "status": "pending",
  "total_price": 1200.0,
  "discount_applied": 300.0,
  "promotion_code": "FIRST20",
  "updated_at": "2024-12-09T10:18:00Z"
}
```

### Step 5: Make Payment

**Scenario:** Maria pays for the booking via GCash.

```bash
POST /api/v1/payments
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
Content-Type: application/json

{
  "booking_id": "bkg_001",
  "payment_method": "gcash",
  "amount": 1200.00,
  "payment_reference": "GCASH-20241209-ABC123"
}
```

**Response: 201 Created**

```json
{
  "payment_id": "pay_001",
  "booking_id": "bkg_001",
  "amount": 1200.0,
  "payment_method": "gcash",
  "payment_reference": "GCASH-20241209-ABC123",
  "status": "completed",
  "paid_at": "2024-12-09T10:20:00Z"
}
```

### Step 6: Check Booking Details

**Scenario:** Maria checks her booking status.

```bash
GET /api/v1/bookings/bkg_001
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

**Response: 200 OK**

```json
{
  "booking_id": "bkg_001",
  "service": {
    "service_id": "svc_001",
    "name": "Swedish Massage",
    "duration_minutes": 60
  },
  "client": {
    "client_id": "123e4567-e89b-12d3-a456-426614174000",
    "name": "Maria Santos",
    "phone": "+639171234567"
  },
  "therapist": {
    "therapist_id": "thr_001",
    "name": "Anna Reyes",
    "rating": 4.8,
    "total_bookings": 245
  },
  "address": {
    "street_address": "123 Acacia Street",
    "barangay": "San Lorenzo",
    "city": "Makati",
    "landmark": "Near St. Paul Church"
  },
  "scheduled_at": "2024-12-14T18:00:00Z",
  "status": "confirmed",
  "total_price": 1200.0,
  "payment_status": "completed",
  "special_requests": "Please bring lavender aromatherapy oil if available",
  "created_at": "2024-12-09T10:15:00Z",
  "updated_at": "2024-12-09T10:20:00Z"
}
```

### Step 7: Track Therapist Location (Real-Time)

**Scenario:** On appointment day, Maria tracks the therapist's location using WebSocket for real-time updates.

**Option 1: REST API (polling)**

```bash
GET /api/v1/locations/live/thr_001
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

**Response: 200 OK**

```json
{
  "user_id": "thr_001",
  "latitude": 14.5547,
  "longitude": 121.0244,
  "updated_at": "2024-12-14T17:45:00Z",
  "eta_minutes": 15,
  "distance_km": 2.3
}
```

**Option 2: WebSocket (real-time, recommended)**

Maria establishes a WebSocket connection and receives automatic updates when the therapist's location changes. See [Real-Time WebSocket Communication](#real-time-websocket-communication) section below.

### Step 8: Receive Notification

**Scenario:** Maria receives a notification when the therapist is nearby.

```bash
GET /api/v1/notifications
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

**Response: 200 OK**

```json
{
  "notifications": [
    {
      "notification_id": "notif_001",
      "type": "booking_update",
      "title": "Therapist Arriving Soon",
      "message": "Anna is 5 minutes away from your location",
      "is_read": false,
      "created_at": "2024-12-14T17:55:00Z",
      "data": {
        "booking_id": "bkg_001",
        "therapist_name": "Anna Reyes"
      }
    }
  ]
}
```

**Mark as read:**

```bash
POST /api/v1/notifications/notif_001/read
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

**Response: 200 OK**

```json
{
  "message": "Notification marked as read"
}
```

### Step 9: Message Therapist (Real-Time)

**Scenario:** Maria wants to confirm aromatherapy oil availability.

**Create conversation (if not exists):**

```bash
POST /api/v1/messages/conversation
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
Content-Type: application/json

{
  "participant_ids": ["123e4567-e89b-12d3-a456-426614174000", "thr_001"],
  "booking_id": "bkg_001"
}
```

**Response: 201 Created**

```json
{
  "conversation_id": "conv_001",
  "participants": [
    {
      "user_id": "123e4567-e89b-12d3-a456-426614174000",
      "name": "Maria Santos",
      "role": "client"
    },
    {
      "user_id": "thr_001",
      "name": "Anna Reyes",
      "role": "therapist"
    }
  ],
  "created_at": "2024-12-14T17:30:00Z"
}
```

**Send message:**

```bash
POST /api/v1/messages/send
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
Content-Type: application/json

{
  "conversation_id": "conv_001",
  "message": "Hi Anna! Do you have lavender aromatherapy oil available?"
}
```

**Response: 201 Created**

```json
{
  "message_id": "msg_001",
  "conversation_id": "conv_001",
  "sender_id": "123e4567-e89b-12d3-a456-426614174000",
  "message": "Hi Anna! Do you have lavender aromatherapy oil available?",
  "sent_at": "2024-12-14T17:31:00Z",
  "is_read": false
}
```

**Note:** When connected via WebSocket, Anna receives the message instantly without polling. See [Real-Time WebSocket Communication](#real-time-websocket-communication) section below.

### Step 9a: View Message History

**Scenario:** Maria checks her conversation history.

**List conversations:**

```bash
GET /api/v1/messages/conversations
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

**Response: 200 OK**

```json
{
  "conversations": [
    {
      "conversation_id": "conv_001",
      "last_message": "Hi Maria! Yes, I have lavender aromatherapy oil...",
      "updated_at": "2024-12-14T17:32:00Z"
    }
  ]
}
```

**Get messages in a conversation:**

```bash
GET /api/v1/messages/conversation/conv_001
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

**Mark message as read:**

```bash
POST /api/v1/messages/message/msg_001/read
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

### Step 10: Update Booking Status (After Service)

**Scenario:** Service is completed, therapist updates status.

```bash
POST /api/v1/bookings/bkg_001/status
Authorization: Bearer <therapist_token>
Content-Type: application/json

{
  "status": "completed"
}
```

**Response: 200 OK**

```json
{
  "booking_id": "bkg_001",
  "status": "completed",
  "completed_at": "2024-12-14T19:00:00Z"
}
```

### Step 11: Leave a Review

**Scenario:** Maria leaves a 5-star review for Anna.

```bash
POST /api/v1/reviews
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
Content-Type: application/json

{
  "therapist_id": "thr_001",
  "booking_id": "bkg_001",
  "rating": 5,
  "comment": "Anna was amazing! Very professional and the massage was exactly what I needed. Highly recommend!"
}
```

**Response: 201 Created**

```json
{
  "review_id": "rev_001",
  "therapist_id": "thr_001",
  "client_id": "123e4567-e89b-12d3-a456-426614174000",
  "client_name": "Maria Santos",
  "booking_id": "bkg_001",
  "rating": 5,
  "comment": "Anna was amazing! Very professional and the massage was exactly what I needed. Highly recommend!",
  "created_at": "2024-12-14T19:15:00Z"
}
```

### Step 12: Refer a Friend

**Scenario:** Maria refers her friend Lisa.

**Create referral:**

```bash
POST /api/v1/referrals
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
Content-Type: application/json

{
  "referred_user_email": "lisa.tan@gmail.com"
}
```

**Response: 201 Created**

```json
{
  "referral_id": "ref_001",
  "referrer_id": "123e4567-e89b-12d3-a456-426614174000",
  "referral_code": "MARIA123",
  "status": "pending",
  "created_at": "2024-12-14T20:00:00Z"
}
```

**View referral history:**

```bash
GET /api/v1/referrals
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

**Validate referral code:**

```bash
GET /api/v1/referrals/code?code=MARIA123
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

**Check referral rewards:**

```bash
GET /api/v1/referrals/rewards
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

**Response: 200 OK**

```json
{
  "total_referrals": 1,
  "successful_referrals": 0,
  "pending_referrals": 1,
  "rewards": [
    {
      "reward_id": "rwd_001",
      "type": "discount",
      "value": 200.0,
      "description": "₱200 off next booking",
      "status": "locked",
      "unlock_condition": "Friend completes first booking"
    }
  ]
}
```

---

## Therapist Journey

### Step 1: Therapist Registration

**Scenario:** Anna Reyes signs up as a therapist.

```bash
POST /api/v1/register
Content-Type: application/json

{
  "provider": "email",
  "provider_key": "anna.reyes@gmail.com",
  "password": "TherapistPass123!",
  "role": "therapist",
  "referral_code": "" (optional)
}
```

**Response: 201 Created**

```json
{
  "message": "User registered successfully"
}
```

### Step 2: Therapist Login

```bash
POST /api/v1/login
Content-Type: application/json

{
  "provider": "email",
  "provider_key": "anna.reyes@gmail.com",
  "password": "TherapistPass123!"
}
```

**Response: 200 OK**

```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_at": "2024-12-10T15:30:00Z"
}
```

### Step 3: Update Profile

**Scenario:** Anna completes her therapist profile.

```bash
PATCH /api/v1/therapists/profile
Authorization: Bearer <therapist_token>
Content-Type: application/json

{
  "bio": "Certified massage therapist with 8 years of experience specializing in Swedish and deep tissue massage",
  "years_experience": 8,
  "certifications": ["National Certification Board for Therapeutic Massage & Bodywork (NCBTMB)", "Swedish Massage Certification"],
  "phone": "+639171234567",
  "specializations": ["Swedish Massage", "Deep Tissue", "Sports Massage"]
}
```

**Response: 200 OK**

```json
{
  "therapist_id": "thr_001",
  "user_id": "thr_001",
  "bio": "Certified massage therapist with 8 years of experience specializing in Swedish and deep tissue massage",
  "years_experience": 8,
  "certifications": [
    "National Certification Board for Therapeutic Massage & Bodywork (NCBTMB)",
    "Swedish Massage Certification"
  ],
  "rating": 0,
  "total_bookings": 0,
  "is_verified": false,
  "updated_at": "2024-12-09T11:00:00Z"
}
```

### Step 4: Upload Documents

**Scenario:** Anna uploads her certification documents for verification.

```bash
POST /api/v1/therapists/documents
Authorization: Bearer <therapist_token>
Content-Type: application/json

{
  "document_type": "certification",
  "document_url": "https://storage.example.com/docs/anna_ncbtmb_cert.pdf",
  "description": "NCBTMB Certification",
  "expiry_date": "2026-12-31"
}
```

**Response: 201 Created**

```json
{
  "document_id": "doc_001",
  "therapist_id": "thr_001",
  "document_type": "certification",
  "document_url": "https://storage.example.com/docs/anna_ncbtmb_cert.pdf",
  "description": "NCBTMB Certification",
  "status": "pending_verification",
  "expiry_date": "2026-12-31",
  "uploaded_at": "2024-12-09T11:10:00Z"
}
```

### Step 5: Add Services

**Scenario:** Anna adds services she can provide.

```bash
POST /api/v1/therapists/services
Authorization: Bearer <therapist_token>
Content-Type: application/json

{
  "service_id": "svc_001"
}
```

**Response: 201 Created**

```json
{
  "message": "Service added successfully",
  "therapist_id": "thr_001",
  "service_id": "svc_001",
  "service_name": "Swedish Massage"
}
```

**Add another service:**

```bash
POST /api/v1/therapists/services
Authorization: Bearer <therapist_token>
Content-Type: application/json

{
  "service_id": "svc_002"
}
```

**Response: 201 Created**

```json
{
  "message": "Service added successfully",
  "therapist_id": "thr_001",
  "service_id": "svc_002",
  "service_name": "Deep Tissue Massage"
}
```

### Step 5a: Remove Service

**Scenario:** Anna decides to stop offering a specific service.

```bash
DELETE /api/v1/therapists/services/svc_002
Authorization: Bearer <therapist_token>
```

**Response: 200 OK**

```json
{
  "message": "Service removed successfully"
}
```

### Step 6: View Assigned Bookings

**Scenario:** Anna checks her upcoming bookings.

```bash
GET /api/v1/bookings?status=confirmed
Authorization: Bearer <therapist_token>
```

**Response: 200 OK**

```json
{
  "bookings": [
    {
      "booking_id": "bkg_001",
      "service": {
        "service_id": "svc_001",
        "name": "Swedish Massage",
        "duration_minutes": 60
      },
      "client": {
        "client_id": "123e4567-e89b-12d3-a456-426614174000",
        "name": "Maria Santos",
        "phone": "+639171234567"
      },
      "address": {
        "street_address": "123 Acacia Street",
        "barangay": "San Lorenzo",
        "city": "Makati",
        "landmark": "Near St. Paul Church"
      },
      "scheduled_at": "2024-12-14T18:00:00Z",
      "status": "confirmed",
      "special_requests": "Please bring lavender aromatherapy oil if available"
    }
  ]
}
```

### Step 7: Update Live Location (On the Way)

**Scenario:** Anna is heading to Maria's location.

```bash
POST /api/v1/locations/live
Authorization: Bearer <therapist_token>
Content-Type: application/json

{
  "latitude": 14.5547,
  "longitude": 121.0244
}
```

**Response: 200 OK**

```json
{
  "user_id": "thr_001",
  "latitude": 14.5547,
  "longitude": 121.0244,
  "updated_at": "2024-12-14T17:45:00Z"
}
```

### Step 8: Reply to Client Message

**Scenario:** Anna responds to Maria's message about lavender oil.

```bash
POST /api/v1/messages/send
Authorization: Bearer <therapist_token>
Content-Type: application/json

{
  "conversation_id": "conv_001",
  "message": "Hi Maria! Yes, I have lavender aromatherapy oil. See you soon! 😊"
}
```

**Response: 201 Created**

```json
{
  "message_id": "msg_002",
  "conversation_id": "conv_001",
  "sender_id": "thr_001",
  "message": "Hi Maria! Yes, I have lavender aromatherapy oil. See you soon! 😊",
  "sent_at": "2024-12-14T17:32:00Z",
  "is_read": false
}
```

### Step 9: Start Service

**Scenario:** Anna arrives and starts the massage session.

```bash
POST /api/v1/bookings/bkg_001/status
Authorization: Bearer <therapist_token>
Content-Type: application/json

{
  "status": "in_progress"
}
```

**Response: 200 OK**

```json
{
  "booking_id": "bkg_001",
  "status": "in_progress",
  "started_at": "2024-12-14T18:05:00Z"
}
```

### Step 10: Complete Service

**Scenario:** Service is completed.

```bash
POST /api/v1/bookings/bkg_001/status
Authorization: Bearer <therapist_token>
Content-Type: application/json

{
  "status": "completed"
}
```

**Response: 200 OK**

```json
{
  "booking_id": "bkg_001",
  "status": "completed",
  "completed_at": "2024-12-14T19:00:00Z"
}
```

### Step 11: Trigger Emergency Alert (If Needed)

**Scenario:** Anna feels unsafe and triggers an emergency alert.

```bash
POST /api/v1/emergency/trigger
Authorization: Bearer <therapist_token>
Content-Type: application/json

{
  "alert_type": "safety_concern",
  "latitude": 14.5547,
  "longitude": 121.0244,
  "description": "Client behaving inappropriately"
}
```

**Response: 201 Created**

```json
{
  "alert_id": "alert_001",
  "user_id": "thr_001",
  "alert_type": "safety_concern",
  "latitude": 14.5547,
  "longitude": 121.0244,
  "description": "Client behaving inappropriately",
  "status": "active",
  "triggered_at": "2024-12-14T18:30:00Z",
  "response_team_notified": true
}
```

### Step 12: View Reviews

**Scenario:** Anna checks her reviews.

```bash
GET /api/v1/reviews/therapist/thr_001
Authorization: Bearer <therapist_token>
```

**Response: 200 OK**

```json
{
  "therapist_id": "thr_001",
  "average_rating": 4.8,
  "total_reviews": 245,
  "reviews": [
    {
      "review_id": "rev_001",
      "client_name": "Maria Santos",
      "rating": 5,
      "comment": "Anna was amazing! Very professional and the massage was exactly what I needed. Highly recommend!",
      "created_at": "2024-12-14T19:15:00Z"
    },
    {
      "review_id": "rev_002",
      "client_name": "John Doe",
      "rating": 5,
      "comment": "Best massage I've ever had! Anna really knows what she's doing.",
      "created_at": "2024-12-10T20:00:00Z"
    }
  ]
}
```

---

## Admin Operations

### Step 1: Admin Login

**Scenario:** Admin Mike logs in to manage the platform.

```bash
POST /api/v1/login
Content-Type: application/json

{
  "provider": "email",
  "provider_key": "admin@relaxationhub.com",
  "password": "AdminSecure123!"
}
```

**Response: 200 OK**

```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "user_id": "admin_001",
    "role": "admin"
  },
  "expires_at": "2024-12-10T15:30:00Z"
}
```

### Step 2: Create New Service

**Scenario:** Admin adds a new massage service to the catalog.

```bash
POST /api/v1/services
Authorization: Bearer <admin_token>
Content-Type: application/json

{
  "name": "Prenatal Massage",
  "description": "Gentle massage specially designed for expectant mothers",
  "base_price": 2000.00,
  "duration_minutes": 75,
  "category": "massage"
}
```

**Response: 201 Created**

```json
{
  "service_id": "svc_004",
  "name": "Prenatal Massage",
  "description": "Gentle massage specially designed for expectant mothers",
  "base_price": 2000.0,
  "duration_minutes": 75,
  "category": "massage",
  "is_active": true,
  "created_at": "2024-12-09T12:00:00Z"
}
```

### Step 3: Create Promotion

**Scenario:** Admin creates a holiday promotion.

```bash
POST /api/v1/promotions
Authorization: Bearer <admin_token>
Content-Type: application/json

{
  "code": "XMAS2024",
  "description": "Christmas special - 25% off all services",
  "discount_percentage": 25,
  "valid_from": "2024-12-20T00:00:00Z",
  "valid_until": "2024-12-26T23:59:59Z",
  "max_uses": 500
}
```

**Response: 201 Created**

```json
{
  "promotion_id": "promo_002",
  "code": "XMAS2024",
  "description": "Christmas special - 25% off all services",
  "discount_percentage": 25,
  "discount_amount": null,
  "valid_from": "2024-12-20T00:00:00Z",
  "valid_until": "2024-12-26T23:59:59Z",
  "max_uses": 500,
  "current_uses": 0,
  "created_at": "2024-12-09T12:05:00Z"
}
```

### Step 4: Create New Branch

**Scenario:** Admin adds a new branch location.

```bash
POST /api/v1/branches
Authorization: Bearer <admin_token>
Content-Type: application/json

{
  "name": "Makati Main Branch",
  "address": "456 Ayala Avenue, Makati City",
  "phone": "+639171234567",
  "email": "makati@relaxationhub.com",
  "operating_hours": {
    "monday": "08:00-20:00",
    "tuesday": "08:00-20:00",
    "wednesday": "08:00-20:00",
    "thursday": "08:00-20:00",
    "friday": "08:00-20:00",
    "saturday": "09:00-18:00",
    "sunday": "09:00-18:00"
  }
}
```

**Response: 201 Created**

```json
{
  "branch_id": "brn_001",
  "name": "Makati Main Branch",
  "address": "456 Ayala Avenue, Makati City",
  "phone": "+639171234567",
  "email": "makati@relaxationhub.com",
  "operating_hours": {
    "monday": "08:00-20:00",
    "tuesday": "08:00-20:00",
    "wednesday": "08:00-20:00",
    "thursday": "08:00-20:00",
    "friday": "08:00-20:00",
    "saturday": "09:00-18:00",
    "sunday": "09:00-18:00"
  },
  "is_active": true,
  "created_at": "2024-12-09T12:10:00Z"
}
```

### Step 5: Verify Therapist Documents

**Scenario:** Admin reviews and verifies Anna's certification documents.

**Get therapist documents:**

```bash
GET /api/v1/therapists/thr_001/documents
Authorization: Bearer <admin_token>
```

**Response: 200 OK**

```json
{
  "therapist_id": "thr_001",
  "documents": [
    {
      "document_id": "doc_001",
      "document_type": "certification",
      "document_url": "https://storage.example.com/docs/anna_ncbtmb_cert.pdf",
      "description": "NCBTMB Certification",
      "status": "pending_verification",
      "expiry_date": "2026-12-31",
      "uploaded_at": "2024-12-09T11:10:00Z"
    }
  ]
}
```

**Verify document:**

```bash
POST /api/v1/therapists/documents/doc_001/verify
Authorization: Bearer <admin_token>
Content-Type: application/json

{
  "status": "verified",
  "notes": "Valid NCBTMB certification confirmed"
}
```

**Response: 200 OK**

```json
{
  "document_id": "doc_001",
  "status": "verified",
  "verified_by": "admin_001",
  "verified_at": "2024-12-09T12:15:00Z",
  "notes": "Valid NCBTMB certification confirmed"
}
```

### Step 6: Log Admin Action

**Scenario:** Admin logs the verification action for audit trail.

```bash
POST /api/v1/admin/actions
Authorization: Bearer <admin_token>
Content-Type: application/json

{
  "action_type": "document_verification",
  "target_type": "therapist",
  "target_id": "thr_001",
  "description": "Verified NCBTMB certification for Anna Reyes",
  "metadata": {
    "document_id": "doc_001",
    "document_type": "certification"
  }
}
```

**Response: 201 Created**

```json
{
  "action_id": "act_001",
  "admin_id": "admin_001",
  "action_type": "document_verification",
  "target_type": "therapist",
  "target_id": "thr_001",
  "description": "Verified NCBTMB certification for Anna Reyes",
  "metadata": {
    "document_id": "doc_001",
    "document_type": "certification"
  },
  "performed_at": "2024-12-09T12:16:00Z"
}
```

### Step 7: View All Admin Actions

**Scenario:** Admin reviews audit log.

```bash
GET /api/v1/admin/actions?limit=10
Authorization: Bearer <admin_token>
```

**Response: 200 OK**

```json
{
  "total": 127,
  "actions": [
    {
      "action_id": "act_001",
      "admin_id": "admin_001",
      "admin_name": "Mike Admin",
      "action_type": "document_verification",
      "target_type": "therapist",
      "target_id": "thr_001",
      "description": "Verified NCBTMB certification for Anna Reyes",
      "performed_at": "2024-12-09T12:16:00Z"
    },
    {
      "action_id": "act_002",
      "admin_id": "admin_001",
      "admin_name": "Mike Admin",
      "action_type": "promotion_created",
      "target_type": "promotion",
      "target_id": "promo_002",
      "description": "Created Christmas promotion XMAS2024",
      "performed_at": "2024-12-09T12:05:00Z"
    }
  ]
}
```

### Step 7a: View My Actions

**Scenario:** Admin Mike checks his own activity log.

```bash
GET /api/v1/admin/actions/me
Authorization: Bearer <admin_token>
```

### Step 8: Handle Emergency Alert

**Scenario:** Admin reviews and responds to Anna's emergency alert.

```bash
GET /api/v1/emergency/alert/alert_001
Authorization: Bearer <admin_token>
```

**Response: 200 OK**

```json
{
  "alert_id": "alert_001",
  "user_id": "thr_001",
  "user_name": "Anna Reyes",
  "user_role": "therapist",
  "alert_type": "safety_concern",
  "latitude": 14.5547,
  "longitude": 121.0244,
  "address": "123 Acacia Street, San Lorenzo, Makati",
  "description": "Client behaving inappropriately",
  "status": "active",
  "triggered_at": "2024-12-14T18:30:00Z",
  "booking_id": "bkg_001",
  "client_info": {
    "client_id": "123e4567-e89b-12d3-a456-426614174000",
    "name": "Maria Santos",
    "phone": "+639171234567"
  }
}
```

**Resolve alert:**

```bash
POST /api/v1/emergency/alert/alert_001/resolve
Authorization: Bearer <admin_token>
Content-Type: application/json

{
  "resolution_notes": "Contacted therapist. Situation resolved. Client warned about appropriate behavior."
}
```

**Response: 200 OK**

```json
{
  "alert_id": "alert_001",
  "status": "resolved",
  "resolved_by": "admin_001",
  "resolved_at": "2024-12-14T18:45:00Z",
  "resolution_notes": "Contacted therapist. Situation resolved. Client warned about appropriate behavior."
}
```

### Step 9: List All Therapists

**Scenario:** Admin views all registered therapists.

```bash
GET /api/v1/therapists?status=all
Authorization: Bearer <admin_token>
```

**Response: 200 OK**

```json
{
  "total": 45,
  "therapists": [
    {
      "therapist_id": "thr_001",
      "name": "Anna Reyes",
      "email": "anna.reyes@gmail.com",
      "phone": "+639171234567",
      "rating": 4.8,
      "total_bookings": 245,
      "is_verified": true,
      "is_active": true,
      "joined_at": "2024-01-15T10:00:00Z"
    },
    {
      "therapist_id": "thr_002",
      "name": "Ben Cruz",
      "email": "ben.cruz@gmail.com",
      "phone": "+639181234567",
      "rating": 4.6,
      "total_bookings": 189,
      "is_verified": true,
      "is_active": true,
      "joined_at": "2024-02-20T09:30:00Z"
    }
  ]
}
```

### Step 10: View Payment for Booking

**Scenario:** Admin checks payment details for a booking.

```bash
GET /api/v1/payments/booking/bkg_001
Authorization: Bearer <admin_token>
```

**Response: 200 OK**

```json
{
  "payment_id": "pay_001",
  "booking_id": "bkg_001",
  "client_id": "123e4567-e89b-12d3-a456-426614174000",
  "client_name": "Maria Santos",
  "amount": 1200.0,
  "payment_method": "gcash",
  "payment_reference": "GCASH-20241209-ABC123",
  "status": "completed",
  "paid_at": "2024-12-09T10:20:00Z"
}
```

### Step 11: Send System Notification

**Scenario:** Admin sends a manual notification to a user.

```bash
POST /api/v1/notifications
Authorization: Bearer <admin_token>
Content-Type: application/json

{
  "user_id": "123e4567-e89b-12d3-a456-426614174000",
  "title": "System Maintenance",
  "message": "Scheduled maintenance tonight at 10 PM.",
  "type": "system_alert"
}
```

**Response: 201 Created**

```json
{
  "notification_id": "notif_003",
  "message": "Notification sent successfully"
}
```

---

## Real-World Scenarios

### Scenario 1: Client Cancels Booking

**Step 1: Client requests cancellation**

```bash
PATCH /api/v1/bookings/bkg_001
Authorization: Bearer <client_token>
Content-Type: application/json

{
  "status": "cancelled",
  "cancellation_reason": "Schedule conflict, need to reschedule"
}
```

**Response: 200 OK**

```json
{
  "booking_id": "bkg_001",
  "status": "cancelled",
  "cancellation_reason": "Schedule conflict, need to reschedule",
  "cancelled_at": "2024-12-13T10:00:00Z",
  "refund_eligible": true,
  "refund_amount": 1200.0
}
```

**Step 2: Admin processes refund**

```bash
POST /api/v1/payments/booking/bkg_001/status
Authorization: Bearer <admin_token>
Content-Type: application/json

{
  "status": "refunded",
  "refund_amount": 1200.00,
  "refund_reference": "REF-20241213-XYZ789"
}
```

**Response: 200 OK**

```json
{
  "payment_id": "pay_001",
  "booking_id": "bkg_001",
  "status": "refunded",
  "refund_amount": 1200.0,
  "refund_reference": "REF-20241213-XYZ789",
  "refunded_at": "2024-12-13T10:30:00Z"
}
```

### Scenario 2: Therapist Reschedules

**Step 1: Therapist requests reschedule**

```bash
PATCH /api/v1/bookings/bkg_002
Authorization: Bearer <therapist_token>
Content-Type: application/json

{
  "scheduled_at": "2024-12-15T19:00:00Z",
  "reschedule_reason": "Previous appointment running late"
}
```

**Response: 200 OK**

```json
{
  "booking_id": "bkg_002",
  "original_time": "2024-12-15T18:00:00Z",
  "new_time": "2024-12-15T19:00:00Z",
  "reschedule_reason": "Previous appointment running late",
  "status": "rescheduled",
  "client_notified": true
}
```

**Step 2: Client receives notification**

```bash
GET /api/v1/notifications
Authorization: Bearer <client_token>
```

**Response: 200 OK**

```json
{
  "notifications": [
    {
      "notification_id": "notif_002",
      "type": "booking_rescheduled",
      "title": "Booking Rescheduled",
      "message": "Your booking has been rescheduled from 6:00 PM to 7:00 PM",
      "is_read": false,
      "created_at": "2024-12-15T17:30:00Z",
      "data": {
        "booking_id": "bkg_002",
        "original_time": "2024-12-15T18:00:00Z",
        "new_time": "2024-12-15T19:00:00Z"
      }
    }
  ]
}
```

### Scenario 3: Referral Completion

**Step 1: Lisa (referred friend) completes registration using Maria's code**

```bash
POST /api/v1/register
Content-Type: application/json

{
  "provider": "email",
  "provider_key": "lisa.tan@gmail.com",
  "password": "SecurePass123!",
  "role": "client",
  "referral_code": "MARIA123"
}
```

**Response: 201 Created**

```json
{
  "message": "User registered successfully",
  "referral_applied": true,
  "referrer_name": "Maria Santos",
  "bonus": {
    "type": "discount",
    "value": 200.0,
    "description": "₱200 off first booking"
  }
}
```

**Step 2: Lisa makes her first booking and completes it**

_[Same booking flow as Client User Journey]_

**Step 3: Maria receives referral reward**

```bash
GET /api/v1/referrals/rewards
Authorization: Bearer <maria_token>
```

**Response: 200 OK**

```json
{
  "total_referrals": 1,
  "successful_referrals": 1,
  "pending_referrals": 0,
  "rewards": [
    {
      "reward_id": "rwd_001",
      "type": "discount",
      "value": 200.0,
      "description": "₱200 off next booking",
      "status": "available",
      "unlocked_at": "2024-12-15T20:00:00Z"
    }
  ]
}
```

**Step 4: Maria redeems reward**

```bash
POST /api/v1/referrals/rewards/rwd_001/redeem
Authorization: Bearer <maria_token>
```

**Response: 200 OK**

```json
{
  "reward_id": "rwd_001",
  "status": "redeemed",
  "coupon_code": "REF-MARIA-200",
  "redeemed_at": "2024-12-16T10:00:00Z",
  "message": "Your ₱200 discount has been added. Use code REF-MARIA-200 at checkout"
}
```

### Scenario 4: Branch-Specific Service

**Step 1: Admin assigns therapist to branch**

```bash
PATCH /api/v1/branches/brn_001
Authorization: Bearer <admin_token>
Content-Type: application/json

{
  "therapist_ids": ["thr_001", "thr_002", "thr_003"]
}
```

**Response: 200 OK**

```json
{
  "branch_id": "brn_001",
  "name": "Makati Main Branch",
  "therapists": [
    {
      "therapist_id": "thr_001",
      "name": "Anna Reyes",
      "rating": 4.8
    },
    {
      "therapist_id": "thr_002",
      "name": "Ben Cruz",
      "rating": 4.6
    },
    {
      "therapist_id": "thr_003",
      "name": "Clara Santos",
      "rating": 4.9
    }
  ],
  "updated_at": "2024-12-09T13:00:00Z"
}
```

**Step 2: Client books service at specific branch**

```bash
POST /api/v1/bookings
Authorization: Bearer <client_token>
Content-Type: application/json

{
  "service_id": "svc_001",
  "branch_id": "brn_001",
  "scheduled_at": "2024-12-16T14:00:00Z",
  "preference": "female_therapist"
}
```

**Response: 201 Created**

```json
{
  "booking_id": "bkg_003",
  "service_id": "svc_001",
  "service_name": "Swedish Massage",
  "branch_id": "brn_001",
  "branch_name": "Makati Main Branch",
  "branch_address": "456 Ayala Avenue, Makati City",
  "therapist_id": "thr_003",
  "therapist_name": "Clara Santos",
  "scheduled_at": "2024-12-16T14:00:00Z",
  "status": "pending",
  "total_price": 1500.0
}
```

---

## Error Responses

### Authentication Errors

**401 Unauthorized - Missing Token**

```json
{
  "error": "Missing authorization token"
}
```

**401 Unauthorized - Invalid Token**

```json
{
  "error": "Invalid or expired token"
}
```

**403 Forbidden - Insufficient Permissions**

```json
{
  "error": "Insufficient permissions to access this resource"
}
```

### Validation Errors

**400 Bad Request - Invalid Input**

```json
{
  "error": "Invalid request body",
  "details": {
    "scheduled_at": "Must be a future date and time",
    "service_id": "Service ID is required"
  }
}
```

**409 Conflict - Duplicate Resource**

```json
{
  "error": "Email already in use"
}
```

### Resource Errors

**404 Not Found**

```json
{
  "error": "Booking not found"
}
```

**422 Unprocessable Entity**

```json
{
  "error": "Cannot cancel booking less than 24 hours before scheduled time"
}
```

---

## Testing Guide

### Using cURL

**Register a new user:**

````bash
curl -X POST http://localhost:8080/api/v1/register \
  ## Onboarding Flow

  After registration, users should complete onboarding to provide profile details required for bookings and safety:

  - Full name
  - Gender (options: male, female, other, prefer_not_to_say)
  - Address (one or more; used for booking locations)
  - Emergency contact name
  - Emergency contact phone number

  These fields can be updated later via the authenticated `PATCH /api/v1/profile` endpoint. Example payload:

  ```json
  {
    "full_name": "Jane Doe",
    "gender": "female",
    "profile_photo": "https://cdn.example.com/avatar.jpg",
    "emergency_contact_name": "John Doe",
    "emergency_contact_phone": "+63917..."
  }
````

The onboarding order is intentionally flexible but the minimum recommended sequence is: register -> provide full name -> set gender -> add address -> set emergency contact.

-H "Content-Type: application/json" \
 -d '{
"provider": "email",
"provider_key": "test@example.com",
"password": "TestPass123!",
"role": "client"
}'

````

**Login and save token:**

```bash
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/login \
  -H "Content-Type: application/json" \
  -d '{
    "provider": "email",
    "provider_key": "test@example.com",
    "password": "TestPass123!"
  }' | jq -r '.token')

echo $TOKEN
````

**Make authenticated request:**

```bash
curl -X GET http://localhost:8080/api/v1/bookings \
  -H "Authorization: Bearer $TOKEN"
```

### Using Postman

1. **Import Collection:**

   - Create a new collection: "Relaxation Hub API"
   - Add environment variables:
     - `base_url`: `http://localhost:8080/api/v1`
     - `token`: (will be set automatically)

2. **Setup Scripts:**

   - In Login request, add to "Tests" tab:
     ```javascript
     pm.test("Login successful", function () {
       pm.response.to.have.status(200);
       var jsonData = pm.response.json();
       pm.environment.set("token", jsonData.token);
     });
     ```

3. **Authorization:**
   - In collection settings → Authorization
   - Type: Bearer Token
   - Token: `{{token}}`

---

## Real-Time WebSocket Communication

### Overview

The WebSocket endpoint provides persistent, bidirectional communication for real-time features. Instead of polling REST endpoints, clients maintain an open connection and receive instant updates.

**WebSocket URL:** `ws://localhost:8080/api/v1/ws` (or `wss://` for production)

### Authentication

WebSocket connections require JWT authentication. Pass the token as a query parameter or in the `Authorization` header during the upgrade request.

**Example connection (JavaScript):**

```javascript
const token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...";
const ws = new WebSocket(`ws://localhost:8080/api/v1/ws?token=${token}`);
// or in some clients:
// headers: { Authorization: `Bearer ${token}` }

ws.onopen = () => {
  console.log("WebSocket connected");
};

ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  handleWebSocketMessage(data);
};

ws.onerror = (error) => {
  console.error("WebSocket error:", error);
};

ws.onclose = () => {
  console.log("WebSocket disconnected");
};
```

### Message Types

#### 1. New Message (`new_message`)

Sent when a message is delivered in a conversation you're part of.

**Server → Client:**

```json
{
  "type": "new_message",
  "data": {
    "message_id": "msg_123",
    "conversation_id": "conv_001",
    "sender_id": 456,
    "sender_name": "Anna Reyes",
    "content": "Yes, I have lavender oil with me!",
    "sent_at": "2024-12-14T17:32:00Z",
    "is_read": false
  }
}
```

**Frontend handling:**

```javascript
function handleWebSocketMessage(message) {
  switch (message.type) {
    case "new_message":
      // Update chat UI instantly
      appendMessageToChat(message.data);
      playNotificationSound();
      updateConversationList(message.data.conversation_id);
      break;
  }
}
```

#### 2. Location Update (`location_update`)

Sent when a tracked user updates their GPS coordinates.

**Server → Client:**

```json
{
  "type": "location_update",
  "data": {
    "user_id": 789,
    "latitude": 14.5547,
    "longitude": 121.0244,
    "accuracy": 10.5,
    "updated_at": "2024-12-14T17:45:30Z"
  }
}
```

**Frontend handling:**

```javascript
function handleWebSocketMessage(message) {
  switch (message.type) {
    case "location_update":
      // Update map marker position
      updateMapMarker(message.data.user_id, {
        lat: message.data.latitude,
        lng: message.data.longitude,
      });
      calculateETA(message.data);
      break;
  }
}
```

### Connection Lifecycle

#### 1. Establishing Connection

```javascript
// Connect with JWT token
const ws = new WebSocket(`ws://localhost:8080/api/v1/ws?token=${jwtToken}`);

ws.onopen = () => {
  console.log("Connected to real-time server");
  // No additional handshake required
};
```

#### 2. Heartbeat / Keep-Alive

The server automatically sends ping frames every 54 seconds. Your client should respond with pong frames (most WebSocket libraries handle this automatically).

**Manual ping handling (if needed):**

```javascript
ws.onping = () => {
  console.log("Received ping from server");
  // Most libraries auto-respond with pong
};
```

#### 3. Reconnection Strategy

```javascript
let ws;
let reconnectAttempts = 0;
const maxReconnectDelay = 30000; // 30 seconds

function connect() {
  ws = new WebSocket(`ws://localhost:8080/api/v1/ws?token=${getToken()}`);

  ws.onopen = () => {
    reconnectAttempts = 0;
    console.log("WebSocket connected");
  };

  ws.onclose = () => {
    // Exponential backoff
    const delay = Math.min(
      1000 * Math.pow(2, reconnectAttempts),
      maxReconnectDelay
    );
    reconnectAttempts++;

    console.log(`Reconnecting in ${delay}ms...`);
    setTimeout(connect, delay);
  };

  ws.onerror = (error) => {
    console.error("WebSocket error:", error);
    ws.close();
  };

  ws.onmessage = (event) => {
    const message = JSON.parse(event.data);
    handleWebSocketMessage(message);
  };
}

connect();
```

### Use Cases

#### Real-Time Chat

**Scenario:** Maria and Anna chatting about the booking.

1. Maria opens chat → Frontend establishes WebSocket connection
2. Maria sends message via `POST /api/v1/messages/send`
3. Server broadcasts to Anna's WebSocket connection
4. Anna's app displays message instantly (no polling)

**Benefits over REST polling:**

- Instant delivery (< 100ms vs 1-5 second polling)
- Reduced server load (1 connection vs continuous requests)
- Battery efficient on mobile devices

#### Live Location Tracking

**Scenario:** Client tracking therapist en route.

1. Client opens booking details → Subscribes to location updates
2. Therapist app updates location via `POST /api/v1/locations/live`
3. Server broadcasts to client's WebSocket connection
4. Client's map updates therapist marker position

**Benefits:**

- Smooth map animations (updates every 5-10 seconds)
- No polling delays
- Real-time ETA calculations

### Error Handling

#### Connection Errors

```javascript
ws.onerror = (error) => {
  console.error("WebSocket error:", error);

  // Show user-friendly message
  showNotification("Connection issue. Retrying...", "warning");

  // Fallback to REST polling
  startPollingFallback();
};
```

#### Authentication Failure

If JWT token is invalid or expired, the server closes the connection immediately.

**Response:** WebSocket closes with code `1008` (Policy Violation)

**Handling:**

```javascript
ws.onclose = (event) => {
  if (event.code === 1008) {
    console.error("Authentication failed");
    // Redirect to login
    window.location.href = "/login";
  }
};
```

### Testing WebSocket

#### Using `wscat` (CLI tool)

```bash
# Install
npm install -g wscat

# Connect
wscat -c "ws://localhost:8080/api/v1/ws?token=YOUR_JWT_TOKEN"

# Wait for messages
Connected (press CTRL+C to quit)
> {"type":"new_message","data":{...}}
```

#### Using Browser Console

```javascript
// In browser console
const token = "YOUR_JWT_TOKEN";
const ws = new WebSocket(`ws://localhost:8080/api/v1/ws?token=${token}`);

ws.onmessage = (event) => {
  console.log("Received:", JSON.parse(event.data));
};

// Trigger a test by sending a message via REST API in another tab
```

#### Using Postman

1. Create WebSocket request
2. URL: `ws://localhost:8080/api/v1/ws?token=YOUR_JWT_TOKEN`
3. Click "Connect"
4. Monitor incoming messages in "Messages" tab

### Performance Characteristics

- **Max message size:** 512 KB
- **Ping interval:** 54 seconds
- **Pong timeout:** 60 seconds
- **Connection limit:** Determined by server resources
- **Auto-reconnect:** Client responsibility (see reconnection strategy above)

### Security Considerations

1. **Authentication:** Always required via JWT token
2. **User isolation:** Users only receive messages intended for them
3. **Message validation:** Server validates all message types
4. **Rate limiting:** Connection attempts rate-limited per IP
5. **TLS/SSL:** Use `wss://` in production (encrypted WebSocket)

---

## Summary

This documentation covers complete user journeys including:

✅ **Client Journey (12 steps):** Registration → Browse services → Book → Pay → Track → Review → Refer  
✅ **Therapist Journey (12 steps):** Registration → Profile setup → Document upload → Service management → Booking fulfillment → Emergency handling  
✅ **Admin Operations (10 steps):** Service creation → Promotion management → Branch setup → Document verification → Emergency response → Audit logging  
✅ **Real-World Scenarios:** Cancellations, reschedules, referrals, branch-specific bookings  
✅ **Real-Time WebSocket:** Chat messaging, live location tracking, connection management  
✅ **Error Handling:** Complete error response documentation  
✅ **Testing Guide:** cURL, Postman, and WebSocket testing examples

**Total API Endpoints Documented:** 40+ REST + WebSocket  
**Complete User Flows:** 3 major journeys + real-time communication  
**Real-World Scenarios:** 4 complex flows

---

**Last Updated:** December 9, 2024  
**API Version:** v1  
**Status:** Production Ready
