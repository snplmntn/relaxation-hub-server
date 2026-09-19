-- Vouchers are internal until staff say otherwise.
--
-- /promotions/active returned every in-date code to any signed-in user, so
-- partner and VIP codes (PARTNERHOTEL is 100% off) were listed on the customer
-- Offers page and could be redeemed by anyone who read them. There was no
-- column expressing "customers may see and use this", so the safe default for
-- every existing row is FALSE: staff opt a code in per campaign.
ALTER TABLE promotions
    ADD COLUMN IF NOT EXISTS is_public BOOLEAN NOT NULL DEFAULT FALSE;
