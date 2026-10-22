package httpapi

import (
	"net/http"

	"github.com/titipku/titipku-backend/internal/platform/httpx"
	"github.com/titipku/titipku-backend/services/core-service/internal/usecase"
)

type registerRequest struct {
	BusinessName string `json:"business_name"`
	OwnerName    string `json:"owner_name"`
	Email        string `json:"email"`
	WhatsApp     string `json:"whatsapp"`
	Password     string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) error {
	var request registerRequest
	if err := httpx.DecodeJSON(w, r, &request); err != nil {
		return err
	}

	result, err := h.app.Register(r.Context(), usecase.RegisterInput{
		BusinessName: request.BusinessName,
		OwnerName:    request.OwnerName,
		Email:        request.Email,
		WhatsApp:     request.WhatsApp,
		Password:     request.Password,
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

	result, err := h.app.Login(r.Context(), usecase.LoginInput{
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

	account, err := h.app.Account(r.Context(), principal.UserID)
	if err != nil {
		return err
	}

	httpx.WriteData(w, r, http.StatusOK, map[string]any{
		"user":     newUserResponse(account.User),
		"business": newBusinessResponse(account.Business),
	})
	return nil
}

type updateBusinessRequest struct {
	Name      string `json:"name"`
	OwnerName string `json:"owner_name"`
	WhatsApp  string `json:"whatsapp"`
	Address   string `json:"address"`
}

func (h *Handler) GetBusiness(w http.ResponseWriter, r *http.Request) error {
	principal, err := httpx.PrincipalFromRequest(r)
	if err != nil {
		return err
	}

	account, err := h.app.Account(r.Context(), principal.UserID)
	if err != nil {
		return err
	}

	httpx.WriteData(w, r, http.StatusOK, newBusinessResponse(account.Business))
	return nil
}

func (h *Handler) UpdateBusiness(w http.ResponseWriter, r *http.Request) error {
	businessID, _, err := h.scope(r)
	if err != nil {
		return err
	}

	var request updateBusinessRequest
	if err := httpx.DecodeJSON(w, r, &request); err != nil {
		return err
	}

	business, err := h.app.UpdateBusiness(r.Context(), businessID, usecase.UpdateBusinessInput{
		Name:      request.Name,
		OwnerName: request.OwnerName,
		WhatsApp:  request.WhatsApp,
		Address:   request.Address,
	})
	if err != nil {
		return err
	}

	httpx.WriteData(w, r, http.StatusOK, newBusinessResponse(*business))
	return nil
}
