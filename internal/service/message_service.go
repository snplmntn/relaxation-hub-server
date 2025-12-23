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
			// Send real-time notification to all participants
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
