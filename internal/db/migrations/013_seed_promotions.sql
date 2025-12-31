-- Seed test promotions
INSERT INTO promotions (code, description, discount_amount, discount_percentage, valid_from, valid_until, max_uses, current_uses, days_of_week)
VALUES 
('WELCOME50', 'Get 50 PHP off your booking', 50.00, NULL, NOW() - INTERVAL '1 day', NOW() + INTERVAL '1 year', 1000, 0, NULL),
('SUMMER10', 'Get 10% off your booking', NULL, 10, NOW() - INTERVAL '1 day', NOW() + INTERVAL '1 year', 1000, 0, NULL)
ON CONFLICT (code) DO NOTHING;
