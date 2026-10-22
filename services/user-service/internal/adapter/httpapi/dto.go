package httpapi

import (
	"time"

	"github.com/titipku/titipku-backend/services/user-service/internal/domain"
)

type upsertProfileRequest struct {
	FullName  string `json:"full_name"`
	Phone     string `json:"phone"`
	AvatarURL string `json:"avatar_url"`
	Bio       string `json:"bio"`
}

type profileResponse struct {
	UserID    string    `json:"user_id"`
	FullName  string    `json:"full_name"`
	Phone     string    `json:"phone"`
	AvatarURL string    `json:"avatar_url"`
	Bio       string    `json:"bio"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func newProfileResponse(profile domain.Profile) profileResponse {
	return profileResponse{
		UserID:    profile.UserID.String(),
		FullName:  profile.FullName,
		Phone:     profile.Phone,
		AvatarURL: profile.AvatarURL,
		Bio:       profile.Bio,
		CreatedAt: profile.CreatedAt,
		UpdatedAt: profile.UpdatedAt,
	}
}
