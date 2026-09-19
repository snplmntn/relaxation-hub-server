ALTER TABLE public.bookings
    ADD COLUMN IF NOT EXISTS partner_hotel_id BIGINT
        REFERENCES public.partner_hotels(partner_hotel_id) ON DELETE SET NULL;

ALTER TABLE public.recurring_bookings
    ADD COLUMN IF NOT EXISTS partner_hotel_id BIGINT
        REFERENCES public.partner_hotels(partner_hotel_id) ON DELETE SET NULL;

-- Preserve every existing booking that already has an unambiguous hotel
-- account owner.
UPDATE public.bookings AS booking
SET partner_hotel_id = staff.partner_hotel_id
FROM public.partner_hotel_staff AS staff
WHERE staff.user_id = booking.client_id
  AND booking.partner_hotel_id IS NULL;

-- Recover legacy bookings made for ordinary clients only when the saved
-- service address identifies exactly one partnered property.
WITH address_matches AS (
    SELECT booking.booking_id, MIN(hotel.partner_hotel_id) AS partner_hotel_id
    FROM public.bookings AS booking
    JOIN public.addresses AS address ON address.address_id = booking.address_id
    JOIN public.partner_hotels AS hotel
      ON lower(btrim(address.street_address)) = lower(btrim(hotel.address_line))
     AND lower(btrim(address.city)) = lower(btrim(hotel.city))
    WHERE booking.partner_hotel_id IS NULL
      AND btrim(hotel.address_line) <> ''
      AND btrim(hotel.city) <> ''
    GROUP BY booking.booking_id
    HAVING COUNT(DISTINCT hotel.partner_hotel_id) = 1
)
UPDATE public.bookings AS booking
SET partner_hotel_id = address_matches.partner_hotel_id
FROM address_matches
WHERE address_matches.booking_id = booking.booking_id;

-- Keep future occurrences tied to the same property as their series.
UPDATE public.recurring_bookings AS recurring
SET partner_hotel_id = staff.partner_hotel_id
FROM public.partner_hotel_staff AS staff
WHERE staff.user_id = recurring.client_id
  AND recurring.partner_hotel_id IS NULL;

WITH address_matches AS (
    SELECT recurring.recurring_id, MIN(hotel.partner_hotel_id) AS partner_hotel_id
    FROM public.recurring_bookings AS recurring
    JOIN public.addresses AS address ON address.address_id = recurring.address_id
    JOIN public.partner_hotels AS hotel
      ON lower(btrim(address.street_address)) = lower(btrim(hotel.address_line))
     AND lower(btrim(address.city)) = lower(btrim(hotel.city))
    WHERE recurring.partner_hotel_id IS NULL
      AND btrim(hotel.address_line) <> ''
      AND btrim(hotel.city) <> ''
    GROUP BY recurring.recurring_id
    HAVING COUNT(DISTINCT hotel.partner_hotel_id) = 1
)
UPDATE public.recurring_bookings AS recurring
SET partner_hotel_id = address_matches.partner_hotel_id
FROM address_matches
WHERE address_matches.recurring_id = recurring.recurring_id;

CREATE INDEX IF NOT EXISTS idx_bookings_partner_hotel_schedule
    ON public.bookings (partner_hotel_id, scheduled_start)
    WHERE partner_hotel_id IS NOT NULL;
