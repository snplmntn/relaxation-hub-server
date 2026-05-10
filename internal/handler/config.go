package handler

import (
	"encoding/json"
	"net/http"
)

const supabaseStorageBase = "https://qzdyqtdvcpmomujwionp.supabase.co/storage/v1/object/public/relaxation-hub-assets"

// AvatarsResponse is the response for GET /config/avatars
type AvatarsResponse struct {
	ClientAvatars    []string         `json:"client_avatars"`
	TherapistAvatars TherapistAvatars `json:"therapist_avatars"`
}

// TherapistAvatars holds male and female therapist avatar URLs
type TherapistAvatars struct {
	Female []string `json:"female"`
	Male   []string `json:"male"`
}

// ConfigHandler handles configuration-related endpoints
type ConfigHandler struct{}

// NewConfigHandler creates a new ConfigHandler
func NewConfigHandler() *ConfigHandler {
	return &ConfigHandler{}
}

// GetAvatars returns all available avatar URLs
func (h *ConfigHandler) GetAvatars(w http.ResponseWriter, r *http.Request) {
	resp := AvatarsResponse{
		ClientAvatars: []string{
			supabaseStorageBase + "/avatar_1.png",
			supabaseStorageBase + "/avatar_2.png",
			supabaseStorageBase + "/avatar_3.png",
			supabaseStorageBase + "/avatar_4.png",
			supabaseStorageBase + "/avatar_5.png",
			supabaseStorageBase + "/avatar_6.png",
			supabaseStorageBase + "/avatar_7.png",
			supabaseStorageBase + "/avatar_8.png",
			supabaseStorageBase + "/avatar_9.png",
			supabaseStorageBase + "/avatar_10.png",
		},
		TherapistAvatars: TherapistAvatars{
			Female: []string{
				supabaseStorageBase + "/therapist_female_avatar_1.png",
				supabaseStorageBase + "/therapist_female_avatar_2.png",
				supabaseStorageBase + "/therapist_female_avatar_3.png",
				supabaseStorageBase + "/therapist_female_avatar_4.png",
				supabaseStorageBase + "/therapist_female_avatar_5.png",
				supabaseStorageBase + "/therapist_female_avatar_6.png",
				supabaseStorageBase + "/therapist_female_avatar_7.png",
				supabaseStorageBase + "/therapist_female_avatar_8.png",
				supabaseStorageBase + "/therapist_female_avatar_9.png",
				supabaseStorageBase + "/therapist_female_avatar_10.png",
			},
			Male: []string{
				supabaseStorageBase + "/therapist_male_avatar_1.png",
				supabaseStorageBase + "/therapist_male_avatar_2.png",
				supabaseStorageBase + "/therapist_male_avatar_3.png",
				supabaseStorageBase + "/therapist_male_avatar_4.png",
				supabaseStorageBase + "/therapist_male_avatar_5.png",
				supabaseStorageBase + "/therapist_male_avatar_6.png",
				supabaseStorageBase + "/therapist_male_avatar_7.png",
				supabaseStorageBase + "/therapist_male_avatar_8.png",
				supabaseStorageBase + "/therapist_male_avatar_9.png",
				supabaseStorageBase + "/therapist_male_avatar_10.png",
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
