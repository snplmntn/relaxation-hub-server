-- ============================================================================
-- Migration 035: Shopping Cart Schema
-- ============================================================================
-- Adds server-side cart persistence for users.

-- Each user has one active cart.
CREATE TABLE IF NOT EXISTS carts (
    cart_id SERIAL PRIMARY KEY,
    user_id INT NOT NULL UNIQUE REFERENCES users(user_id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_carts_user_id ON carts(user_id);

-- Cart items store services with preferences, similar to booking details.
CREATE TABLE IF NOT EXISTS cart_items (
    cart_item_id SERIAL PRIMARY KEY,
    cart_id INT NOT NULL REFERENCES carts(cart_id) ON DELETE CASCADE,
    service_id INT NOT NULL REFERENCES services(service_id) ON DELETE CASCADE,
    
    -- Guest and preferences
    guest_name VARCHAR(100) NOT NULL DEFAULT 'Self',
    duration_minutes INT NOT NULL DEFAULT 60,
    gender_preference VARCHAR(10) CHECK (gender_preference IN ('male', 'female', 'any')) DEFAULT 'any',
    pressure_preference VARCHAR(10) CHECK (pressure_preference IN ('soft', 'medium', 'hard')) DEFAULT 'medium',
    notes TEXT,
    
    -- Sequencing for group bookings
    sequence_number INT NOT NULL DEFAULT 0,
    start_condition VARCHAR(20) CHECK (start_condition IN ('fixed_time', 'after_previous')) DEFAULT 'fixed_time',
    
    -- Add-ons stored as JSONB array: [{"product_id": 1, "quantity": 2}, ...]
    addons JSONB DEFAULT '[]'::jsonb,
    
    date_added TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CHECK (duration_minutes > 0)
);

CREATE INDEX IF NOT EXISTS idx_cart_items_cart_id ON cart_items(cart_id);

-- Trigger to update cart's updated_at when items change
CREATE OR REPLACE FUNCTION update_cart_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE carts SET updated_at = NOW() WHERE cart_id = NEW.cart_id;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_cart_items_update_cart_timestamp
    AFTER INSERT OR UPDATE OR DELETE ON cart_items
    FOR EACH ROW
    EXECUTE FUNCTION update_cart_timestamp();
