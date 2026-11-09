package httpapi

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/titipku/titipku-backend/internal/platform/apperror"
	"github.com/titipku/titipku-backend/internal/platform/httpx"
	"github.com/titipku/titipku-backend/services/core-service/internal/usecase"
)

type partnerRequest struct {
	Name      string  `json:"name"`
	OwnerName string  `json:"owner_name"`
	WhatsApp  string  `json:"whatsapp"`
	Address   string  `json:"address"`
	IsActive  *bool   `json:"is_active"`
	// StoreID diisi bila menambahkan toko dari katalog peta.
	StoreID   *string `json:"store_id"`
}

func (h *Handler) ListPartners(w http.ResponseWriter, r *http.Request) error {
	businessID, _, err := h.scope(r)
	if err != nil {
		return err
	}

	partners, err := h.app.ListPartners(r.Context(), businessID, r.URL.Query().Get("active") == "true")
	if err != nil {
		return err
	}

	httpx.WriteData(w, r, http.StatusOK, newPartnerResponses(partners))
	return nil
}

func (h *Handler) CreatePartner(w http.ResponseWriter, r *http.Request) error {
	return h.savePartner(w, r, uuid.Nil, http.StatusCreated)
}

func (h *Handler) UpdatePartner(w http.ResponseWriter, r *http.Request) error {
	partnerID, err := pathUUID(r, "partnerID")
	if err != nil {
		return err
	}
	return h.savePartner(w, r, partnerID, http.StatusOK)
}

func (h *Handler) savePartner(w http.ResponseWriter, r *http.Request, partnerID uuid.UUID, status int) error {
	businessID, _, err := h.scope(r)
	if err != nil {
		return err
	}

	var request partnerRequest
	if err := httpx.DecodeJSON(w, r, &request); err != nil {
		return err
	}

	var storeID *uuid.UUID
	if request.StoreID != nil && *request.StoreID != "" {
		parsed, err := uuid.Parse(*request.StoreID)
		if err != nil {
			return apperror.Validation(map[string]string{"store_id": "must be a valid uuid"})
		}
		storeID = &parsed
	}

	partner, err := h.app.UpsertPartner(r.Context(), businessID, usecase.PartnerInput{
		ID:        partnerID,
		Name:      request.Name,
		OwnerName: request.OwnerName,
		WhatsApp:  request.WhatsApp,
		Address:   request.Address,
		IsActive:  request.IsActive,
		StoreID:   storeID,
	})
	if err != nil {
		return err
	}

	httpx.WriteData(w, r, status, newPartnerResponse(*partner))
	return nil
}

func (h *Handler) SetPartnerActive(w http.ResponseWriter, r *http.Request) error {
	businessID, _, err := h.scope(r)
	if err != nil {
		return err
	}

	partnerID, err := pathUUID(r, "partnerID")
	if err != nil {
		return err
	}

	var request activeRequest
	if err := httpx.DecodeJSON(w, r, &request); err != nil {
		return err
	}
	if request.IsActive == nil {
		return apperror.Validation(map[string]string{"is_active": "is required"})
	}

	partner, err := h.app.SetPartnerActive(r.Context(), businessID, partnerID, *request.IsActive)
	if err != nil {
		return err
	}

	httpx.WriteData(w, r, http.StatusOK, newPartnerResponse(*partner))
	return nil
}
