package usecase

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/titipku/titipku-backend/internal/platform/apperror"
	"github.com/titipku/titipku-backend/services/core-service/internal/domain"
)

type PartnerInput struct {
	ID        uuid.UUID
	Name      string
	OwnerName string
	WhatsApp  string
	Address   string
	IsActive  *bool
	// StoreID diisi bila partner berasal dari katalog toko pada peta.
	StoreID *uuid.UUID
}

func (a *App) ListPartners(ctx context.Context, businessID uuid.UUID, activeOnly bool) ([]domain.Partner, error) {
	partners, err := a.store.ListPartners(ctx, businessID, activeOnly)
	if err != nil {
		return nil, mapStoreError(err, "PARTNER_LIST_FAILED", "unable to load partners")
	}
	return partners, nil
}

func (a *App) UpsertPartner(ctx context.Context, businessID uuid.UUID, input PartnerInput) (*domain.Partner, error) {
	name := strings.TrimSpace(input.Name)
	ownerName := strings.TrimSpace(input.OwnerName)
	address := strings.TrimSpace(input.Address)
	whatsApp := strings.TrimSpace(input.WhatsApp)

	// Saat berasal dari katalog toko, data toko menjadi sumber utama.
	var storeID *uuid.UUID
	if input.StoreID != nil && *input.StoreID != uuid.Nil {
		store, err := a.store.StoreByID(ctx, *input.StoreID, &businessID)
		if err != nil {
			return nil, mapStoreError(err, "STORE_NOT_FOUND", "store was not found")
		}
		if !store.IsActive {
			return nil, apperror.Conflict("STORE_INACTIVE", "store is no longer accepting consignments")
		}
		id := store.ID
		storeID = &id

		name = store.Name
		whatsApp = store.WhatsApp
		if store.Address != "" {
			address = store.Address
		}
		// owner_name: pakai nama pemilik yang dikirim klien, bila kosong pakai nama toko.
		if ownerName == "" {
			ownerName = truncateRunes(store.Name, 100)
		}
	}

	fields := make(map[string]string)
	if !domain.IsValidName(name) {
		fields["name"] = "must be between 2 and 120 characters"
	}
	if !domain.IsValidName(ownerName) {
		fields["owner_name"] = "must be between 2 and 100 characters"
	}

	normalizedWhatsApp, valid := domain.NormalizeWhatsApp(whatsApp)
	if !valid {
		fields["whatsapp"] = "must be a valid Indonesian whatsapp number"
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

	var storedAddress *string
	if address != "" {
		storedAddress = &address
	}

	partner, err := a.store.UpsertPartner(ctx, domain.Partner{
		ID:         id,
		BusinessID: businessID,
		Name:       name,
		OwnerName:  ownerName,
		WhatsApp:   normalizedWhatsApp,
		Address:    storedAddress,
		StoreID:    storeID,
		IsActive:   isActive,
	})
	if err != nil {
		return nil, mapStoreError(err, "PARTNER_WRITE_FAILED", "unable to save partner")
	}
	return partner, nil
}

// truncateRunes memotong teks agar muat pada batas karakter tertentu.
func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

func (a *App) SetPartnerActive(ctx context.Context, businessID, partnerID uuid.UUID, active bool) (*domain.Partner, error) {
	partner, err := a.store.SetPartnerActive(ctx, businessID, partnerID, active)
	if err != nil {
		return nil, mapStoreError(err, "PARTNER_UPDATE_FAILED", "unable to update partner")
	}
	return partner, nil
}
