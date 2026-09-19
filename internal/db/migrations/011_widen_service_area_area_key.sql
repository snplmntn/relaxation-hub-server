-- 011. Widen service-area keys for generated city/barangay keys.
--
-- Earlier deployed schemas used VARCHAR(20) for area_key. Current code and the
-- consolidated baseline use TEXT because generated keys can include both
-- barangay and city, for example: barangay:galas|city:quezon-city.

ALTER TABLE public.area_coverage_requests
    ALTER COLUMN area_key TYPE TEXT USING area_key::TEXT;

ALTER TABLE public.service_areas
    ALTER COLUMN area_key TYPE TEXT USING area_key::TEXT;
