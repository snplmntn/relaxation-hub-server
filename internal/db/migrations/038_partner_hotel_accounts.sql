-- Existing hotel contacts remain contacts until credentials are assigned.
ALTER TABLE public.users DROP CONSTRAINT IF EXISTS users_role_check;
ALTER TABLE public.users ADD CONSTRAINT users_role_check
    CHECK (role IN ('client', 'therapist', 'admin', 'super_admin', 'rider', 'hotel_admin', 'hotel_staff'));

ALTER TABLE public.partner_hotel_staff
    ADD COLUMN user_id INTEGER UNIQUE REFERENCES public.users(user_id) ON DELETE RESTRICT,
    ADD COLUMN access_role VARCHAR(20) NOT NULL DEFAULT 'hotel_staff'
        CHECK (access_role IN ('hotel_admin', 'hotel_staff'));
