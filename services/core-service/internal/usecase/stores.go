package usecase

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/titipku/titipku-backend/internal/platform/apperror"
	"github.com/titipku/titipku-backend/internal/platform/identity"
	"github.com/titipku/titipku-backend/internal/platform/imageupload"
	"github.com/titipku/titipku-backend/services/core-service/internal/domain"
)

type StoreListInput struct {
	MinLat, MaxLat, MinLng, MaxLng *float64
	Category                       string
	Search                         string
	Limit                          int
	Offset                         int
}

type StoreInput struct {
	Name        string
	Category    string
	Description string
	Address     string
	WhatsApp    string
	Latitude    float64
	Longitude   float64
	PhotoURL    string
	IsActive    *bool
}

// ListStores mengembalikan katalog toko untuk peta.
// viewerBusinessID dipakai menandai toko yang sudah dititipkan.
func (a *App) ListStores(ctx context.Context, viewerBusinessID *uuid.UUID, input StoreListInput) ([]domain.Store, error) {
	if input.Category != "" && !domain.IsValidStoreCategory(input.Category) {
		return nil, apperror.Validation(map[string]string{
			"category": "must be one of " + strings.Join(domain.StoreCategories, ", "),
		})
	}
	if input.Limit <= 0 || input.Limit > 200 {
		input.Limit = 200
	}
	if input.Offset < 0 {
		input.Offset = 0
	}

	stores, err := a.store.ListStores(ctx, domain.StoreFilter{
		MinLat:           input.MinLat,
		MaxLat:           input.MaxLat,
		MinLng:           input.MinLng,
		MaxLng:           input.MaxLng,
		Category:         input.Category,
		Search:           input.Search,
		ActiveOnly:       true,
		ViewerBusinessID: viewerBusinessID,
		Limit:            input.Limit,
		Offset:           input.Offset,
	})
	if err != nil {
		return nil, mapStoreError(err, "STORE_LIST_FAILED", "unable to load stores")
	}
	return stores, nil
}

// StoreDetail mengembalikan satu toko beserta status sudah-dititipkan.
func (a *App) StoreDetail(ctx context.Context, storeID uuid.UUID, viewerBusinessID *uuid.UUID) (*domain.Store, error) {
	store, err := a.store.StoreByID(ctx, storeID, viewerBusinessID)
	if err != nil {
		return nil, mapStoreError(err, "STORE_LOOKUP_FAILED", "unable to load store")
	}
	return store, nil
}

// MyStore mengembalikan toko milik akun pemilik toko.
func (a *App) MyStore(ctx context.Context, userID uuid.UUID) (*domain.Store, error) {
	store, err := a.store.StoreByOwnerUserID(ctx, userID)
	if err != nil {
		return nil, mapStoreError(err, "STORE_LOOKUP_FAILED", "unable to load store")
	}
	return store, nil
}

// UpdateMyStore memperbarui profil toko milik akun pemilik toko.
func (a *App) UpdateMyStore(ctx context.Context, userID uuid.UUID, input StoreInput) (*domain.Store, error) {
	store, err := a.store.StoreByOwnerUserID(ctx, userID)
	if err != nil {
		return nil, mapStoreError(err, "STORE_LOOKUP_FAILED", "unable to load store")
	}

	fields, normalizedWhatsApp := validateStoreInput(input)
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}

	store.Name = strings.TrimSpace(input.Name)
	store.Category = strings.TrimSpace(input.Category)
	if description := strings.TrimSpace(input.Description); description != "" {
		store.Description = &description
	} else {
		store.Description = nil
	}
	store.Address = strings.TrimSpace(input.Address)
	store.WhatsApp = normalizedWhatsApp
	store.Latitude = input.Latitude
	store.Longitude = input.Longitude
	// Foto toko tidak diubah lewat JSON; gunakan PUT/DELETE /v1/stores/mine/photo.
	if input.IsActive != nil {
		store.IsActive = *input.IsActive
	}

	updated, err := a.store.UpdateStore(ctx, *store)
	if err != nil {
		return nil, mapStoreError(err, "STORE_UPDATE_FAILED", "unable to update store")
	}
	return updated, nil
}

// SetMyStoreActive menonaktifkan/mengaktifkan toko agar tampil atau tidak di peta.
func (a *App) SetMyStoreActive(ctx context.Context, userID, storeID uuid.UUID, active bool) (*domain.Store, error) {
	store, err := a.store.SetStoreActive(ctx, userID, storeID, active)
	if err != nil {
		return nil, mapStoreError(err, "STORE_UPDATE_FAILED", "unable to update store")
	}
	return store, nil
}

// CreateStore membuat toko untuk akun pemilik toko yang belum punya toko.
func (a *App) CreateStore(ctx context.Context, userID uuid.UUID, input StoreInput) (*domain.Store, error) {
	if _, err := a.store.StoreByOwnerUserID(ctx, userID); err == nil {
		return nil, apperror.Conflict("STORE_EXISTS", "this account already has a store")
	}

	fields, normalizedWhatsApp := validateStoreInput(input)
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}

	now := a.now()
	store := domain.Store{
		ID:          uuid.New(),
		OwnerUserID: userID,
		Name:        strings.TrimSpace(input.Name),
		Category:    strings.TrimSpace(input.Category),
		Address:     strings.TrimSpace(input.Address),
		WhatsApp:    normalizedWhatsApp,
		Latitude:    input.Latitude,
		Longitude:   input.Longitude,
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if description := strings.TrimSpace(input.Description); description != "" {
		store.Description = &description
	}
	if photoURL := strings.TrimSpace(input.PhotoURL); imageupload.IsUploadedPath(photoURL) {
		store.PhotoURL = &photoURL
	}

	created, err := a.store.CreateStore(ctx, store)
	if err != nil {
		return nil, mapStoreError(err, "STORE_CREATE_FAILED", "unable to create store")
	}
	return created, nil
}

func validateStoreInput(input StoreInput) (map[string]string, string) {
	fields := make(map[string]string)
	if !domain.IsValidName(strings.TrimSpace(input.Name)) {
		fields["name"] = "must be between 2 and 120 characters"
	}
	category := strings.TrimSpace(input.Category)
	if category == "" {
		category = "lainnya"
	}
	if !domain.IsValidStoreCategory(category) {
		fields["category"] = "must be one of " + strings.Join(domain.StoreCategories, ", ")
	}
	if !domain.IsValidDescription(input.Description) {
		fields["description"] = "must be at most 1000 characters"
	}
	if !domain.IsValidAddress(strings.TrimSpace(input.Address)) {
		fields["address"] = "must be between 3 and 300 characters"
	}
	whatsApp, valid := domain.NormalizeWhatsApp(strings.TrimSpace(input.WhatsApp))
	if !valid {
		fields["whatsapp"] = "must be a valid Indonesian whatsapp number"
	}
	if !domain.IsValidLatitude(input.Latitude) {
		fields["latitude"] = "must be between -90 and 90"
	}
	if !domain.IsValidLongitude(input.Longitude) {
		fields["longitude"] = "must be between -180 and 180"
	}
	if len(strings.TrimSpace(input.PhotoURL)) > 500 {
		fields["photo_url"] = "must be at most 500 characters"
	}
	return fields, whatsApp
}

// StoreAccountType mengembalikan persona akun pada token.
func StoreAccountType(principal identity.Principal) identity.AccountType {
	if principal.AccountType == "" {
		return identity.AccountTypePenitip
	}
	return principal.AccountType
}
