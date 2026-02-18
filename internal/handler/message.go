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
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	conv, err := h.messageService.CreateConversation(r.Context(), userID, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(conv)
}

func (h *MessageHandler) ListConversations(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	convs, err := h.messageService.GetConversationsByUser(r.Context(), userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(convs)
}

// ListAllConversations returns all conversations for admin.
func (h *MessageHandler) ListAllConversations(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			if l > 100 {
				limit = 100
			} else {
				limit = l
			}
		}
	}

	page := 1
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	offset := (page - 1) * limit

	convs, err := h.messageService.GetAllConversations(r.Context(), limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(convs)
}

// AdminJoinConversation adds the admin as a participant in a conversation.
func (h *MessageHandler) AdminJoinConversation(w http.ResponseWriter, r *http.Request) {
	adminID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}
	convIDStr := chi.URLParam(r, "conversation_id")
	convID, err := strconv.ParseInt(convIDStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid conversation_id")
		return
	}
	conv, err := h.messageService.AdminJoinConversation(r.Context(), adminID, convID)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(conv)
}

func (h *MessageHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	var req model.SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	msg, err := h.messageService.SendMessage(r.Context(), userID, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
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
		respondError(w, http.StatusBadRequest, "invalid conversation_id")
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

	page := 1
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	offset := (page - 1) * limit

	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	paginatedResp, err := h.messageService.GetMessagesByConversation(r.Context(), convID, userID, limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(paginatedResp)
}

func (h *MessageHandler) UpdateMessage(w http.ResponseWriter, r *http.Request) {
	msgIDStr := chi.URLParam(r, "message_id")
	msgID, err := strconv.ParseInt(msgIDStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid message_id")
		return
	}

	var req struct {
		IsRead *bool `json:"isRead"`
	}
	// Decode is optional. If it fails, we proceed.
	_ = json.NewDecoder(r.Body).Decode(&req)

	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	// Default to true if not provided (compatibility with old /read POST shim)
	isRead := true
	if req.IsRead != nil {
		isRead = *req.IsRead
	}

	if isRead {
		if err := h.messageService.MarkMessageAsRead(r.Context(), msgID, userID); err != nil {
			if err == pgx.ErrNoRows {
				respondError(w, http.StatusNotFound, "message not found or already read")
				return
			}
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
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
		ClientTempID:   m.ClientTempID,
	}
}
