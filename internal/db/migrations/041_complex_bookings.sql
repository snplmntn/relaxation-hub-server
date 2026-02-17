-- Migration: 033_complex_bookings.sql
-- Purpose: Add support for multi-service booking groups, add-on products, and booking relationships.

-- =============================================================================
-- 1. NEW TABLE: products (Add-ons catalog)
-- =============================================================================
CREATE TABLE IF NOT EXISTS products (
    product_id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    price NUMERIC(10,2) NOT NULL DEFAULT 0,      -- Selling price (what customer pays)
    cost NUMERIC(10,2) NOT NULL DEFAULT 0,       -- Business cost (for profit margin)
    image_url TEXT,
    category VARCHAR(50) DEFAULT 'add_on', -- e.g., 'add_on', 'linen', 'wellness'
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- =============================================================================
-- 2. NEW TABLE: booking_groups (Container for multiple related bookings)
-- =============================================================================
CREATE TABLE IF NOT EXISTS booking_groups (
    group_id SERIAL PRIMARY KEY,
    client_id INT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    address_id INT REFERENCES addresses(address_id) ON DELETE SET NULL,
    scheduled_start TIMESTAMPTZ,
    raw_total NUMERIC(10,2) DEFAULT 0,
    discount NUMERIC(10,2) DEFAULT 0,
    final_total NUMERIC(10,2) DEFAULT 0,
    payment_method VARCHAR(20) CHECK (payment_method IN ('cash', 'gcash', 'bdo')),
    status VARCHAR(30) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'assigned', 'in_progress', 'completed', 'cancelled', 'paid')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for quick lookup by client
CREATE INDEX IF NOT EXISTS idx_booking_groups_client_id ON booking_groups(client_id);

-- =============================================================================
-- 3. ALTER TABLE: bookings (Add group relationship and sequencing)
-- =============================================================================
ALTER TABLE bookings
    ADD COLUMN IF NOT EXISTS group_id INT REFERENCES booking_groups(group_id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS guest_name VARCHAR(100),
    ADD COLUMN IF NOT EXISTS sequence_number INT DEFAULT 0,
    ADD COLUMN IF NOT EXISTS start_condition VARCHAR(30) DEFAULT 'fixed_time'
        CHECK (start_condition IN ('fixed_time', 'after_previous'));

-- Index for group lookups
CREATE INDEX IF NOT EXISTS idx_bookings_group_id ON bookings(group_id);

-- =============================================================================
-- 4. NEW TABLE: booking_addons (Links products to bookings)
-- =============================================================================
CREATE TABLE IF NOT EXISTS booking_addons (
    addon_id SERIAL PRIMARY KEY,
    booking_id INT NOT NULL REFERENCES bookings(booking_id) ON DELETE CASCADE,
    product_id INT NOT NULL REFERENCES products(product_id) ON DELETE RESTRICT,
    quantity INT NOT NULL DEFAULT 1 CHECK (quantity > 0),
    price_at_booking NUMERIC(10,2) NOT NULL, -- Snapshot of price at time of booking
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for fetching addons by booking
CREATE INDEX IF NOT EXISTS idx_booking_addons_booking_id ON booking_addons(booking_id);

-- =============================================================================
-- 5. SEED: Sample products (can be removed in production)
-- =============================================================================
INSERT INTO products (name, description, price, category) VALUES
    ('Premium Massage Oil', 'Lavender-scented premium oil', 150.00, 'add_on'),
    ('Bed Linen Set', 'Fresh linens for the session', 100.00, 'linen'),
    ('Vicks Vaporub', 'Soothing menthol rub', 80.00, 'wellness'),
    ('Extra Towel', 'Additional towel for the session', 50.00, 'linen')
ON CONFLICT DO NOTHING;
