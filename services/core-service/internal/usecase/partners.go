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

	fields := make(map[string]string)
	if !domain.IsValidName(name) {
		fields["name"] = "must be between 2 and 120 characters"
	}
	if !domain.IsValidName(ownerName) {
		fields["owner_name"] = "must be between 2 and 100 characters"
	}

	whatsApp, valid := domain.NormalizeWhatsApp(input.WhatsApp)
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

	var address *string
	if value := strings.TrimSpace(input.Address); value != "" {
		address = &value
	}

	partner, err := a.store.UpsertPartner(ctx, domain.Partner{
		ID:         id,
		BusinessID: businessID,
		Name:       name,
		OwnerName:  ownerName,
		WhatsApp:   whatsApp,
		Address:    address,
		IsActive:   isActive,
	})
	if err != nil {
		return nil, mapStoreError(err, "PARTNER_WRITE_FAILED", "unable to save partner")
	}
	return partner, nil
}

func (a *App) SetPartnerActive(ctx context.Context, businessID, partnerID uuid.UUID, active bool) (*domain.Partner, error) {
	partner, err := a.store.SetPartnerActive(ctx, businessID, partnerID, active)
	if err != nil {
		return nil, mapStoreError(err, "PARTNER_UPDATE_FAILED", "unable to update partner")
	}
	return partner, nil
}
