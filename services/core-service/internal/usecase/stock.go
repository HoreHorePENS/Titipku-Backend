package usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/titipku/titipku-backend/internal/platform/apperror"
	"github.com/titipku/titipku-backend/services/core-service/internal/domain"
)

// PartnerStock: stok produk penitip di satu mitra miliknya (dilihat oleh penitip).
func (a *App) PartnerStock(ctx context.Context, businessID, partnerID uuid.UUID) ([]domain.PartnerStockItem, error) {
	items, err := a.store.PartnerStock(ctx, businessID, partnerID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, apperror.NotFound("PARTNER_NOT_FOUND", "partner was not found")
		}
		return nil, mapStoreError(err, "PARTNER_STOCK_FAILED", "unable to load partner stock")
	}
	return items, nil
}
