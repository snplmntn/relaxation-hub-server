-- The consolidated schema can leave both trg_update_rider_wallet and
-- trigger_update_rider_wallet attached to rides. Both call
-- update_rider_wallet_on_earning(), so ride completion can double-credit rider
-- earnings. Keep the canonical trigger_update_rider_wallet trigger.

DROP TRIGGER IF EXISTS trg_update_rider_wallet ON public.rides;
