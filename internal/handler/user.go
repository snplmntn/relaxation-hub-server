package handler

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

type UserHandler struct {
	userService    service.UserService
	storageService service.StorageService
	authService    service.AuthService
}

func NewUserHandler(userService service.UserService, storageService service.StorageService, authService service.AuthService) *UserHandler {
	return &UserHandler{userService: userService, storageService: storageService, authService: authService}
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
	if req.NotificationPreferences != nil {
		updates["notification_preferences"] = req.NotificationPreferences
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

// ListUsers returns a list of users. Optional query params: role, page, limit, q.
func (h *UserHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	// Security: Only admins can list all users
	requestingUserRole, ok := middleware.GetUserRole(r)
	if !ok || requestingUserRole != "admin" {
		respondError(w, http.StatusForbidden, "access denied: admin role required")
		return
	}

	role := r.URL.Query().Get("role")
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")
	search := r.URL.Query().Get("q")

	page := 1
	limit := 20
	if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
		page = p
	}
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
		limit = l
	}

	users, total, err := h.userService.ListPaginated(r.Context(), role, page, limit, search)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Convert to JSON-friendly objects
	out := make([]map[string]interface{}, 0, len(users))
	for _, u := range users {
		out = append(out, map[string]interface{}{
			"user_id":                 u.UserID,
			"full_name":               u.FullName,
			"role":                    u.Role,
			"status":                  u.AccountStatus,
			"email":                   u.PrimaryEmail,
			"phone":                   u.PrimaryPhone,
			"profile_photo":           u.ProfilePhoto,
			"gender":                  u.Gender,
			"emergency_contact_name":  u.EmergencyContactName,
			"emergency_contact_phone": u.EmergencyContactPhone,
			"created_at":              u.CreatedAt,
			"updated_at":              u.UpdatedAt,
		})
	}

	totalPages := (total + limit - 1) / limit
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"users": out,
		"pagination": map[string]interface{}{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": totalPages,
		},
	})
}

func (h *UserHandler) AdminExportUsers(w http.ResponseWriter, r *http.Request) {
	requestingUserRole, ok := middleware.GetUserRole(r)
	if !ok || requestingUserRole != "admin" {
		respondError(w, http.StatusForbidden, "access denied: admin role required")
		return
	}

	role := strings.TrimSpace(r.URL.Query().Get("role"))
	if role == "" {
		role = "client"
	}
	search := strings.TrimSpace(r.URL.Query().Get("q"))
	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
	if format == "" {
		format = "csv"
	}
	if format != "csv" && format != "json" {
		respondError(w, http.StatusBadRequest, "invalid format: use csv or json")
		return
	}

	page := 1
	const pageSize = 100
	exportUsers := make([]model.User, 0, pageSize)

	for {
		users, total, err := h.userService.ListPaginated(r.Context(), role, page, pageSize, search)
		if err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		for _, u := range users {
			if status == "" || strings.EqualFold(u.AccountStatus, status) {
				exportUsers = append(exportUsers, u)
			}
		}

		if len(users) == 0 || (page*pageSize) >= total {
			break
		}
		page++
	}

	if format == "json" {
		type outUser struct {
			UserID    int       `json:"user_id"`
			FullName  string    `json:"full_name"`
			Role      string    `json:"role"`
			Status    string    `json:"status"`
			Email     string    `json:"email,omitempty"`
			Phone     string    `json:"phone,omitempty"`
			CreatedAt time.Time `json:"created_at"`
			UpdatedAt time.Time `json:"updated_at"`
		}

		out := make([]outUser, 0, len(exportUsers))
		for _, u := range exportUsers {
			out = append(out, outUser{
				UserID:    u.UserID,
				FullName:  u.FullName,
				Role:      u.Role,
				Status:    u.AccountStatus,
				Email:     u.PrimaryEmail,
				Phone:     u.PrimaryPhone,
				CreatedAt: u.CreatedAt,
				UpdatedAt: u.UpdatedAt,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"users": out,
			"total": len(out),
		})
		return
	}

	csvField := func(value string) string {
		trimmed := strings.TrimLeft(value, " \t\r\n")
		if strings.HasPrefix(trimmed, "=") || strings.HasPrefix(trimmed, "+") || strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "@") {
			return "'" + value
		}
		return value
	}

	filename := fmt.Sprintf("users-%s.csv", time.Now().UTC().Format("20060102-150405"))
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))

	writer := csv.NewWriter(w)
	defer writer.Flush()

	if err := writer.Write([]string{"User ID", "Full Name", "Role", "Status", "Email", "Phone", "Created At", "Updated At"}); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to write export headers")
		return
	}

	for _, u := range exportUsers {
		row := []string{
			strconv.FormatInt(int64(u.UserID), 10),
			csvField(u.FullName),
			csvField(u.Role),
			csvField(u.AccountStatus),
			csvField(u.PrimaryEmail),
			csvField(u.PrimaryPhone),
			u.CreatedAt.Format(time.RFC3339),
			u.UpdatedAt.Format(time.RFC3339),
		}
		if err := writer.Write(row); err != nil {
			respondError(w, http.StatusInternalServerError, "failed to write export data")
			return
		}
	}
}

