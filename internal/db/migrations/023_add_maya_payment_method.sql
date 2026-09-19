-- Add 'maya' as an accepted payment method for booking_groups and bookings.

ALTER TABLE booking_groups
    DROP CONSTRAINT IF EXISTS booking_groups_payment_method_check,
    ADD CONSTRAINT booking_groups_payment_method_check
        CHECK (payment_method IN ('cash', 'gcash', 'bdo', 'maya'));

ALTER TABLE bookings
    DROP CONSTRAINT IF EXISTS bookings_payment_method_check,
    ADD CONSTRAINT bookings_payment_method_check
        CHECK (payment_method IN ('cash', 'gcash', 'bdo', 'bank_transfer', 'maya'));
