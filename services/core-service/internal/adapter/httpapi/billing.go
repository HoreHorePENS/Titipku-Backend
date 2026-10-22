package httpapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/titipku/titipku-backend/internal/platform/apperror"
	"github.com/titipku/titipku-backend/internal/platform/httpx"
	"github.com/titipku/titipku-backend/services/core-service/internal/usecase"
)

type paymentRequest struct {
	ClientRequestID uuid.UUID  `json:"client_request_id"`
	PartnerID       uuid.UUID  `json:"partner_id"`
	Amount          int64      `json:"amount"`
	Method          string     `json:"method"`
	PaidAt          *time.Time `json:"paid_at"`
}

func (h *Handler) ListInvoices(w http.ResponseWriter, r *http.Request) error {
	businessID, _, err := h.scope(r)
	if err != nil {
		return err
	}

	limit, err := httpx.QueryInt(r, "limit", 20, 1, 100)
	if err != nil {
		return err
	}
	offset, err := httpx.QueryInt(r, "offset", 0, 0, 100000)
	if err != nil {
		return err
	}

	var partnerID *uuid.UUID
	if value := strings.TrimSpace(r.URL.Query().Get("partner_id")); value != "" {
		parsed, err := uuid.Parse(value)
		if err != nil {
			return apperror.Validation(map[string]string{"partner_id": "must be a valid UUID"})
		}
		partnerID = &parsed
	}

	invoices, total, err := h.app.ListInvoices(r.Context(), businessID, usecase.InvoiceFilter{
		Status:    r.URL.Query().Get("status"),
		PartnerID: partnerID,
		Limit:     limit,
		Offset:    offset,
	})
	if err != nil {
		return err
	}

	httpx.WritePage(w, r, http.StatusOK, h.newInvoiceResponses(invoices), total, limit, offset)
	return nil
}

func (h *Handler) GetInvoice(w http.ResponseWriter, r *http.Request) error {
	businessID, _, err := h.scope(r)
	if err != nil {
		return err
	}

	invoiceID, err := pathUUID(r, "invoiceID")
	if err != nil {
		return err
	}

	invoice, err := h.app.Invoice(r.Context(), businessID, invoiceID)
	if err != nil {
		return err
	}

	httpx.WriteData(w, r, http.StatusOK, h.newInvoiceResponse(*invoice, true))
	return nil
}

func (h *Handler) CreatePayment(w http.ResponseWriter, r *http.Request) error {
	businessID, memberID, err := h.scope(r)
	if err != nil {
		return err
	}

	var request paymentRequest
	if err := httpx.DecodeJSON(w, r, &request); err != nil {
		return err
	}

	payment, err := h.app.CreatePayment(r.Context(), businessID, memberID, usecase.PaymentInput{
		ClientRequestID: request.ClientRequestID,
		PartnerID:       request.PartnerID,
		Amount:          request.Amount,
		Method:          request.Method,
		PaidAt:          request.PaidAt,
	})
	if err != nil {
		return err
	}

	httpx.WriteData(w, r, http.StatusCreated, newPaymentResponse(*payment))
	return nil
}

func (h *Handler) VoidPayment(w http.ResponseWriter, r *http.Request) error {
	businessID, memberID, err := h.scope(r)
	if err != nil {
		return err
	}

	paymentID, err := pathUUID(r, "paymentID")
	if err != nil {
		return err
	}

	payment, err := h.app.VoidPayment(r.Context(), businessID, memberID, paymentID)
	if err != nil {
		return err
	}

	httpx.WriteData(w, r, http.StatusOK, newPaymentResponse(*payment))
	return nil
}

func (h *Handler) PublicInvoice(w http.ResponseWriter, r *http.Request) error {
	invoice, err := h.app.PublicInvoice(r.Context(), chi.URLParam(r, "token"))
	if err != nil {
		return err
	}

	w.Header().Set("Cache-Control", "no-store")
	httpx.WriteData(w, r, http.StatusOK, newPublicInvoiceResponse(*invoice))
	return nil
}
