package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/titipku/titipku-backend/internal/platform/apperror"
	"github.com/titipku/titipku-backend/services/core-service/internal/domain"
)

type VisitItemInput struct {
	ProductID uuid.UUID `json:"product_id"`
	Remaining int64     `json:"remaining"`
	Returned  int64     `json:"returned"`
	NewDrop   int64     `json:"new_drop"`
}

type VisitPaymentInput struct {
	ClientRequestID uuid.UUID  `json:"client_request_id"`
	Amount          int64      `json:"amount"`
	Method          string     `json:"method"`
	PaidAt          *time.Time `json:"paid_at,omitempty"`
}

type CreateVisitInput struct {
	ClientRequestID uuid.UUID
	PartnerID       uuid.UUID
	VisitedAt       *time.Time
	ReceivedBy      string
	Items           []VisitItemInput
	Payment         *VisitPaymentInput
}

type VisitFilter struct {
	PartnerID *uuid.UUID
	Limit     int
	Offset    int
}

func (a *App) CreateVisit(ctx context.Context, businessID, memberID uuid.UUID, input CreateVisitInput) (*domain.Visit, error) {
	fields := make(map[string]string)
	if input.ClientRequestID == uuid.Nil {
		fields["client_request_id"] = "is required"
	}
	if input.PartnerID == uuid.Nil {
		fields["partner_id"] = "is required"
	}
	if len(input.Items) == 0 {
		fields["items"] = "must contain at least one item"
	}

	seenProducts := make(map[uuid.UUID]struct{}, len(input.Items))
	for index, item := range input.Items {
		prefix := "items[" + itoa(index) + "]"
		if item.ProductID == uuid.Nil {
			fields[prefix+".product_id"] = "is required"
			continue
		}
		if _, exists := seenProducts[item.ProductID]; exists {
			fields[prefix+".product_id"] = "must not be duplicated"
		}
		seenProducts[item.ProductID] = struct{}{}
		if item.Remaining < 0 {
			fields[prefix+".remaining"] = "must not be negative"
		}
		if item.Returned < 0 {
			fields[prefix+".returned"] = "must not be negative"
		}
		if item.NewDrop < 0 {
			fields[prefix+".new_drop"] = "must not be negative"
		}
	}

	now := a.now()
	visitedAt := now
	if input.VisitedAt != nil {
		visitedAt = input.VisitedAt.UTC()
		if visitedAt.After(now.Add(5*time.Minute)) || visitedAt.Before(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)) {
			fields["visited_at"] = "must be a reasonable timestamp"
		}
	}

	receivedBy := strings.TrimSpace(input.ReceivedBy)
	if len([]rune(receivedBy)) > 100 {
		fields["received_by"] = "must not exceed 100 characters"
	}

	payment := input.Payment
	paymentHash := ""
	if payment != nil {
		if payment.Amount < 0 {
			fields["payment.amount"] = "must not be negative"
		}
		if payment.Amount > 0 && payment.ClientRequestID == uuid.Nil {
			fields["payment.client_request_id"] = "is required when payment amount is greater than zero"
		}
		if strings.TrimSpace(payment.Method) != "" && !domain.IsValidPaymentMethod(payment.Method) {
			fields["payment.method"] = "must be one of cash, transfer, qris, other"
		}
	}

	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}

	fingerprint := struct {
		ClientRequestID uuid.UUID          `json:"client_request_id"`
		PartnerID       uuid.UUID          `json:"partner_id"`
		VisitedAt       *time.Time         `json:"visited_at,omitempty"`
		ReceivedBy      string             `json:"received_by,omitempty"`
		Items           []VisitItemInput   `json:"items"`
		Payment         *VisitPaymentInput `json:"payment,omitempty"`
	}{
		ClientRequestID: input.ClientRequestID,
		PartnerID:       input.PartnerID,
		VisitedAt:       input.VisitedAt,
		ReceivedBy:      receivedBy,
		Items:           input.Items,
		Payment:         payment,
	}
	requestHash, err := hashPayload(fingerprint)
	if err != nil {
		return nil, apperror.Internal("VISIT_HASH_FAILED", "unable to process visit request").WithCause(err)
	}

	publicToken, err := domain.GeneratePublicToken()
	if err != nil {
		return nil, apperror.Internal("VISIT_TOKEN_FAILED", "unable to create public invoice token").WithCause(err)
	}

	payload := map[string]any{
		"visit_id":          uuid.NewString(),
		"invoice_id":        uuid.NewString(),
		"client_request_id": input.ClientRequestID.String(),
		"partner_id":        input.PartnerID.String(),
		"visited_at":        visitedAt.Format(time.RFC3339Nano),
		"received_by":       receivedBy,
		"public_token":      publicToken,
		"items":             input.Items,
	}

	if payment != nil && payment.Amount > 0 {
		method := strings.TrimSpace(payment.Method)
		if method == "" {
			method = "cash"
		}
		paymentHash, err = hashPayload(struct {
			ClientRequestID uuid.UUID  `json:"client_request_id"`
			PartnerID       uuid.UUID  `json:"partner_id"`
			Amount          int64      `json:"amount"`
			Method          string     `json:"method"`
			PaidAt          *time.Time `json:"paid_at,omitempty"`
		}{
			ClientRequestID: payment.ClientRequestID,
			PartnerID:       input.PartnerID,
			Amount:          payment.Amount,
			Method:          method,
			PaidAt:          payment.PaidAt,
		})
		if err != nil {
			return nil, apperror.Internal("PAYMENT_HASH_FAILED", "unable to process payment").WithCause(err)
		}

		paymentPayload := map[string]any{
			"payment_id":        uuid.NewString(),
			"client_request_id": payment.ClientRequestID.String(),
			"amount":            payment.Amount,
			"method":            method,
			"request_hash":      paymentHash,
		}
		if payment.PaidAt != nil {
			paymentPayload["paid_at"] = payment.PaidAt.UTC().Format(time.RFC3339Nano)
		}
		payload["payment"] = paymentPayload
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, apperror.Internal("VISIT_ENCODE_FAILED", "unable to process visit request").WithCause(err)
	}

	result, err := a.store.CreateVisit(ctx, businessID, memberID, encoded, requestHash)
	if err != nil {
		return nil, mapStoreError(err, "VISIT_CREATE_FAILED", "unable to save visit")
	}

	visit, err := a.store.VisitByID(ctx, businessID, result.ID)
	if err != nil {
		return nil, mapStoreError(err, "VISIT_LOOKUP_FAILED", "unable to load saved visit")
	}
	return visit, nil
}

