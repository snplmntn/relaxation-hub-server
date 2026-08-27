-- Forward migration for databases where 001_init.sql was already applied
-- before day-view therapist ordering was added to the consolidated schema.

CREATE TABLE IF NOT EXISTS public.day_view_therapist_orders (
    order_id BIGSERIAL PRIMARY KEY,
    view_key TEXT NOT NULL,
    business_date DATE NOT NULL,
    therapist_ids BIGINT[] NOT NULL DEFAULT '{}',
    source TEXT NOT NULL CHECK (source IN ('auto', 'manual')),
    updated_by_admin_id BIGINT REFERENCES public.users(user_id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (view_key, business_date)
);

CREATE INDEX IF NOT EXISTS idx_day_view_therapist_orders_view_date
    ON public.day_view_therapist_orders (view_key, business_date DESC);
