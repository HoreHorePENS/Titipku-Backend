package httpapi

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/titipku/titipku-backend/internal/platform/apperror"
	"github.com/titipku/titipku-backend/internal/platform/httpx"
	"github.com/titipku/titipku-backend/services/core-service/internal/usecase"
)

type productRequest struct {
	Name          string `json:"name"`
	PhotoPath     string `json:"photo_path"`
	ConsignPrice  int64  `json:"consign_price"`
	RetailPrice   int64  `json:"retail_price"`
	ShelfLifeDays int32  `json:"shelf_life_days"`
	IsActive      *bool  `json:"is_active"`
}

type activeRequest struct {
	IsActive *bool `json:"is_active"`
}

func (h *Handler) ListProducts(w http.ResponseWriter, r *http.Request) error {
	businessID, _, err := h.scope(r)
	if err != nil {
		return err
	}

	products, err := h.app.ListProducts(r.Context(), businessID, r.URL.Query().Get("active") == "true")
	if err != nil {
		return err
	}

	httpx.WriteData(w, r, http.StatusOK, newProductResponses(products))
	return nil
}

func (h *Handler) CreateProduct(w http.ResponseWriter, r *http.Request) error {
	return h.saveProduct(w, r, uuid.Nil, http.StatusCreated)
}

func (h *Handler) UpdateProduct(w http.ResponseWriter, r *http.Request) error {
	productID, err := pathUUID(r, "productID")
	if err != nil {
		return err
	}
	return h.saveProduct(w, r, productID, http.StatusOK)
}

func (h *Handler) saveProduct(w http.ResponseWriter, r *http.Request, productID uuid.UUID, status int) error {
	businessID, _, err := h.scope(r)
	if err != nil {
		return err
	}

	var request productRequest
	if err := httpx.DecodeJSON(w, r, &request); err != nil {
		return err
	}

	product, err := h.app.UpsertProduct(r.Context(), businessID, usecase.ProductInput{
		ID:            productID,
		Name:          request.Name,
		PhotoPath:     request.PhotoPath,
		ConsignPrice:  request.ConsignPrice,
		RetailPrice:   request.RetailPrice,
		ShelfLifeDays: request.ShelfLifeDays,
		IsActive:      request.IsActive,
	})
	if err != nil {
		return err
	}

	httpx.WriteData(w, r, status, newProductResponse(*product))
	return nil
}

func (h *Handler) SetProductActive(w http.ResponseWriter, r *http.Request) error {
	businessID, _, err := h.scope(r)
	if err != nil {
		return err
	}

	productID, err := pathUUID(r, "productID")
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

	product, err := h.app.SetProductActive(r.Context(), businessID, productID, *request.IsActive)
	if err != nil {
		return err
	}

	httpx.WriteData(w, r, http.StatusOK, newProductResponse(*product))
	return nil
}
