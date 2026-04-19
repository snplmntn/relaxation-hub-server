ALTER TABLE promotions
ADD COLUMN IF NOT EXISTS applies_to TEXT NOT NULL DEFAULT 'full_basket';

DO $$
BEGIN
	IF NOT EXISTS (
		SELECT 1
		FROM pg_constraint
		WHERE conname = 'promotions_applies_to_check'
	) THEN
		ALTER TABLE promotions
		ADD CONSTRAINT promotions_applies_to_check
		CHECK (applies_to IN ('full_basket', 'services_only'));
	END IF;
END $$;
