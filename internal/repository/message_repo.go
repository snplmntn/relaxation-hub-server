package repository

import (
	"context"
	"time"

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
	GetMessagesByConversation(ctx context.Context, conversationID int64, limit, offset int) ([]model.Message, int, error)
	MarkMessageAsRead(ctx context.Context, messageID, userID int64) error
	GetMessage(ctx context.Context, messageID int64) (*model.Message, error)
	GetConversationsWithDetails(ctx context.Context, userID int64) ([]model.ConversationResponse, error)
	GetAllConversationsWithDetails(ctx context.Context, limit, offset int) ([]model.ConversationResponse, int, error)
	GetUserRole(ctx context.Context, userID int64) (string, error)
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

func (r *messageRepoImpl) GetConversationsWithDetails(ctx context.Context, userID int64) ([]model.ConversationResponse, error) {
	query := `
		SELECT 
			c.conversation_id,
			c.created_at,
			c.updated_at,
			cp.user_id,
			cp.joined_at,
			COALESCE(u.full_name, ''),
			COALESCE(u.primary_email, ''),
			COALESCE(u.role, ''),
			COALESCE(u.profile_photo, ''),
			COALESCE(tp.avg_rating, 0),
			(
				SELECT s.name
				FROM bookings b
				JOIN services s ON b.service_id = s.service_id
				WHERE (b.client_id = $1 AND b.therapist_id = cp.user_id)
				   OR (b.client_id = cp.user_id AND b.therapist_id = $1)
				ORDER BY b.created_at DESC
				LIMIT 1
			) as last_service_name
		FROM conversations c
		JOIN conversation_participants my_cp ON c.conversation_id = my_cp.conversation_id
		JOIN conversation_participants cp ON c.conversation_id = cp.conversation_id
		JOIN users u ON cp.user_id = u.user_id
		LEFT JOIN therapist_profiles tp ON u.user_id = tp.therapist_id
		WHERE my_cp.user_id = $1
		ORDER BY c.updated_at DESC, c.conversation_id
	`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Use a map to group participants by conversation
	convMap := make(map[int64]*model.ConversationResponse)
	// To preserve order
	var convOrder []int64

	for rows.Next() {
		var (
			convID       int64
			createdAt    time.Time
			updatedAt    time.Time
			pUserID      int64
			pJoinedAt    time.Time
			pFullName    string
			pEmail       string
			pRole        string
			pPhoto       string
			pRating      float64
			pLastService *string
		)
		err := rows.Scan(
			&convID, &createdAt, &updatedAt,
			&pUserID, &pJoinedAt,
			&pFullName, &pEmail, &pRole, &pPhoto, &pRating, &pLastService,
		)
		if err != nil {
			return nil, err
		}

		if _, exists := convMap[convID]; !exists {
			convMap[convID] = &model.ConversationResponse{
				ConversationID: convID,
				CreatedAt:      createdAt,
				UpdatedAt:      updatedAt,
				Participants:   []model.ConversationParticipant{},
			}
			convOrder = append(convOrder, convID)
		}

		p := model.ConversationParticipant{
			ConversationID:  convID,
			UserID:          pUserID,
			JoinedAt:        pJoinedAt,
			FullName:        pFullName,
			Email:           pEmail,
			Role:            pRole,
			ProfilePhoto:    pPhoto,
			Rating:          pRating,
			LastServiceName: "",
		}
		if pLastService != nil {
			p.LastServiceName = *pLastService
		}
		convMap[convID].Participants = append(convMap[convID].Participants, p)
	}

	result := make([]model.ConversationResponse, 0, len(convOrder))
	for _, id := range convOrder {
		result = append(result, *convMap[id])
	}

	return result, rows.Err()
}

// GetAllConversationsWithDetails returns all conversations with all participants (admin view).
func (r *messageRepoImpl) GetAllConversationsWithDetails(ctx context.Context, limit, offset int) ([]model.ConversationResponse, int, error) {
	// 1. Get total count
	var total int
	err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM conversations").Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	// 2. Fetch paginated conversations with details using a CTE to paginate by conversation first
	query := `
		WITH paged_conversations AS (
			SELECT conversation_id, created_at, updated_at
			FROM conversations
			ORDER BY updated_at DESC
			LIMIT $1 OFFSET $2
		)
		SELECT
			c.conversation_id,
			c.created_at,
			c.updated_at,
			cp.user_id,
			cp.joined_at,
			COALESCE(u.full_name, ''),
			COALESCE(u.primary_email, ''),
			COALESCE(u.role, ''),
			COALESCE(u.profile_photo, ''),
			COALESCE(tp.avg_rating, 0)
		FROM paged_conversations c
		JOIN conversation_participants cp ON c.conversation_id = cp.conversation_id
		JOIN users u ON cp.user_id = u.user_id
		LEFT JOIN therapist_profiles tp ON u.user_id = tp.therapist_id
		ORDER BY c.updated_at DESC, c.conversation_id
	`
	rows, err := r.db.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	convMap := make(map[int64]*model.ConversationResponse)
	var convOrder []int64

	for rows.Next() {
		var (
			convID    int64
			createdAt time.Time
			updatedAt time.Time
			pUserID   int64
			pJoinedAt time.Time
			pFullName string
			pEmail    string
			pRole     string
			pPhoto    string
			pRating   float64
		)
		if err := rows.Scan(
			&convID, &createdAt, &updatedAt,
			&pUserID, &pJoinedAt,
			&pFullName, &pEmail, &pRole, &pPhoto, &pRating,
		); err != nil {
			return nil, 0, err
		}
		if _, exists := convMap[convID]; !exists {
			convMap[convID] = &model.ConversationResponse{
				ConversationID: convID,
				CreatedAt:      createdAt,
				UpdatedAt:      updatedAt,
				Participants:   []model.ConversationParticipant{},
			}
			convOrder = append(convOrder, convID)
		}
		convMap[convID].Participants = append(convMap[convID].Participants, model.ConversationParticipant{
			ConversationID: convID,
			UserID:         pUserID,
			JoinedAt:       pJoinedAt,
			FullName:       pFullName,
			Email:          pEmail,
			Role:           pRole,
			ProfilePhoto:   pPhoto,
			Rating:         pRating,
		})
	}

	result := make([]model.ConversationResponse, 0, len(convOrder))
	for _, id := range convOrder {
		result = append(result, *convMap[id])
	}
	return result, total, rows.Err()
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
	var senderID interface{} = msg.SenderID
	if msg.SenderID == 0 {
		senderID = nil
	}
	return r.db.QueryRow(ctx, query,
		msg.ConversationID,
		senderID,
		msg.MessageType,
		msg.Content,
		msg.MediaURL,
	).Scan(&msg.MessageID, &msg.SentAt)
}

func (r *messageRepoImpl) GetMessagesByConversation(ctx context.Context, conversationID int64, limit, offset int) ([]model.Message, int, error) {
	// 1. Get total count
	var total int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM messages WHERE conversation_id = $1 AND deleted_at IS NULL`, conversationID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// 2. Get paginated messages
	query := `
        SELECT message_id, conversation_id, sender_id, message_type, content, media_url, sent_at, read_at
        FROM messages
        WHERE conversation_id = $1 AND deleted_at IS NULL
        ORDER BY sent_at DESC
        LIMIT $2 OFFSET $3
    `
	rows, err := r.db.Query(ctx, query, conversationID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var msgs []model.Message
	for rows.Next() {
		var m model.Message
		var senderID *int64
		if err := rows.Scan(&m.MessageID, &m.ConversationID, &senderID, &m.MessageType, &m.Content, &m.MediaURL, &m.SentAt, &m.ReadAt); err != nil {
			return nil, 0, err
		}
		if senderID != nil {
			m.SenderID = *senderID
		}
		msgs = append(msgs, m)
	}
	return msgs, total, rows.Err()
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

func (r *messageRepoImpl) GetMessage(ctx context.Context, messageID int64) (*model.Message, error) {
	query := `
        SELECT message_id, conversation_id, sender_id, message_type, content, media_url, sent_at, read_at
        FROM messages
        WHERE message_id = $1
    `
	var m model.Message
	var senderID *int64
	err := r.db.QueryRow(ctx, query, messageID).Scan(
		&m.MessageID, &m.ConversationID, &senderID, &m.MessageType, &m.Content, &m.MediaURL, &m.SentAt, &m.ReadAt,
	)
	if err != nil {
		return nil, err
	}
	if senderID != nil {
		m.SenderID = *senderID
	}
	return &m, nil
}

func (r *messageRepoImpl) GetUserRole(ctx context.Context, userID int64) (string, error) {
	var role string
	err := r.db.QueryRow(ctx, "SELECT role FROM users WHERE user_id = $1", userID).Scan(&role)
	return role, err
}
