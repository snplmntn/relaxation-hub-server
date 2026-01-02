package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	ws "github.com/snplmntn/relaxation-hub-server/internal/websocket"
)

type MessageService struct {
	repo                repository.MessageRepository
	notificationService *NotificationService
	userRepo            repository.UserRepository
	hub                 *ws.Hub
}

func NewMessageService(repo repository.MessageRepository, notificationService *NotificationService, userRepo repository.UserRepository, hub *ws.Hub) *MessageService {
	return &MessageService{
		repo:                repo,
		notificationService: notificationService,
		userRepo:            userRepo,
		hub:                 hub,
	}
}

func (s *MessageService) CreateConversation(ctx context.Context, initiatorID int64, req *model.CreateConversationRequest) (*model.ConversationResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	if len(req.ParticipantIDs) == 0 {
		return nil, fmt.Errorf("at least one participant is required")
	}

	// Make conversation creation idempotent: try to find an existing conversation
	// that contains exactly the same set of participants (including initiator).
	allParticipants := append([]int64{initiatorID}, req.ParticipantIDs...)

	// Build a set for comparison
	wantSet := make(map[int64]struct{})
	for _, id := range allParticipants {
		wantSet[id] = struct{}{}
	}

	// Fetch conversations for the initiator and inspect participants
	convs, err := s.repo.GetConversationsByUser(ctx, initiatorID)
	if err == nil {
		for _, c := range convs {
			ps, err := s.repo.GetParticipantsByConversation(ctx, c.ConversationID)
			if err != nil {
				continue
			}
			// Build participant set
			if len(ps) != len(wantSet) {
				continue
			}
			match := true
			for _, p := range ps {
				if _, ok := wantSet[p.UserID]; !ok {
					match = false
					break
				}
			}
			if match {
				return &model.ConversationResponse{
					ConversationID: c.ConversationID,
					Participants:   ps,
					CreatedAt:      c.CreatedAt,
					UpdatedAt:      c.UpdatedAt,
				}, nil
			}
		}
	}

	// No existing conversation found; create a new one and add participants
	conv := &model.Conversation{}
	if err := s.repo.CreateConversation(ctx, conv); err != nil {
		return nil, err
	}

	seen := make(map[int64]bool)
	var participants []model.ConversationParticipant

	for _, uid := range allParticipants {
		if seen[uid] {
			continue
		}
		seen[uid] = true
		p := &model.ConversationParticipant{
			ConversationID: conv.ConversationID,
			UserID:         uid,
		}
		if err := s.repo.AddParticipant(ctx, p); err != nil {
			return nil, err
		}
		participants = append(participants, *p)
	}

	return &model.ConversationResponse{
		ConversationID: conv.ConversationID,
		Participants:   participants,
		CreatedAt:      conv.CreatedAt,
		UpdatedAt:      conv.UpdatedAt,
	}, nil
}

func (s *MessageService) GetConversationsByUser(ctx context.Context, userID int64) ([]model.ConversationResponse, error) {
	return s.repo.GetConversationsWithDetails(ctx, userID)
}

