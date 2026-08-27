-- Single-row table holding the public landing page contact / social / application
-- info that the Super Admin can edit. The CHECK keeps it to exactly one row (id = 1).
CREATE TABLE IF NOT EXISTS landing_settings (
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

-- Seed the single row with the values previously hardcoded on the landing page so
-- the live site is unchanged until an admin edits them.
INSERT INTO landing_settings (
    id, phone_globe, phone_smart, email, address, hours,
    facebook_url, instagram_url, whatsapp_url, viber_url, application_link
) VALUES (
    1,
    '0917-568-8383',
    '0917-324-8686',
    'hello@bookhiraya.com',
    '#50 Esteban Abada St., Brgy. Loyola Heights, Quezon City.',
    'Open daily from 3 PM to 12 MN, last call.',
    'https://www.facebook.com/hirayahomespa',
    'https://www.instagram.com/hirayahomespa?utm_source=ig_web_button_share_sheet&igsh=ZDNlZDc0MzIxNw==',
    'https://wa.me/639175688383',
    'viber://chat/?number=%2B639175688383',
    '/our-team'
)
ON CONFLICT (id) DO NOTHING;