func (h *UserHandler) BlockUser(w http.ResponseWriter, r *http.Request) {
	requestingUserID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	targetIDStr := chi.URLParam(r, "id")
	var targetID int64
	var err error

	if targetIDStr != "" {
		targetID, err = strconv.ParseInt(targetIDStr, 10, 64)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid user id in path")
			return
		}
	} else {
		// Try body for backward compatibility (POST /users/block Case)
		var req model.BlockUserRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "blocked_user_id is required")
			return
		}
		targetID = req.BlockedUserID
	}

	if err := h.userService.BlockUser(r.Context(), requestingUserID, targetID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *UserHandler) UnblockUser(w http.ResponseWriter, r *http.Request) {
	requestingUserID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	targetIDStr := chi.URLParam(r, "id")
	var targetID int64
	var err error

	if targetIDStr != "" {
		targetID, err = strconv.ParseInt(targetIDStr, 10, 64)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid user id in path")
			return
		}
	} else {
		// Try body for backward compatibility (POST /users/unblock Case)
		var req model.UnblockUserRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "unblocked_user_id is required")
			return
		}
		targetID = req.UnblockedUserID
	}

	if err := h.userService.UnblockUser(r.Context(), requestingUserID, targetID); err != nil {
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

	// Fetch current user to check for existing photo
	currentUser, err := h.userService.Get(r.Context(), userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to fetch user profile")
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
		slog.Warn("storage upload error", "error", err)
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

	// Cleanup old photo if exists
	if currentUser.ProfilePhoto != "" {
		if key := extractProfileS3Key(currentUser.ProfilePhoto); key != "" {
			// Best effort deletion
			go func(k string) {
				_ = h.storageService.DeleteFile(context.Background(), k)
			}(key)
		}
	}
}

func extractProfileS3Key(s3URL string) string {
	parsed, err := url.Parse(s3URL)
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(parsed.Path, "/")
}

// AdminUpdateStatus allows an admin to update a user's account status.
func (h *UserHandler) AdminUpdateStatus(w http.ResponseWriter, r *http.Request) {
	// Middleware already verifies admin role for this route group

	userIDStr := chi.URLParam(r, "userID")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	var req struct {
		AccountStatus string `json:"account_status"`
		StatusReason  string `json:"status_reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate status
	validStatuses := map[string]bool{
		model.AccountStatusActive:    true,
		model.AccountStatusInactive:  true,
		model.AccountStatusSuspended: true,
		model.AccountStatusVIP:       true,
	}
	if !validStatuses[req.AccountStatus] {
		respondError(w, http.StatusBadRequest, "invalid account status")
		return
	}

	updates := map[string]interface{}{
		"account_status": req.AccountStatus,
	}
	// Only update reason if provided, or clear it if activating
	if req.AccountStatus == "active" {
		updates["status_reason"] = ""
	} else if req.StatusReason != "" {
		updates["status_reason"] = req.StatusReason
	}

	updatedUser, err := h.userService.Update(r.Context(), userID, updates)
	if err != nil {
		if err == pgx.ErrNoRows {
			respondError(w, http.StatusNotFound, "user not found")
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updatedUser)
}

func (h *UserHandler) AdminCreateUser(w http.ResponseWriter, r *http.Request) {
	var req AuthRequest // Reusing AuthRequest from AuthHandler, but it's not exported there effectively for use here?
	// Ah, AuthRequest is defined in auth.go but it's in the same package 'handler'. So I can use it.

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate role if needed. Admin creating user can pick role.
	if req.Role == "" {
		req.Role = "client" // default?
	}

	var userID int
	var err error
	if req.Role == "therapist" {
		userID, _, err = h.authService.SignupWithTherapistProfile(r.Context(), req.Provider, req.ProviderKey, req.Password, req.Role)
	} else {
		userID, _, err = h.authService.Signup(r.Context(), req.Provider, req.ProviderKey, req.Password, req.Role)
	}
	if err != nil {
		if strings.Contains(err.Error(), "already in use") {
			respondError(w, http.StatusConflict, err.Error())
			return
		}
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// If FullName or Phone is provided in the request body but not used by Signup (Signup uses provider_key as email/phone),
	// we might need to update the user profile immediately.
	// Signup function in AuthService (lines 130-140) sets PrimaryEmail or PrimaryPhone based on provider.
	// But Full Name?
	// AuthService.Signup creates model.User with just Role and timestamps. FULL NAME IS MISSING in Signup logic!
	// Wait, let me check AuthService.Signup again.
	// Line 130: user := model.User{ Role: role, ... }
	// It does NOT set FullName.
	// So I need to update the user profile after creation if FullName is provided.

	updates := make(map[string]interface{})
	if req.FullName != "" {
		updates["full_name"] = req.FullName
	}
	// Also if provider is email, but phone is provided separately?
	// Note: AuthRequest struct in auth.go has FullName, Phone.
	if req.Phone != "" && req.Provider == "email" {
		updates["primary_phone"] = req.Phone
	}
	if req.Provider == "phone" && strings.Contains(req.ProviderKey, "@") {
		// weird case, but if email provided separately
	}

	if len(updates) > 0 {
		_, err := h.userService.Update(r.Context(), int64(userID), updates)
		if err != nil {
			// Log error but don't fail the request completely as user is created
			slog.Error("failed to update user profile after creation", "user_id", userID, "error", err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "User created successfully",
		"user_id": userID,
	})
}

// AdminUpdateUserProfile allows admins to update any user's profile
func (h *UserHandler) AdminUpdateUserProfile(w http.ResponseWriter, r *http.Request) {
	userIDStr := chi.URLParam(r, "userID")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid user ID")
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

func (h *UserHandler) AdminDeactivateClient(w http.ResponseWriter, r *http.Request) {
	h.adminSetClientLifecycle(w, r, false)
}

func (h *UserHandler) AdminReactivateClient(w http.ResponseWriter, r *http.Request) {
	h.adminSetClientLifecycle(w, r, true)
}

func (h *UserHandler) adminSetClientLifecycle(w http.ResponseWriter, r *http.Request, active bool) {
	userID, err := strconv.ParseInt(chi.URLParam(r, "userID"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	var user *model.User
	if active {
		user, err = h.userService.ReactivateClient(r.Context(), userID)
	} else {
		user, err = h.userService.DeactivateClient(r.Context(), userID)
	}
	if err != nil {
		if err == pgx.ErrNoRows || strings.Contains(err.Error(), "not found") {
			respondError(w, http.StatusNotFound, "user not found")
			return
		}
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}
