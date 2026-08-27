CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_rides_unique_active_booking_leg
    ON public.rides (booking_id, (COALESCE(ride_type, 'outbound')))
    WHERE booking_id IS NOT NULL
      AND COALESCE(status, 'pending') NOT IN ('cancelled', 'completed', 'declined', 'unmatched');
