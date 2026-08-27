-- Avoid creating zero-value rider transactions when a completed ride has no
-- calculated rider earnings. rider_transactions requires non-zero amounts.

CREATE OR REPLACE FUNCTION update_rider_wallet_on_earning()
RETURNS TRIGGER AS $$
DECLARE
    v_rider_user_id BIGINT;
BEGIN
    IF NEW.rider_earnings_cents IS NOT NULL
       AND NEW.rider_earnings_cents > 0
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
