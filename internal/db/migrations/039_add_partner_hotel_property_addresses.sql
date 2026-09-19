-- Hotel accounts book service at their registered property. Store that
-- property as the account's default service address so the hotel booking form
-- does not need to ask staff for the same address on every booking.
INSERT INTO public.addresses (
    user_id,
    label,
    street_address,
    city,
    country,
    is_default
)
SELECT
    staff.user_id,
    'Hotel property',
    hotel.address_line,
    hotel.city,
    'Philippines',
    NOT EXISTS (
        SELECT 1
        FROM public.addresses existing_default
        WHERE existing_default.user_id = staff.user_id
          AND existing_default.deleted_at IS NULL
          AND existing_default.is_default
    )
FROM public.partner_hotel_staff staff
JOIN public.partner_hotels hotel
  ON hotel.partner_hotel_id = staff.partner_hotel_id
WHERE staff.user_id IS NOT NULL
  AND btrim(hotel.address_line) <> ''
  AND btrim(hotel.city) <> ''
  AND NOT EXISTS (
      SELECT 1
      FROM public.addresses existing_property
      WHERE existing_property.user_id = staff.user_id
        AND existing_property.deleted_at IS NULL
        AND existing_property.label = 'Hotel property'
  );
