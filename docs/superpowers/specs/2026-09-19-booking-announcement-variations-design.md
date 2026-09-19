# Booking Announcement Variations Design

Date: 2026-09-19

## Goal

Let staff create booking-announcement campaigns with one or more message variations. Multiple variations are assigned across bookings until staff manually chooses a winner. The application never records conversion or performance data.

Announcements appear at the bottom of Day View's copied booking details and in the booking-completed email only.

## Behavior

- One variation is a normal campaign.
- Multiple variations with no winner are distributed deterministically across bookings.
- Once staff selects a winner, only that version is assigned to new bookings.
- Staff may replace the winner, but the campaign does not return to distribution mode.
- Existing booking assignments retain their message snapshot after edits or winner changes.
- Tandem/group bookings share the same assignment.
- Multiple date-overlapping campaigns are allowed and each contributes one message.
- Eligibility uses the booking's existing Day View business date, including its 4:00 AM rollover.

## Storage

Store campaign variations, the optional winner, and one operational assignment per campaign and booking/group. Assignments contain a message snapshot solely to keep repeated copies and the completed email consistent. Do not add conversion counts, impressions, clicks, customer exposure screens, analytics, or automatic winner selection.

## API and UI

Campaign CRUD returns ordered variations and an optional winner. Super Admin manages campaigns and chooses winners. Operational admins may resolve announcements for a booking through an endpoint that loads the booking's authoritative schedule and group.

The Administration page starts with one required version, allows unlimited additional versions, shows all versions, and marks the winner. Day View resolves messages before copying; on resolution failure it reports the error and does not copy incomplete details.

## Email

The booking-completed email resolves the same stored assignments and appends an Announcements section to plain-text and HTML bodies. Advanced confirmation, booking-day, and therapist-on-the-way emails remain unchanged.

## Validation

- Reject blank or empty variation lists.
- Reject end dates before start dates.
- Reject deleting the last variation.
- Reject deleting the winning variation until another winner is selected.
- Reject winner IDs that do not belong to the campaign.
- Make assignment creation idempotent under concurrent requests.
