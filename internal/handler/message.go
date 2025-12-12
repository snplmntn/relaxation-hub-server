package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

type MessageHandler struct {
	messageService *service.MessageService
}

func NewMessageHandler(messageService *service.MessageService) *MessageHandler {
	return &MessageHandler{messageService: messageService}
}

func (h *MessageHandler) CreateConversation(w http.ResponseWriter, r *http.Request) {
	var req model.CreateConversationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	userID, ok := middleware.GetUserID(r)
	if !ok {
		http.Error(w, "user not found in context", http.StatusUnauthorized)
		return
	}

	conv, err := h.messageService.CreateConversation(r.Context(), userID, &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(conv)
}

func (h *MessageHandler) ListConversations(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		http.Error(w, "user not found in context", http.StatusUnauthorized)
		return
	}

	convs, err := h.messageService.GetConversationsByUser(r.Context(), userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(convs)
}

func (h *MessageHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	var req model.SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	userID, ok := middleware.GetUserID(r)
	if !ok {
		http.Error(w, "user not found in context", http.StatusUnauthorized)
		return
	}

	msg, err := h.messageService.SendMessage(r.Context(), userID, &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(toMessageResponse(msg))
}

func (h *MessageHandler) GetMessages(w http.ResponseWriter, r *http.Request) {
	convIDStr := chi.URLParam(r, "conversation_id")
	convID, err := strconv.ParseInt(convIDStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid conversation_id", http.StatusBadRequest)
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := DefaultLimit
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			if l > MaxLimit {
				limit = MaxLimit
			} else {
				limit = l
			}
		}
	}

	msgs, err := h.messageService.GetMessagesByConversation(r.Context(), convID, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var resp []model.MessageResponse
	for _, m := range msgs {
		resp = append(resp, toMessageResponse(&m))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *MessageHandler) MarkMessageAsRead(w http.ResponseWriter, r *http.Request) {
	msgIDStr := chi.URLParam(r, "message_id")
	msgID, err := strconv.ParseInt(msgIDStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid message_id", http.StatusBadRequest)
		return
	}

	userID, ok := middleware.GetUserID(r)
	if !ok {
		http.Error(w, "user not found in context", http.StatusUnauthorized)
		return
	}

	if err := h.messageService.MarkMessageAsRead(r.Context(), msgID, userID); err != nil {
		if err == pgx.ErrNoRows {
			http.Error(w, "message not found or already read", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func toMessageResponse(m *model.Message) model.MessageResponse {
	return model.MessageResponse{
		MessageID:      m.MessageID,
		ConversationID: m.ConversationID,
		SenderID:       m.SenderID,
		MessageType:    m.MessageType,
		Content:        m.Content,
		MediaURL:       m.MediaURL,
		SentAt:         m.SentAt,
		ReadAt:         m.ReadAt,
	}
}
