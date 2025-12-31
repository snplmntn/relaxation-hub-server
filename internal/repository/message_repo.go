package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

// MessageRepository manages conversations and messages.
type MessageRepository interface {
	CreateConversation(ctx context.Context, conv *model.Conversation) error
	AddParticipant(ctx context.Context, p *model.ConversationParticipant) error
	GetConversationsByUser(ctx context.Context, userID int64) ([]model.Conversation, error)
	GetParticipantsByConversation(ctx context.Context, conversationID int64) ([]model.ConversationParticipant, error)
	SendMessage(ctx context.Context, msg *model.Message) error
	GetMessagesByConversation(ctx context.Context, conversationID int64, limit int) ([]model.Message, error)
	MarkMessageAsRead(ctx context.Context, messageID, userID int64) error
}

type messageRepoImpl struct {
	db db.DBTX
}

func NewMessageRepository(db db.DBTX) MessageRepository {
	return &messageRepoImpl{db: db}
}

func (r *messageRepoImpl) CreateConversation(ctx context.Context, conv *model.Conversation) error {
	query := `
        INSERT INTO conversations DEFAULT VALUES
        RETURNING conversation_id, created_at, updated_at
    `
	return r.db.QueryRow(ctx, query).Scan(&conv.ConversationID, &conv.CreatedAt, &conv.UpdatedAt)
}

func (r *messageRepoImpl) AddParticipant(ctx context.Context, p *model.ConversationParticipant) error {
	query := `
		INSERT INTO conversation_participants (conversation_id, user_id)
		VALUES ($1,$2)
		RETURNING joined_at
    `
	return r.db.QueryRow(ctx, query, p.ConversationID, p.UserID).Scan(&p.JoinedAt)
}

func (r *messageRepoImpl) GetConversationsByUser(ctx context.Context, userID int64) ([]model.Conversation, error) {
	query := `
        SELECT DISTINCT c.conversation_id, c.created_at, c.updated_at
        FROM conversations c
        JOIN conversation_participants cp ON c.conversation_id = cp.conversation_id
        WHERE cp.user_id = $1
        ORDER BY c.updated_at DESC
    `
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var convs []model.Conversation
	for rows.Next() {
		var c model.Conversation
		if err := rows.Scan(&c.ConversationID, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		convs = append(convs, c)
	}
	return convs, rows.Err()
}

func (r *messageRepoImpl) GetParticipantsByConversation(ctx context.Context, conversationID int64) ([]model.ConversationParticipant, error) {
	query := `
		SELECT conversation_id, user_id, joined_at
        FROM conversation_participants
        WHERE conversation_id = $1
    `
	rows, err := r.db.Query(ctx, query, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ps []model.ConversationParticipant
	for rows.Next() {
		var p model.ConversationParticipant
		if err := rows.Scan(&p.ConversationID, &p.UserID, &p.JoinedAt); err != nil {
			return nil, err
		}
		ps = append(ps, p)
	}
	return ps, rows.Err()
}

func (r *messageRepoImpl) SendMessage(ctx context.Context, msg *model.Message) error {
	query := `
        INSERT INTO messages (conversation_id, sender_id, message_type, content, media_url)
        VALUES ($1,$2,$3,$4,$5)
        RETURNING message_id, sent_at
    `
	return r.db.QueryRow(ctx, query,
		msg.ConversationID,
		msg.SenderID,
		msg.MessageType,
		msg.Content,
		msg.MediaURL,
	).Scan(&msg.MessageID, &msg.SentAt)
}

func (r *messageRepoImpl) GetMessagesByConversation(ctx context.Context, conversationID int64, limit int) ([]model.Message, error) {
	query := `
        SELECT message_id, conversation_id, sender_id, message_type, content, media_url, sent_at, read_at
        FROM messages
        WHERE conversation_id = $1 AND deleted_at IS NULL
        ORDER BY sent_at DESC
        LIMIT $2
    `
	rows, err := r.db.Query(ctx, query, conversationID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []model.Message
	for rows.Next() {
		var m model.Message
		if err := rows.Scan(&m.MessageID, &m.ConversationID, &m.SenderID, &m.MessageType, &m.Content, &m.MediaURL, &m.SentAt, &m.ReadAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

func (r *messageRepoImpl) MarkMessageAsRead(ctx context.Context, messageID, userID int64) error {
	cmd, err := r.db.Exec(ctx, `
        UPDATE messages
        SET read_at = CURRENT_TIMESTAMP
        WHERE message_id = $1
          AND sender_id != $2
          AND read_at IS NULL
    `, messageID, userID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
