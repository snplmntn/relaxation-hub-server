DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM information_schema.columns
    WHERE table_name = 'service_areas'
      AND column_name = 'psgc_code'
  ) THEN
    ALTER TABLE service_areas RENAME COLUMN psgc_code TO area_key;
  END IF;
END$$;

DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM information_schema.columns
    WHERE table_name = 'area_coverage_requests'
      AND column_name = 'psgc_code'
  ) THEN
    ALTER TABLE area_coverage_requests RENAME COLUMN psgc_code TO area_key;
  END IF;
END$$;
