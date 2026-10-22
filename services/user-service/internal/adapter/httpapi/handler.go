package httpapi

import (
	"net/http"

	"github.com/titipku/titipku-backend/internal/platform/httpx"
	"github.com/titipku/titipku-backend/services/user-service/internal/usecase"
)

type Handler struct {
	profiles *usecase.Service
}

func NewHandler(profiles *usecase.Service) *Handler {
	return &Handler{profiles: profiles}
}

func (h *Handler) GetMine(w http.ResponseWriter, r *http.Request) error {
	principal, err := httpx.PrincipalFromRequest(r)
	if err != nil {
		return err
	}

	profile, err := h.profiles.Get(r.Context(), principal.UserID)
	if err != nil {
		return err
	}

	httpx.WriteData(w, r, http.StatusOK, newProfileResponse(*profile))
	return nil
}

func (h *Handler) UpsertMine(w http.ResponseWriter, r *http.Request) error {
	principal, err := httpx.PrincipalFromRequest(r)
	if err != nil {
		return err
	}

	var request upsertProfileRequest
	if err := httpx.DecodeJSON(w, r, &request); err != nil {
		return err
	}

	profile, err := h.profiles.Upsert(r.Context(), principal.UserID, usecase.UpsertInput{
		FullName:  request.FullName,
		Phone:     request.Phone,
		AvatarURL: request.AvatarURL,
		Bio:       request.Bio,
	})
	if err != nil {
		return err
	}

	httpx.WriteData(w, r, http.StatusOK, newProfileResponse(*profile))
	return nil
}
