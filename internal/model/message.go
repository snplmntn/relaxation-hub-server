package model

import "time"

// Conversation represents the conversations table.
type Conversation struct {
	ConversationID int64     `db:"conversation_id" json:"conversation_id"`
	CreatedAt      time.Time `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time `db:"updated_at" json:"updated_at"`
}

// ConversationParticipant represents the conversation_participants table.
type ConversationParticipant struct {
	ParticipantID  int64     `db:"participant_id" json:"participant_id"`
	ConversationID int64     `db:"conversation_id" json:"conversation_id"`
	UserID         int64     `db:"user_id" json:"user_id"`
	JoinedAt       time.Time `db:"joined_at" json:"joined_at"`
	// Enriched fields (not from DB, populated by service layer)
	FullName        string  `json:"full_name,omitempty"`
	Email           string  `json:"email,omitempty"`
	ProfilePhoto    string  `json:"profile_photo,omitempty"`
	Role            string  `json:"role,omitempty"`
	LastServiceName string  `json:"last_service_name,omitempty"`
	Rating          float64 `json:"rating,omitempty"`
}

// Message represents the messages table.
type Message struct {
	MessageID      int64      `db:"message_id" json:"message_id"`
	ConversationID int64      `db:"conversation_id" json:"conversation_id"`
	SenderID       int64      `db:"sender_id" json:"sender_id"`
	MessageType    string     `db:"message_type" json:"message_type"`
	Content        *string    `db:"content" json:"content,omitempty"`
	MediaURL       *string    `db:"media_url" json:"media_url,omitempty"`
	SentAt         time.Time  `db:"sent_at" json:"sent_at"`
	ReadAt         *time.Time `db:"read_at" json:"read_at,omitempty"`
	DeletedAt      *time.Time `db:"deleted_at" json:"deleted_at,omitempty"`
	ClientTempID   string     `db:"-" json:"client_temp_id,omitempty"` // Transient, not stored
}

// CreateConversationRequest for starting conversation.
type CreateConversationRequest struct {
	ParticipantIDs []int64 `json:"participant_ids"`
}

// SendMessageRequest for sending message.
type SendMessageRequest struct {
	ConversationID int64   `json:"conversation_id"`
	MessageType    string  `json:"message_type"`
	Content        *string `json:"content"`
	MediaURL       *string `json:"media_url"`
	ClientTempID   string  `json:"client_temp_id"`
}

// ConversationResponse to clients.
type ConversationResponse struct {
	ConversationID int64                     `json:"conversation_id"`
	Participants   []ConversationParticipant `json:"participants"`
	CreatedAt      time.Time                 `json:"created_at"`
	UpdatedAt      time.Time                 `json:"updated_at"`
}

// MessageResponse to clients.
type MessageResponse struct {
	MessageID      int64      `json:"message_id"`
	ConversationID int64      `json:"conversation_id"`
	SenderID       int64      `json:"sender_id"`
	MessageType    string     `json:"message_type"`
	Content        *string    `json:"content,omitempty"`
	MediaURL       *string    `json:"media_url,omitempty"`
	SentAt         time.Time  `json:"sent_at"`
	ReadAt         *time.Time `json:"read_at,omitempty"`
	ClientTempID   string     `json:"client_temp_id,omitempty"`
}

// PaginatedMessagesResponse wraps a list of messages with pagination metadata.
type PaginatedMessagesResponse struct {
	Messages   []MessageResponse `json:"messages"`
	Total      int               `json:"total"`
	Page       int               `json:"page"`
	Limit      int               `json:"limit"`
	TotalPages int               `json:"total_pages"`
	HasMore    bool              `json:"has_more"`
}

// PaginatedConversationsResponse wraps a list of conversations with pagination metadata.
type PaginatedConversationsResponse struct {
	Conversations []ConversationResponse `json:"conversations"`
	Total         int                    `json:"total"`
	Page          int                    `json:"page"`
	Limit         int                    `json:"limit"`
	TotalPages    int                    `json:"total_pages"`
	HasMore       bool                   `json:"has_more"`
}
