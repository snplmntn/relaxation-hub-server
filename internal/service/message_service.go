package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	ws "github.com/snplmntn/relaxation-hub-server/internal/websocket"
)

type MessageService struct {
	repo repository.MessageRepository
	hub  *ws.Hub
}

func NewMessageService(repo repository.MessageRepository, hub *ws.Hub) *MessageService {
	return &MessageService{repo: repo, hub: hub}
}

func (s *MessageService) CreateConversation(ctx context.Context, initiatorID int64, req *model.CreateConversationRequest) (*model.ConversationResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	if len(req.ParticipantIDs) == 0 {
		return nil, fmt.Errorf("at least one participant is required")
	}

	conv := &model.Conversation{}
	if err := s.repo.CreateConversation(ctx, conv); err != nil {
		return nil, err
	}

	allParticipants := append([]int64{initiatorID}, req.ParticipantIDs...)
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
	convs, err := s.repo.GetConversationsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	var resp []model.ConversationResponse
	for _, c := range convs {
		ps, err := s.repo.GetParticipantsByConversation(ctx, c.ConversationID)
		if err != nil {
			return nil, err
		}
		resp = append(resp, model.ConversationResponse{
			ConversationID: c.ConversationID,
			Participants:   ps,
			CreatedAt:      c.CreatedAt,
			UpdatedAt:      c.UpdatedAt,
		})
	}
	return resp, nil
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

	msg := &model.Message{
		ConversationID: req.ConversationID,
		SenderID:       senderID,
		MessageType:    msgType,
		Content:        req.Content,
		MediaURL:       req.MediaURL,
	}

	if err := s.repo.SendMessage(ctx, msg); err != nil {
		return nil, err
	}

	// Broadcast message to all conversation participants via WebSocket
	participants, err := s.repo.GetParticipantsByConversation(ctx, req.ConversationID)
	if err == nil && len(participants) > 0 {
		participantIDs := make([]int64, 0, len(participants))
		for _, p := range participants {
			if p.UserID != senderID { // Don't send to sender
				participantIDs = append(participantIDs, p.UserID)
			}
		}

		if len(participantIDs) > 0 {
			// Send real-time notification
			s.hub.SendToUsers(participantIDs, "new_message", msg)
		}
	}

	return msg, nil
}

func (s *MessageService) GetMessagesByConversation(ctx context.Context, conversationID int64, limit int) ([]model.Message, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.repo.GetMessagesByConversation(ctx, conversationID, limit)
}

func (s *MessageService) MarkMessageAsRead(ctx context.Context, messageID, userID int64) error {
	return s.repo.MarkMessageAsRead(ctx, messageID, userID)
}