func (a *App) ListVisits(ctx context.Context, businessID uuid.UUID, filter VisitFilter) ([]domain.Visit, int64, error) {
	visits, total, err := a.store.ListVisits(ctx, businessID, filter.PartnerID, filter.Limit, filter.Offset)
	if err != nil {
		return nil, 0, mapStoreError(err, "VISIT_LIST_FAILED", "unable to load visits")
	}
	return visits, total, nil
}

// ListStoreVisits mengambil riwayat kunjungan dan nota toko milik pemilik toko.
func (a *App) ListStoreVisits(ctx context.Context, actor Actor, limit, offset int) ([]domain.Visit, int64, error) {
	if !actor.IsStoreOwner() {
		return nil, 0, apperror.Forbidden("STORE_OWNER_REQUIRED", "only store owners can list store visits")
	}
	storeID, err := a.myStoreID(ctx, actor.UserID)
	if err != nil {
		return nil, 0, err
	}
	return a.store.ListStoreVisits(ctx, storeID, limit, offset)
}

// StoreVisit: detail satu nota dari penitip untuk toko milik pemilik toko.
func (a *App) StoreVisit(ctx context.Context, actor Actor, visitID uuid.UUID) (*domain.Visit, error) {
	if !actor.IsStoreOwner() {
		return nil, apperror.Forbidden("STORE_OWNER_REQUIRED", "only store owners can view store visits")
	}
	storeID, err := a.myStoreID(ctx, actor.UserID)
	if err != nil {
		return nil, err
	}
	visit, err := a.store.StoreVisitByID(ctx, storeID, visitID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, apperror.NotFound("VISIT_NOT_FOUND", "visit was not found for this store")
		}
		return nil, mapStoreError(err, "VISIT_LOOKUP_FAILED", "unable to load visit")
	}
	return visit, nil
}

func (a *App) Visit(ctx context.Context, businessID, visitID uuid.UUID) (*domain.Visit, error) {
	visit, err := a.store.VisitByID(ctx, businessID, visitID)
	if err != nil {
		return nil, mapStoreError(err, "VISIT_LOOKUP_FAILED", "unable to load visit")
	}
	return visit, nil
}

func hashPayload(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}

	var buffer [20]byte
	position := len(buffer)
	for value > 0 {
		position--
		buffer[position] = byte('0' + value%10)
		value /= 10
	}
	return string(buffer[position:])
}
