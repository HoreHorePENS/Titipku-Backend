package usecase

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/titipku/titipku-backend/internal/platform/apperror"
	"github.com/titipku/titipku-backend/services/core-service/internal/domain"
)

type PaymentInput struct {
	ClientRequestID uuid.UUID
	PartnerID       uuid.UUID
	Amount          int64
	Method          string
	PaidAt          *time.Time
}

type InvoiceFilter struct {
	Status    string
	PartnerID *uuid.UUID
	Limit     int
	Offset    int
}

func (a *App) ListInvoices(ctx context.Context, businessID uuid.UUID, filter InvoiceFilter) ([]domain.Invoice, int64, error) {
	status := strings.TrimSpace(filter.Status)
	if status == "" {
		status = "all"
	}
	switch status {
	case "all", "unpaid", "partial", "paid":
	default:
		return nil, 0, apperror.Validation(map[string]string{
			"status": "must be one of all, unpaid, partial, paid",
		})
	}

	invoices, total, err := a.store.ListInvoices(ctx, businessID, status, filter.PartnerID, filter.Limit, filter.Offset)
	if err != nil {
		return nil, 0, mapStoreError(err, "INVOICE_LIST_FAILED", "unable to load invoices")
	}
	return invoices, total, nil
}

func (a *App) Invoice(ctx context.Context, businessID, invoiceID uuid.UUID) (*domain.Invoice, error) {
	invoice, err := a.store.InvoiceByID(ctx, businessID, invoiceID)
	if err != nil {
		return nil, mapStoreError(err, "INVOICE_LOOKUP_FAILED", "unable to load invoice")
	}
	return invoice, nil
}

func (a *App) PublicInvoice(ctx context.Context, token string) (*domain.Invoice, error) {
	token = strings.TrimSpace(token)
	if len(token) < 20 || len(token) > 128 {
		return nil, apperror.NotFound("INVOICE_NOT_FOUND", "invoice was not found")
	}

	invoice, err := a.store.PublicInvoice(ctx, token)
	if err != nil {
		return nil, mapStoreError(err, "INVOICE_LOOKUP_FAILED", "unable to load invoice")
	}
	return invoice, nil
}

func (a *App) CreatePayment(ctx context.Context, businessID, memberID uuid.UUID, input PaymentInput) (*domain.Payment, error) {
	method := strings.TrimSpace(input.Method)
	if method == "" {
		method = "cash"
	}

	fields := make(map[string]string)
	if input.ClientRequestID == uuid.Nil {
		fields["client_request_id"] = "is required"
	}
	if input.PartnerID == uuid.Nil {
		fields["partner_id"] = "is required"
	}
	if input.Amount <= 0 {
		fields["amount"] = "must be greater than zero"
	}
	if !domain.IsValidPaymentMethod(method) {
		fields["method"] = "must be one of cash, transfer, qris, other"
	}
	if input.PaidAt != nil {
		if input.PaidAt.After(a.now().Add(5 * time.Minute)) {
			fields["paid_at"] = "must not be in the future"
		}
	}
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}

	requestHash, err := hashPayload(struct {
		ClientRequestID uuid.UUID  `json:"client_request_id"`
		PartnerID       uuid.UUID  `json:"partner_id"`
		Amount          int64      `json:"amount"`
		Method          string     `json:"method"`
		PaidAt          *time.Time `json:"paid_at,omitempty"`
	}{
		ClientRequestID: input.ClientRequestID,
		PartnerID:       input.PartnerID,
		Amount:          input.Amount,
		Method:          method,
		PaidAt:          input.PaidAt,
	})
	if err != nil {
		return nil, apperror.Internal("PAYMENT_HASH_FAILED", "unable to process payment").WithCause(err)
	}

	payload := map[string]any{
		"payment_id":        uuid.NewString(),
		"client_request_id": input.ClientRequestID.String(),
		"partner_id":        input.PartnerID.String(),
		"amount":            input.Amount,
		"method":            method,
	}
	if input.PaidAt != nil {
		payload["paid_at"] = input.PaidAt.UTC().Format(time.RFC3339Nano)
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, apperror.Internal("PAYMENT_ENCODE_FAILED", "unable to process payment").WithCause(err)
	}

	result, err := a.store.CreatePayment(ctx, businessID, memberID, encoded, requestHash)
	if err != nil {
		return nil, mapStoreError(err, "PAYMENT_CREATE_FAILED", "unable to save payment")
	}

	payment, err := a.store.PaymentByID(ctx, businessID, result.ID)
	if err != nil {
		return nil, mapStoreError(err, "PAYMENT_LOOKUP_FAILED", "unable to load saved payment")
	}
	return payment, nil
}

func (a *App) VoidPayment(ctx context.Context, businessID, memberID, paymentID uuid.UUID) (*domain.Payment, error) {
	payment, err := a.store.VoidPayment(ctx, businessID, memberID, paymentID)
	if err != nil {
		return nil, mapStoreError(err, "PAYMENT_VOID_FAILED", "unable to void payment")
	}
	return payment, nil
}
