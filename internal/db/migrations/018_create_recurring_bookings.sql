-- Forward migration: recurring booking series support

CREATE TABLE IF NOT EXISTS public.recurring_bookings (
    recurring_id     BIGSERIAL PRIMARY KEY,
    client_id        BIGINT NOT NULL REFERENCES public.users(user_id) ON DELETE CASCADE,
    created_by       BIGINT REFERENCES public.users(user_id) ON DELETE SET NULL,
    service_id       BIGINT REFERENCES public.services(service_id) ON DELETE SET NULL,
    address_id       BIGINT REFERENCES public.addresses(address_id) ON DELETE SET NULL,
    therapist_id     BIGINT REFERENCES public.users(user_id) ON DELETE SET NULL,
    duration_minutes INT NOT NULL DEFAULT 60,
    -- preference snapshot
    gender_preference  TEXT NOT NULL DEFAULT 'any',
    pressure_preference TEXT NOT NULL DEFAULT 'medium',
    notes            TEXT NOT NULL DEFAULT '',
    payment_method   TEXT NOT NULL DEFAULT 'cash',
    -- schedule
    frequency        TEXT NOT NULL CHECK (frequency IN ('daily', 'weekly', 'monthly')),
    interval_value   SMALLINT NOT NULL DEFAULT 1 CHECK (interval_value >= 1),
    days_of_week     SMALLINT[] NOT NULL DEFAULT '{}',   -- 0=Sun … 6=Sat, used for weekly
    day_of_month     SMALLINT,                            -- 1–31, used for monthly
    time_of_day      TIME NOT NULL,
    start_date       DATE NOT NULL,
    end_date         DATE,                                -- NULL = open-ended
    -- lifecycle
    status           TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'paused', 'cancelled')),
    generated_until  TIMESTAMPTZ,                         -- high-water mark for rolling generation
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_recurring_bookings_client
    ON public.recurring_bookings (client_id);

CREATE INDEX IF NOT EXISTS idx_recurring_bookings_status
    ON public.recurring_bookings (status) WHERE status = 'active';

-- Add recurring_id to bookings
ALTER TABLE public.bookings
    ADD COLUMN IF NOT EXISTS recurring_id BIGINT REFERENCES public.recurring_bookings(recurring_id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_bookings_recurring
    ON public.bookings (recurring_id) WHERE recurring_id IS NOT NULL;

-- Idempotency guard: prevent double-creating the same occurrence
CREATE UNIQUE INDEX IF NOT EXISTS uidx_bookings_recurring_start
    ON public.bookings (recurring_id, scheduled_start)
    WHERE recurring_id IS NOT NULL;
