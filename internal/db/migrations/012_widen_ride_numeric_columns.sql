-- Widen ride coordinate and distance columns.
--
-- Legacy DECIMAL precision can reject otherwise valid generated ride data with
-- SQLSTATE 22003 numeric field overflow. Ride values are calculated in Go as
-- float64, so store them as DOUBLE PRECISION to match the application boundary.

ALTER TABLE public.rides
    ALTER COLUMN pickup_lat TYPE DOUBLE PRECISION USING pickup_lat::DOUBLE PRECISION;

ALTER TABLE public.rides
    ALTER COLUMN pickup_long TYPE DOUBLE PRECISION USING pickup_long::DOUBLE PRECISION;

ALTER TABLE public.rides
    ALTER COLUMN dropoff_lat TYPE DOUBLE PRECISION USING dropoff_lat::DOUBLE PRECISION;

ALTER TABLE public.rides
    ALTER COLUMN dropoff_long TYPE DOUBLE PRECISION USING dropoff_long::DOUBLE PRECISION;

ALTER TABLE public.rides
    ALTER COLUMN distance_km TYPE DOUBLE PRECISION USING distance_km::DOUBLE PRECISION;
