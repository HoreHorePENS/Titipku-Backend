package httpapi

import (
	"net/http"

	"github.com/titipku/titipku-backend/internal/platform/httpx"
	"github.com/titipku/titipku-backend/services/auth-service/internal/usecase"
)

type Handler struct {
	auth *usecase.Auth
}

func NewHandler(auth *usecase.Auth) *Handler {
	return &Handler{auth: auth}
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) error {
	var request registerRequest
	if err := httpx.DecodeJSON(w, r, &request); err != nil {
		return err
	}

	result, err := h.auth.Register(r.Context(), usecase.RegisterInput{
		Email:    request.Email,
		FullName: request.FullName,
		Password: request.Password,
	})
	if err != nil {
		return err
	}

	httpx.WriteData(w, r, http.StatusCreated, newAuthResponse(result))
	return nil
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) error {
	var request loginRequest
	if err := httpx.DecodeJSON(w, r, &request); err != nil {
		return err
	}

	result, err := h.auth.Login(r.Context(), usecase.LoginInput{
		Email:    request.Email,
		Password: request.Password,
	})
	if err != nil {
		return err
	}

	httpx.WriteData(w, r, http.StatusOK, newAuthResponse(result))
	return nil
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) error {
	principal, err := httpx.PrincipalFromRequest(r)
	if err != nil {
		return err
	}

	user, err := h.auth.Profile(r.Context(), principal.UserID)
	if err != nil {
		return err
	}

	httpx.WriteData(w, r, http.StatusOK, newUserResponse(*user))
	return nil
}
