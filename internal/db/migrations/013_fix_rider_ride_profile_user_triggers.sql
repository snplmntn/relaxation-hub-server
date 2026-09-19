-- Keep ride assignment keyed by rider_profiles.rider_id while wallet and
-- performance tables remain keyed by users.user_id.
--
-- rides.rider_id has a foreign key to rider_profiles(rider_id). The active
-- trigger functions were inserting NEW.rider_id directly into rider wallet and
-- performance tables, whose rider_id columns reference users(user_id), causing
-- FK violations whenever rider_profile IDs differ from user IDs.

CREATE OR REPLACE FUNCTION update_rider_wallet_on_earning()
RETURNS TRIGGER AS $$
DECLARE
    v_rider_user_id BIGINT;
BEGIN
    IF NEW.rider_earnings_cents IS NOT NULL
       AND NEW.status = 'completed'
       AND OLD.status IS DISTINCT FROM 'completed'
       AND NEW.rider_id IS NOT NULL THEN

        SELECT user_id INTO v_rider_user_id
        FROM public.rider_profiles
        WHERE rider_id = NEW.rider_id;

        IF v_rider_user_id IS NULL THEN
            RAISE EXCEPTION 'rider profile % has no user for wallet update', NEW.rider_id;
        END IF;

        INSERT INTO public.rider_wallets (rider_id, balance_cents, total_earned_cents)
        VALUES (v_rider_user_id, 0, 0)
        ON CONFLICT (rider_id) DO NOTHING;

        UPDATE public.rider_wallets
        SET
            balance_cents = balance_cents + NEW.rider_earnings_cents,
            total_earned_cents = total_earned_cents + NEW.rider_earnings_cents,
            updated_at = NOW()
        WHERE rider_id = v_rider_user_id;

        INSERT INTO public.rider_transactions (rider_id, transaction_type, amount_cents, ride_id, status, description)
        VALUES (
            v_rider_user_id,
            'ride_earning',
            NEW.rider_earnings_cents,
            NEW.ride_id,
            'completed',
            FORMAT('Earnings from ride #%s', NEW.ride_id)
        );
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION update_rider_performance_metrics()
RETURNS TRIGGER AS $$
DECLARE
    v_rider_user_id BIGINT;
BEGIN
    IF NEW.rider_id IS NULL THEN
        RETURN NEW;
    END IF;

    SELECT user_id INTO v_rider_user_id
    FROM public.rider_profiles
    WHERE rider_id = NEW.rider_id;

    IF v_rider_user_id IS NULL THEN
        RAISE EXCEPTION 'rider profile % has no user for performance update', NEW.rider_id;
    END IF;

    INSERT INTO public.rider_performance_metrics (rider_id)
    VALUES (v_rider_user_id)
    ON CONFLICT (rider_id) DO NOTHING;

    IF NEW.status = 'accepted' AND OLD.status IS DISTINCT FROM 'accepted' THEN
        UPDATE public.rider_performance_metrics
        SET
            total_rides_accepted = total_rides_accepted + 1,
            updated_at = NOW()
        WHERE rider_id = v_rider_user_id;

    ELSIF NEW.status = 'completed' AND OLD.status IS DISTINCT FROM 'completed' THEN
        UPDATE public.rider_performance_metrics
        SET
            total_rides_completed = total_rides_completed + 1,
            updated_at = NOW()
        WHERE rider_id = v_rider_user_id;

    ELSIF NEW.status = 'cancelled' AND OLD.status NOT IN ('cancelled', 'completed') THEN
        UPDATE public.rider_performance_metrics
        SET
            total_rides_cancelled = total_rides_cancelled + 1,
            updated_at = NOW()
        WHERE rider_id = v_rider_user_id;
    END IF;

    UPDATE public.rider_performance_metrics
    SET
        acceptance_rate = CASE
            WHEN total_offers_received > 0
            THEN (total_rides_accepted::DECIMAL / total_offers_received * 100)
            ELSE 0
        END,
        completion_rate = CASE
            WHEN total_rides_accepted > 0
            THEN (total_rides_completed::DECIMAL / total_rides_accepted * 100)
            ELSE 0
        END
    WHERE rider_id = v_rider_user_id;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

INSERT INTO public.rider_wallets (rider_id, balance_cents, total_earned_cents)
SELECT rp.user_id, 0, 0
FROM public.rider_profiles rp
ON CONFLICT (rider_id) DO NOTHING;

INSERT INTO public.rider_performance_metrics (rider_id)
SELECT rp.user_id
FROM public.rider_profiles rp
ON CONFLICT (rider_id) DO NOTHING;
