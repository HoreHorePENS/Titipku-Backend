package httpapi

import (
	"net/http"

	"github.com/titipku/titipku-backend/internal/platform/httpx"
	"github.com/titipku/titipku-backend/services/core-service/internal/domain"
)

type partnerStockResponse struct {
	ProductID     string  `json:"product_id"`
	ProductName   string  `json:"product_name"`
	PhotoPath     *string `json:"photo_path,omitempty"`
	ConsignPrice  int64   `json:"consign_price"`
	IsActive      bool    `json:"is_active"`
	OnShelf       int64   `json:"on_shelf"`
	NearestExpiry *string `json:"nearest_expiry,omitempty"`
	EverConsigned bool    `json:"ever_consigned"`
}

func newPartnerStockResponses(items []domain.PartnerStockItem) []partnerStockResponse {
	responses := make([]partnerStockResponse, 0, len(items))
	for _, item := range items {
		response := partnerStockResponse{
			ProductID:     item.ProductID.String(),
			ProductName:   item.ProductName,
			PhotoPath:     item.PhotoPath,
			ConsignPrice:  item.ConsignPrice,
			IsActive:      item.IsActive,
			OnShelf:       item.OnShelf,
			EverConsigned: item.EverConsigned,
		}
		if item.NearestExpiry != nil {
			date := item.NearestExpiry.Format("2006-01-02")
			response.NearestExpiry = &date
		}
		responses = append(responses, response)
	}
	return responses
}

// PartnerStock: GET /v1/partners/{partnerID}/stock (penitip).
func (h *Handler) PartnerStock(w http.ResponseWriter, r *http.Request) error {
	businessID, _, err := h.scope(r)
	if err != nil {
		return err
	}
	partnerID, err := pathUUID(r, "partnerID")
	if err != nil {
		return err
	}
	items, err := h.app.PartnerStock(r.Context(), businessID, partnerID)
	if err != nil {
		return err
	}
	httpx.WriteData(w, r, http.StatusOK, newPartnerStockResponses(items))
	return nil
}
