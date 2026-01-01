BEGIN;

-- Defines the support tickets submitted by users
CREATE TABLE support_tickets (
    ticket_id SERIAL PRIMARY KEY,
    user_id INT REFERENCES users(user_id) ON DELETE SET NULL, -- Nullable if user deletes account but we keep ticket
    
    -- Contact Information
    full_name VARCHAR(150),
    connected_email_phone VARCHAR(150), -- Snapshot of profile info at time of creation
    contact_email_phone VARCHAR(150),   -- User provided contact info
    
    -- Ticket Details
    category VARCHAR(50) NOT NULL CHECK (category IN (
        'Booking Issue',
        'Payment & Billing Issue',
        'Safety & Conduct Report',
        'Technical Issue (App Bug)',
        'Account & Profile Support',
        'General Inquiry & Feedback',
        'Other'
    )),
    
    booking_id INT REFERENCES bookings(booking_id) ON DELETE SET NULL, -- Conditional field
    description TEXT NOT NULL,
    
    status VARCHAR(20) NOT NULL DEFAULT 'pending' 
        CHECK (status IN ('pending', 'investigating', 'resolved', 'closed')),
        
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for efficient lookup
CREATE INDEX idx_support_tickets_user ON support_tickets(user_id);
CREATE INDEX idx_support_tickets_status ON support_tickets(status);
CREATE INDEX idx_support_tickets_booking ON support_tickets(booking_id);
CREATE INDEX idx_support_tickets_created_at ON support_tickets(created_at DESC);

-- Attachments for tickets (Images/Screenshots)
CREATE TABLE support_ticket_attachments (
    attachment_id SERIAL PRIMARY KEY,
    ticket_id INT REFERENCES support_tickets(ticket_id) ON DELETE CASCADE,
    file_url TEXT NOT NULL,
    file_type VARCHAR(50) DEFAULT 'image',
    uploaded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_support_ticket_attachments_ticket ON support_ticket_attachments(ticket_id);

-- Trigger for updated_at
CREATE TRIGGER update_support_tickets_updated_at
    BEFORE UPDATE ON support_tickets
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

COMMIT;
