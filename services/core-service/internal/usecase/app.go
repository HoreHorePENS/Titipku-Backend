package usecase

import (
	"errors"
	"time"

	"github.com/titipku/titipku-backend/internal/platform/apperror"
	"github.com/titipku/titipku-backend/services/core-service/internal/domain"
)

type App struct {
	store  Store
	hasher PasswordHasher
	tokens TokenIssuer
	now    func() time.Time

	images        ImageStore
	maxImageBytes int64
}

func New(store Store, hasher PasswordHasher, tokens TokenIssuer, now func() time.Time) *App {
	if now == nil {
		now = time.Now
	}

	return &App{
		store:  store,
		hasher: hasher,
		tokens: tokens,
		now:    now,
	}
}

func mapStoreError(err error, code, message string) error {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return apperror.NotFound("NOT_FOUND", "requested resource was not found")
	case errors.Is(err, domain.ErrEmailTaken):
		return apperror.Conflict("EMAIL_TAKEN", "email is already registered")
	case errors.Is(err, domain.ErrWhatsAppTaken):
		return apperror.Conflict("WHATSAPP_TAKEN", "whatsapp number is already registered")
	case errors.Is(err, domain.ErrConflict):
		return apperror.Conflict("CONFLICT", "requested change conflicts with existing data")
	case errors.Is(err, domain.ErrIdempotencyMismatch):
		return apperror.Conflict("IDEMPOTENCY_MISMATCH", "idempotency key was already used with a different payload")
	case errors.Is(err, domain.ErrStockMismatch):
		return apperror.Conflict("STOCK_MISMATCH", "stock at partner no longer matches the submitted quantities")
	case errors.Is(err, domain.ErrPaymentExceedsReceivable):
		return apperror.Conflict("PAYMENT_EXCEEDS_RECEIVABLE", "payment exceeds the partner receivable")
	case errors.Is(err, domain.ErrPaymentAllocation):
		return apperror.Conflict("PAYMENT_ALLOCATION_FAILED", "payment could not be fully allocated")
	case errors.Is(err, domain.ErrInconsistentData):
		return apperror.Conflict("INCONSISTENT_DATA", "stored data is inconsistent")
	case errors.Is(err, domain.ErrStoreNotOwned):
		return apperror.NotFound("STORE_NOT_FOUND", "requested store was not found")
	case errors.Is(err, domain.ErrProposalPending):
		return apperror.Conflict("PROPOSAL_PENDING", "a pending proposal already exists between this store and producer")
	case errors.Is(err, domain.ErrAlreadyPartners):
		return apperror.Conflict("ALREADY_PARTNERS", "this store and producer are already partners")
	case errors.Is(err, domain.ErrProposalNotPending):
		return apperror.Conflict("PROPOSAL_NOT_PENDING", "proposal is no longer pending")
	case errors.Is(err, domain.ErrStoreExists):
		return apperror.Conflict("STORE_EXISTS", "this account already has a store")
	default:
		return apperror.Internal(code, message).WithCause(err)
	}
}
