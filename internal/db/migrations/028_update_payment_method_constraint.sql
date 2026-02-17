-- Drop the existing constraint
ALTER TABLE bookings DROP CONSTRAINT IF EXISTS bookings_payment_method_check;

-- Add the new constraint with updated values
ALTER TABLE bookings ADD CONSTRAINT bookings_payment_method_check 
    CHECK (payment_method IN ('cash', 'gcash', 'bdo', 'bank_transfer'));