func (s *MessageService) SendMessage(ctx context.Context, senderID int64, req *model.SendMessageRequest) (*model.Message, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	if req.ConversationID == 0 {
		return nil, fmt.Errorf("conversation_id is required")
	}
	msgType := strings.TrimSpace(req.MessageType)
	if msgType == "" {
		return nil, fmt.Errorf("message_type is required")
	}
	if msgType == "text" && (req.Content == nil || strings.TrimSpace(*req.Content) == "") {
		return nil, fmt.Errorf("content is required for text messages")
	}

	// Ensure the conversation exists and has participants before inserting.
	// If no participants are found, auto-create a conversation and add the sender.
	parts, err := s.repo.GetParticipantsByConversation(ctx, req.ConversationID)
	if err != nil {
		return nil, err
	}

	// If conversation is missing or has no participants, create it and add the sender.
	if len(parts) == 0 {
		conv := &model.Conversation{}
		if err := s.repo.CreateConversation(ctx, conv); err != nil {
			return nil, err
		}
		p := &model.ConversationParticipant{
			ConversationID: conv.ConversationID,
			UserID:         senderID,
		}
		if err := s.repo.AddParticipant(ctx, p); err != nil {
			return nil, err
		}
		req.ConversationID = conv.ConversationID
	}

	msg := &model.Message{
		ConversationID: req.ConversationID,
		SenderID:       senderID,
		MessageType:    msgType,
		Content:        req.Content,
		MediaURL:       req.MediaURL,
		ClientTempID:   req.ClientTempID,
	}

	if err := s.repo.SendMessage(ctx, msg); err != nil {
		return nil, err
	}

	// Broadcast message to all conversation participants via WebSocket
	participants, err := s.repo.GetParticipantsByConversation(ctx, req.ConversationID)
	if err == nil && len(participants) > 0 {
		// Broadcast to all participants, including the sender, so the client
		// receives the authoritative persisted message (id/timestamp).
		participantIDs := make([]int64, 0, len(participants))
		seen := make(map[int64]bool)
		for _, p := range participants {
			if !seen[p.UserID] {
				participantIDs = append(participantIDs, p.UserID)
				seen[p.UserID] = true
			}
		}

		if len(participantIDs) > 0 {
			// Send real-time notification to all participants using consistent event name
			s.hub.SendToUsers(participantIDs, "message:new", msg)

			// Send push notification to participants (excluding sender)
			if s.notificationService != nil && s.userRepo != nil {
				// Fetch sender info for push notification
				sender, _ := s.userRepo.FindUserByID(ctx, int(senderID))
				title := "New Message"
				senderPhoto := ""
				if sender != nil {
					name := sender.FullName
					if name == "" {
						name = "Someone"
					}
					if sender.Role == "therapist" {
						title = "Therapist " + name
					} else {
						title = name
					}
					senderPhoto = sender.ProfilePhoto
				}

				msgContent := "Sent a message"
				if msg.Content != nil {
					msgContent = *msg.Content
				} else if msg.MessageType != "text" {
					msgContent = fmt.Sprintf("Sent a %s", msg.MessageType)
				}

				for _, pid := range participantIDs {
					if pid == senderID {
						continue
					}
					// Only notify online users via WS, but push is for offline/background users.
					// We send push to everyone else - FCM/OS handles whether to show it if app is open.
					go s.notificationService.SendPushDirect(context.WithoutCancel(ctx), pid, "chat_message", title, msgContent, map[string]string{
						"conversation_id": fmt.Sprintf("%d", msg.ConversationID),
						"message_id":      fmt.Sprintf("%d", msg.MessageID),
						"name":            title, // Pass the formatted title as name
						"profile_photo":   senderPhoto,
						"user_id":         fmt.Sprintf("%d", senderID),
					})
				}
			}
		}
	}

	return msg, nil
}

func (s *MessageService) GetMessagesByConversation(ctx context.Context, conversationID int64, limit, offset int) (*model.PaginatedMessagesResponse, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	msgs, total, err := s.repo.GetMessagesByConversation(ctx, conversationID, limit, offset)
	if err != nil {
		return nil, err
	}

	var resp []model.MessageResponse
	for _, m := range msgs {
		resp = append(resp, model.MessageResponse{
			MessageID:      m.MessageID,
			ConversationID: m.ConversationID,
			SenderID:       m.SenderID,
			MessageType:    m.MessageType,
			Content:        m.Content,
			MediaURL:       m.MediaURL,
			SentAt:         m.SentAt,
			ReadAt:         m.ReadAt,
			ClientTempID:   m.ClientTempID,
		})
	}

	totalPages := (total + limit - 1) / limit
	hasMore := (offset + limit) < total
	page := (offset / limit) + 1

	return &model.PaginatedMessagesResponse{
		Messages:   resp,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
		HasMore:    hasMore,
	}, nil
}

func (s *MessageService) MarkMessageAsRead(ctx context.Context, messageID, userID int64) error {
	// Check if user is admin - admins shouldn't trigger read receipts
	role, err := s.repo.GetUserRole(ctx, userID)
	if err != nil {
		return err
	}
	if role == "admin" {
		return nil // Do nothing for admins
	}

	if err := s.repo.MarkMessageAsRead(ctx, messageID, userID); err != nil {
		return err
	}

	// Fetch message to get sender_id and conversation_id
	msg, err := s.repo.GetMessage(ctx, messageID)
	if err == nil {
		// Broadcast read receipt to the SENDER of the message (so they see it turned blue/checked)
		// SendToUser wraps data in { type: type, data: data }
		payload := map[string]interface{}{
			"message_id":      messageID,
			"conversation_id": msg.ConversationID,
			"read_at":         time.Now(),
			"read_by":         userID,
		}
		s.hub.SendToUser(msg.SenderID, "message:read", payload)
	}

	return nil
}
