package httpapi

import (
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/titipku/titipku-backend/internal/platform/apperror"
	"github.com/titipku/titipku-backend/internal/platform/httpx"
	"github.com/titipku/titipku-backend/internal/platform/identity"
	"github.com/titipku/titipku-backend/services/core-service/internal/usecase"
)

type storeRequest struct {
	Name        string   `json:"name"`
	Category    string   `json:"category"`
	Description string   `json:"description"`
	Address     string   `json:"address"`
	WhatsApp    string   `json:"whatsapp"`
	Latitude    float64  `json:"latitude"`
	Longitude   float64  `json:"longitude"`
	PhotoURL    string   `json:"photo_url"`
	IsActive    *bool    `json:"is_active"`
}

func (h *Handler) ListStores(w http.ResponseWriter, r *http.Request) error {
	businessID := viewerBusinessID(r)

	query := r.URL.Query()
	input := usecase.StoreListInput{
		Category: query.Get("category"),
		Search:   query.Get("q"),
	}
	if value, err := strconv.ParseFloat(query.Get("min_lat"), 64); err == nil {
		input.MinLat = &value
	}
	if value, err := strconv.ParseFloat(query.Get("max_lat"), 64); err == nil {
		input.MaxLat = &value
	}
	if value, err := strconv.ParseFloat(query.Get("min_lng"), 64); err == nil {
		input.MinLng = &value
	}
	if value, err := strconv.ParseFloat(query.Get("max_lng"), 64); err == nil {
		input.MaxLng = &value
	}
	if value, err := strconv.Atoi(query.Get("limit")); err == nil {
		input.Limit = value
	}
	if value, err := strconv.Atoi(query.Get("offset")); err == nil {
		input.Offset = value
	}

	stores, err := h.app.ListStores(r.Context(), businessID, input)
	if err != nil {
		return err
	}

	httpx.WriteData(w, r, http.StatusOK, newStoreResponses(stores))
	return nil
}

func (h *Handler) GetStore(w http.ResponseWriter, r *http.Request) error {
	storeID, err := pathUUID(r, "storeID")
	if err != nil {
		return err
	}

	store, err := h.app.StoreDetail(r.Context(), storeID, viewerBusinessID(r))
	if err != nil {
		return err
	}

	httpx.WriteData(w, r, http.StatusOK, newStoreResponse(*store))
	return nil
}

func (h *Handler) MyStore(w http.ResponseWriter, r *http.Request) error {
	principal, err := httpx.PrincipalFromRequest(r)
	if err != nil {
		return err
	}

	store, err := h.app.MyStore(r.Context(), principal.UserID)
	if err != nil {
		return err
	}

	httpx.WriteData(w, r, http.StatusOK, newStoreResponse(*store))
	return nil
}

func (h *Handler) CreateStore(w http.ResponseWriter, r *http.Request) error {
	principal, err := httpx.PrincipalFromRequest(r)
	if err != nil {
		return err
	}

	var request storeRequest
	if err := httpx.DecodeJSON(w, r, &request); err != nil {
		return err
	}

	store, err := h.app.CreateStore(r.Context(), principal.UserID, newStoreInput(request))
	if err != nil {
		return err
	}

	httpx.WriteData(w, r, http.StatusCreated, newStoreResponse(*store))
	return nil
}

func (h *Handler) UpdateMyStore(w http.ResponseWriter, r *http.Request) error {
	principal, err := httpx.PrincipalFromRequest(r)
	if err != nil {
		return err
	}

	var request storeRequest
	if err := httpx.DecodeJSON(w, r, &request); err != nil {
		return err
	}

	store, err := h.app.UpdateMyStore(r.Context(), principal.UserID, newStoreInput(request))
	if err != nil {
		return err
	}

	httpx.WriteData(w, r, http.StatusOK, newStoreResponse(*store))
	return nil
}

func (h *Handler) SetMyStoreActive(w http.ResponseWriter, r *http.Request) error {
	principal, err := httpx.PrincipalFromRequest(r)
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

	stores, err := h.app.MyStore(r.Context(), principal.UserID)
	if err != nil {
		return err
	}

	store, err := h.app.SetMyStoreActive(r.Context(), principal.UserID, stores.ID, *request.IsActive)
	if err != nil {
		return err
	}

	httpx.WriteData(w, r, http.StatusOK, newStoreResponse(*store))
	return nil
}

func newStoreInput(request storeRequest) usecase.StoreInput {
	return usecase.StoreInput{
		Name:        request.Name,
		Category:    request.Category,
		Description: request.Description,
		Address:     request.Address,
		WhatsApp:    request.WhatsApp,
		Latitude:    request.Latitude,
		Longitude:   request.Longitude,
		PhotoURL:    request.PhotoURL,
		IsActive:    request.IsActive,
	}
}

// viewerBusinessID mengembalikan business scope penitip yang sedang login,
// atau nil untuk akun tanpa business scope (pemilik toko).
func viewerBusinessID(r *http.Request) *uuid.UUID {
	principal, err := httpx.PrincipalFromRequest(r)
	if err != nil || !principal.HasBusinessScope() {
		return nil
	}
	businessID := principal.BusinessID
	return &businessID
}

// principalAccountType mengembalikan persona akun pada token.
func principalAccountType(r *http.Request) identity.AccountType {
	principal, err := httpx.PrincipalFromRequest(r)
	if err != nil || principal.AccountType == "" {
		return identity.AccountTypePenitip
	}
	return principal.AccountType
}
