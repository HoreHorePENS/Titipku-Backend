package httpapi

import (
	"time"

	"github.com/titipku/titipku-backend/services/auth-service/internal/domain"
	"github.com/titipku/titipku-backend/services/auth-service/internal/usecase"
)

type registerRequest struct {
	Email    string `json:"email"`
	FullName string `json:"full_name"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type userResponse struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	FullName  string    `json:"full_name"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type authResponse struct {
	AccessToken string       `json:"access_token"`
	TokenType   string       `json:"token_type"`
	ExpiresAt   time.Time    `json:"expires_at"`
	User        userResponse `json:"user"`
}

func newUserResponse(user domain.User) userResponse {
	return userResponse{
		ID:        user.ID.String(),
		Email:     user.Email,
		FullName:  user.FullName,
		Role:      string(user.Role),
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}

func newAuthResponse(result *usecase.Result) authResponse {
	return authResponse{
		AccessToken: result.AccessToken,
		TokenType:   "Bearer",
		ExpiresAt:   result.ExpiresAt,
		User:        newUserResponse(result.User),
	}
}
