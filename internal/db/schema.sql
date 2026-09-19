CREATE TABLE services (
    service_id SERIAL PRIMARY KEY,
    name VARCHAR(150) NOT NULL,
    description TEXT,
    category VARCHAR(50),
    preview_image_url TEXT,
    base_price NUMERIC(10,2) NOT NULL,
    therapist_commission NUMERIC(10,2) DEFAULT 0,
    duration_minutes INT NOT NULL DEFAULT 60,
    is_active BOOLEAN DEFAULT TRUE,
    deleted_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    subtitle TEXT,
    is_featured BOOLEAN NOT NULL DEFAULT FALSE,
    featured_order INT NOT NULL DEFAULT 0,
    CHECK (base_price >= 0),
    CHECK (duration_minutes > 0)
);

CREATE INDEX idx_services_active ON services(service_id) WHERE deleted_at IS NULL;

CREATE TABLE landing_settings (
    id INT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    phone_globe TEXT NOT NULL DEFAULT '',
    phone_smart TEXT NOT NULL DEFAULT '',
    email TEXT NOT NULL DEFAULT '',
    address TEXT NOT NULL DEFAULT '',
    hours TEXT NOT NULL DEFAULT '',
    facebook_url TEXT NOT NULL DEFAULT '',
    instagram_url TEXT NOT NULL DEFAULT '',
    whatsapp_url TEXT NOT NULL DEFAULT '',
    viber_url TEXT NOT NULL DEFAULT '',
    application_link TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE bookings (
    booking_id SERIAL PRIMARY KEY,
    client_id INT,
    service_id INT REFERENCES services(service_id) ON DELETE SET NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (status IN (
            'pending', 'assigned', 'on_the_way', 'arrived', 'in_progress', 'completed', 'cancelled',
            'cancelled_by_therapist', 'cancelled_by_client', 'no_show', 'rescheduled'
        )),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_bookings_client ON bookings(client_id);
CREATE INDEX idx_bookings_status ON bookings(status);
CREATE INDEX idx_bookings_created_at ON bookings(created_at DESC);
