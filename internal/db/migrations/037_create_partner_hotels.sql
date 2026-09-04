CREATE TABLE IF NOT EXISTS public.partner_hotels (
    partner_hotel_id BIGSERIAL PRIMARY KEY,
    hotel_name VARCHAR(160) NOT NULL,
    address_line VARCHAR(255) NOT NULL DEFAULT '',
    city VARCHAR(120) NOT NULL DEFAULT '',
    contact_person VARCHAR(160) NOT NULL DEFAULT '',
    email VARCHAR(255) NOT NULL DEFAULT '',
    phone VARCHAR(40) NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT partner_hotels_name_not_blank CHECK (btrim(hotel_name) <> '')
);

CREATE INDEX IF NOT EXISTS idx_partner_hotels_active_name
    ON public.partner_hotels (is_active, lower(hotel_name));

CREATE TABLE IF NOT EXISTS public.partner_hotel_staff (
    partner_hotel_staff_id BIGSERIAL PRIMARY KEY,
    partner_hotel_id BIGINT NOT NULL
        REFERENCES public.partner_hotels(partner_hotel_id) ON DELETE CASCADE,
    full_name VARCHAR(160) NOT NULL,
    position VARCHAR(120) NOT NULL DEFAULT '',
    email VARCHAR(255) NOT NULL DEFAULT '',
    phone VARCHAR(40) NOT NULL DEFAULT '',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT partner_hotel_staff_name_not_blank CHECK (btrim(full_name) <> '')
);

CREATE INDEX IF NOT EXISTS idx_partner_hotel_staff_hotel_active_name
    ON public.partner_hotel_staff (partner_hotel_id, is_active, lower(full_name));
