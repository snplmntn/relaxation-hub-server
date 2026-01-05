package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"mime"
	"net/http"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

type UserHandler struct {
	userService    service.UserService
	storageService service.StorageService
}

func NewUserHandler(userService service.UserService, storageService service.StorageService) *UserHandler {
	return &UserHandler{userService: userService, storageService: storageService}
}

func (h *UserHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	var req model.UpdateUserProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	updates := make(map[string]interface{})
	if req.FullName != nil {
		updates["full_name"] = *req.FullName
	}
	if req.Gender != nil {
		updates["gender"] = *req.Gender
	}
	if req.EmergencyContactName != nil {
		updates["emergency_contact_name"] = *req.EmergencyContactName
	}
	if req.EmergencyContactPhone != nil {
		updates["emergency_contact_phone"] = *req.EmergencyContactPhone
	}
	if req.ProfilePhoto != nil {
		updates["profile_photo"] = *req.ProfilePhoto
	}
	if req.PrimaryPhone != nil {
		updates["primary_phone"] = *req.PrimaryPhone
	}
	if req.Email != nil {
		updates["primary_email"] = *req.Email
	}

	user, err := h.userService.Update(r.Context(), userID, updates)
	if err != nil {
		if err == pgx.ErrNoRows {
			respondError(w, http.StatusNotFound, "user not found")
			return
		}
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func (h *UserHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	user, err := h.userService.Get(r.Context(), userID)
	if err != nil {
		if err == pgx.ErrNoRows {
			respondError(w, http.StatusNotFound, "user not found")
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

// ListUsers returns a list of users. Optional query param `role` filters by role.
func (h *UserHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	role := r.URL.Query().Get("role")

	users, err := h.userService.List(r.Context(), role)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Convert to JSON-friendly objects
	out := make([]map[string]interface{}, 0, len(users))
	for _, u := range users {
		out = append(out, map[string]interface{}{
			"user_id": u.UserID,
			"full_name": u.FullName,
			"role": u.Role,
			"email": u.PrimaryEmail,
			"phone": u.PrimaryPhone,
			"profile_photo": u.ProfilePhoto,
			"gender": u.Gender,
			"emergency_contact_name": u.EmergencyContactName,
			"emergency_contact_phone": u.EmergencyContactPhone,
			"created_at": u.CreatedAt,
			"updated_at": u.UpdatedAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"users": out, "count": len(out)})
}
func (h *UserHandler) BlockUser(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	var req model.BlockUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.userService.BlockUser(r.Context(), userID, req.BlockedUserID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *UserHandler) UnblockUser(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	var req model.BlockUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.userService.UnblockUser(r.Context(), userID, req.BlockedUserID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *UserHandler) GetBlockList(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	list, err := h.userService.GetBlockList(r.Context(), userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"blocked_users": list, "count": len(list)})
}

// UpdateFCMToken updates the FCM token for push notifications
// AddFavorite adds a therapist to the user's favorites list
func (h *UserHandler) AddFavorite(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	therapistIDStr := chi.URLParam(r, "therapist_id")
	if therapistIDStr == "" {
		respondError(w, http.StatusBadRequest, "therapist_id is required")
		return
	}

	var therapistID int64
	if _, err := fmt.Sscanf(therapistIDStr, "%d", &therapistID); err != nil {
		respondError(w, http.StatusBadRequest, "invalid therapist_id")
		return
	}

	if err := h.userService.AddFavorite(r.Context(), userID, therapistID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// RemoveFavorite removes a therapist from the user's favorites list
func (h *UserHandler) RemoveFavorite(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	therapistIDStr := chi.URLParam(r, "therapist_id")
	if therapistIDStr == "" {
		respondError(w, http.StatusBadRequest, "therapist_id is required")
		return
	}

	var therapistID int64
	if _, err := fmt.Sscanf(therapistIDStr, "%d", &therapistID); err != nil {
		respondError(w, http.StatusBadRequest, "invalid therapist_id")
		return
	}

	if err := h.userService.RemoveFavorite(r.Context(), userID, therapistID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListFavorites returns the list of favorite therapists for the user
func (h *UserHandler) ListFavorites(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	favorites, err := h.userService.ListFavorites(r.Context(), userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"favorites": favorites, "count": len(favorites)})
}

func (h *UserHandler) UpdateFCMToken(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	var req model.UpdateFCMTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.FCMToken == "" {
		respondError(w, http.StatusBadRequest, "fcm_token is required")
		return
	}

	if err := h.userService.UpdateFCMToken(r.Context(), userID, req.FCMToken); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// UploadProfilePhoto handles file upload for user profile photo.
func (h *UserHandler) UploadProfilePhoto(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	// Verify storage is configured
	if h.storageService == nil || !h.storageService.IsConfigured() {
		respondError(w, http.StatusInternalServerError, "storage not configured")
		return
	}

	// Parse multipart form (max 5MB for profile photos)
	if err := r.ParseMultipartForm(5 << 20); err != nil {
		respondError(w, http.StatusBadRequest, "invalid form data")
		return
	}

	file, header, err := r.FormFile("photo")
	if err != nil {
		respondError(w, http.StatusBadRequest, "missing photo file")
		return
	}
	defer file.Close()

	// Generate storage key
	key := h.storageService.GenerateKey(fmt.Sprintf("profiles/user_%d", userID), header.Filename)

	// Determine content type
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = mime.TypeByExtension(filepath.Ext(header.Filename))
	}
	if contentType == "" {
		contentType = "image/jpeg"
	}

	// Upload to storage
	photoURL, err := h.storageService.UploadFile(r.Context(), key, file, contentType)
	if err != nil {
		log.Printf("Storage upload error: %v", err)
		respondError(w, http.StatusInternalServerError, "failed to upload photo")
		return
	}

	// Update user profile with new photo URL
	updates := map[string]interface{}{"profile_photo": photoURL}
	user, err := h.userService.Update(r.Context(), userID, updates)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"profile_photo": photoURL,
		"user":          user,
	})
}
