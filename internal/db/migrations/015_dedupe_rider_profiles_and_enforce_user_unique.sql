-- Rider profile IDs are the canonical FK for rides and ride_offers, while
-- users.user_id is the stable identity used by auth, wallet, and admin lists.
-- Keep exactly one rider_profiles row per user so user->profile resolution is
-- deterministic.

CREATE TEMP TABLE tmp_rider_profile_dedupe AS
WITH ranked AS (
    SELECT
        rp.rider_id,
        rp.user_id,
        FIRST_VALUE(rp.rider_id) OVER (
            PARTITION BY rp.user_id
            ORDER BY
                CASE WHEN EXISTS (
                    SELECT 1
                    FROM public.rides r
                    WHERE r.rider_id = rp.rider_id
                      AND r.status IN ('accepted', 'arrived_pickup', 'in_progress', 'arrived_dropoff')
                ) THEN 0 ELSE 1 END,
                rp.updated_at DESC,
                rp.rider_id DESC
        ) AS keep_rider_id
    FROM public.rider_profiles rp
)
SELECT
    rider_id AS old_rider_id,
    user_id,
    keep_rider_id
FROM ranked
WHERE rider_id <> keep_rider_id;

UPDATE public.rides r
SET rider_id = d.keep_rider_id
FROM tmp_rider_profile_dedupe d
WHERE r.rider_id = d.old_rider_id;

DELETE FROM public.ride_offers ro
USING tmp_rider_profile_dedupe d
WHERE ro.rider_id = d.old_rider_id
  AND EXISTS (
      SELECT 1
      FROM public.ride_offers existing
      WHERE existing.ride_id = ro.ride_id
        AND existing.rider_id = d.keep_rider_id
  );

UPDATE public.ride_offers ro
SET rider_id = d.keep_rider_id
FROM tmp_rider_profile_dedupe d
WHERE ro.rider_id = d.old_rider_id;

DELETE FROM public.rider_profiles rp
USING tmp_rider_profile_dedupe d
WHERE rp.rider_id = d.old_rider_id;

DROP TABLE tmp_rider_profile_dedupe;

CREATE UNIQUE INDEX IF NOT EXISTS idx_rider_profiles_user_id_unique
    ON public.rider_profiles(user_id);
