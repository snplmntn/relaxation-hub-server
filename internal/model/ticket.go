package model

import "time"

type SupportTicket struct {
	TicketID            int64                      `json:"ticket_id"`
	UserID              *int64                     `json:"user_id"`
	FullName            string                     `json:"full_name"`
	ConnectedEmailPhone string                     `json:"connected_email_phone"`
	ContactEmailPhone   string                     `json:"contact_email_phone"`
	Category            string                     `json:"category"`
	BookingID           *int64                     `json:"booking_id,omitempty"`
	Description         string                     `json:"description"`
	Status              string                     `json:"status"`
	CreatedAt           time.Time                  `json:"created_at"`
	UpdatedAt           time.Time                  `json:"updated_at"`
	Attachments         []SupportTicketAttachment  `json:"attachments,omitempty"`
}

type SupportTicketAttachment struct {
	AttachmentID int64     `json:"attachment_id"`
	TicketID     int64     `json:"ticket_id"`
	FileURL      string    `json:"file_url"`
	FileType     string    `json:"file_type"`
	UploadedAt   time.Time `json:"uploaded_at"`
}

type CreateSupportTicketRequest struct {
	BookingID         *int64   `json:"booking_id"` // Optional
	Category          string   `json:"category"`
	Description       string   `json:"description"`
	ContactEmailPhone string   `json:"contact_email_phone"`
	// Attachments are handled via multipart form, but we might decode metadata here if needed
}
