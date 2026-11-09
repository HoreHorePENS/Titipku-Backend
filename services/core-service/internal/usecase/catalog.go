package usecase

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/titipku/titipku-backend/internal/platform/apperror"
	"github.com/titipku/titipku-backend/internal/platform/imageupload"
	"github.com/titipku/titipku-backend/services/core-service/internal/domain"
)

type UpdateBusinessInput struct {
	Name      string
	OwnerName string
	WhatsApp  string
	Address   string
}

type ProductInput struct {
	ID            uuid.UUID
	Name          string
	PhotoPath     string
	ConsignPrice  int64
	RetailPrice   int64
	ShelfLifeDays int32
	IsActive      *bool
}

func (a *App) UpdateBusiness(ctx context.Context, businessID uuid.UUID, input UpdateBusinessInput) (*domain.Business, error) {
	name := strings.TrimSpace(input.Name)
	ownerName := strings.TrimSpace(input.OwnerName)

	fields := make(map[string]string)
	if !domain.IsValidName(name) {
		fields["name"] = "must be between 2 and 120 characters"
	}
	if !domain.IsValidName(ownerName) {
		fields["owner_name"] = "must be between 2 and 120 characters"
	}

	var whatsApp *string
	if strings.TrimSpace(input.WhatsApp) != "" {
		normalized, valid := domain.NormalizeWhatsApp(input.WhatsApp)
		if !valid {
			fields["whatsapp"] = "must be a valid Indonesian whatsapp number"
		} else {
			whatsApp = &normalized
		}
	}

	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}

	address := strings.TrimSpace(input.Address)
	var addressPtr *string
	if address != "" {
		addressPtr = &address
	}

	business, err := a.store.UpdateBusiness(ctx, domain.Business{
		ID:        businessID,
		Name:      name,
		OwnerName: ownerName,
		WhatsApp:  whatsApp,
		Address:   addressPtr,
	})
	if err != nil {
		return nil, mapStoreError(err, "BUSINESS_UPDATE_FAILED", "unable to update business")
	}
	return business, nil
}

func (a *App) ListProducts(ctx context.Context, businessID uuid.UUID, activeOnly bool) ([]domain.Product, error) {
	products, err := a.store.ListProducts(ctx, businessID, activeOnly)
	if err != nil {
		return nil, mapStoreError(err, "PRODUCT_LIST_FAILED", "unable to load products")
	}
	return products, nil
}

func (a *App) UpsertProduct(ctx context.Context, businessID uuid.UUID, input ProductInput) (*domain.Product, error) {
	name := strings.TrimSpace(input.Name)

	fields := make(map[string]string)
	if !domain.IsValidName(name) {
		fields["name"] = "must be between 2 and 120 characters"
	}
	if input.ConsignPrice < 0 {
		fields["consign_price"] = "must not be negative"
	}
	if input.RetailPrice < 0 {
		fields["retail_price"] = "must not be negative"
	}
	if !domain.IsValidShelfLife(input.ShelfLifeDays) {
		fields["shelf_life_days"] = "must be between 1 and 3650"
	}
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}

	id := input.ID
	if id == uuid.Nil {
		id = uuid.New()
	}

	isActive := true
	if input.IsActive != nil {
		isActive = *input.IsActive
	}

	var photoPath *string
	// Foto produk diatur lewat PUT/DELETE /v1/products/{id}/photo. Pada update,
	// repository mempertahankan foto lama; URL eksternal sembarang diabaikan.
	if path := strings.TrimSpace(input.PhotoPath); imageupload.IsUploadedPath(path) {
		photoPath = &path
	}

	product, err := a.store.UpsertProduct(ctx, domain.Product{
		ID:            id,
		BusinessID:    businessID,
		Name:          name,
		PhotoPath:     photoPath,
		ConsignPrice:  input.ConsignPrice,
		RetailPrice:   input.RetailPrice,
		ShelfLifeDays: input.ShelfLifeDays,
		IsActive:      isActive,
	})
	if err != nil {
		return nil, mapStoreError(err, "PRODUCT_WRITE_FAILED", "unable to save product")
	}
	return product, nil
}

func (a *App) SetProductActive(ctx context.Context, businessID, productID uuid.UUID, active bool) (*domain.Product, error) {
	product, err := a.store.SetProductActive(ctx, businessID, productID, active)
	if err != nil {
		return nil, mapStoreError(err, "PRODUCT_UPDATE_FAILED", "unable to update product")
	}
	return product, nil
}
