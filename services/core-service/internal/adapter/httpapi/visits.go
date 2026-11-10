package httpapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/titipku/titipku-backend/internal/platform/apperror"
	"github.com/titipku/titipku-backend/internal/platform/httpx"
	"github.com/titipku/titipku-backend/services/core-service/internal/usecase"
)

type visitItemRequest struct {
	ProductID uuid.UUID `json:"product_id"`
	Remaining int64     `json:"remaining"`
	Returned  int64     `json:"returned"`
	NewDrop   int64     `json:"new_drop"`
}

type visitPaymentRequest struct {
	ClientRequestID uuid.UUID  `json:"client_request_id"`
	Amount          int64      `json:"amount"`
	Method          string     `json:"method"`
	PaidAt          *time.Time `json:"paid_at"`
}

type createVisitRequest struct {
	ClientRequestID uuid.UUID            `json:"client_request_id"`
	PartnerID       uuid.UUID            `json:"partner_id"`
	VisitedAt       *time.Time           `json:"visited_at"`
	ReceivedBy      string               `json:"received_by"`
	Items           []visitItemRequest   `json:"items"`
	Payment         *visitPaymentRequest `json:"payment"`
}

type createStoreVisitRequest struct {
	ClientRequestID string               `json:"client_request_id"`
	ProposalID      string               `json:"proposal_id"`
	BusinessID      string               `json:"business_id"`
	VisitedAt       *time.Time           `json:"visited_at"`
	ReceivedBy      string               `json:"received_by"`
	Items           []visitItemRequest   `json:"items"`
	Payment         *visitPaymentRequest `json:"payment"`
}

// CreateStoreVisit: POST /v1/store-visits (pemilik toko membuat nota untuk penitip)
func (h *Handler) CreateStoreVisit(w http.ResponseWriter, r *http.Request) error {
	actor, err := h.actor(r)
	if err != nil {
		return err
	}

	var request createStoreVisitRequest
	if err := httpx.DecodeJSON(w, r, &request); err != nil {
		return err
	}

	fields := map[string]string{}
	clientRequestID := optionalUUID(request.ClientRequestID, "client_request_id", fields)
	if clientRequestID == uuid.Nil {
		clientRequestID = uuid.New()
	}
	proposalID := optionalUUID(request.ProposalID, "proposal_id", fields)
	businessID := optionalUUID(request.BusinessID, "business_id", fields)

	items := make([]usecase.VisitItemInput, 0, len(request.Items))
	for _, item := range request.Items {
		items = append(items, usecase.VisitItemInput{
			ProductID: item.ProductID,
			Remaining: item.Remaining,
			Returned:  item.Returned,
			NewDrop:   item.NewDrop,
		})
	}

	var payment *usecase.VisitPaymentInput
	if request.Payment != nil && request.Payment.Amount > 0 {
		paymentClientRequestID := request.Payment.ClientRequestID
		if paymentClientRequestID == uuid.Nil {
			paymentClientRequestID = uuid.New()
		}
		payment = &usecase.VisitPaymentInput{
			ClientRequestID: paymentClientRequestID,
			Amount:          request.Payment.Amount,
			Method:          request.Payment.Method,
			PaidAt:          request.Payment.PaidAt,
		}
	}

	if len(fields) > 0 {
		return apperror.Validation(fields)
	}

	visit, err := h.app.CreateStoreVisit(r.Context(), actor, usecase.CreateStoreVisitInput{
		ClientRequestID: clientRequestID,
		ProposalID:      proposalID,
		BusinessID:      businessID,
		VisitedAt:       request.VisitedAt,
		ReceivedBy:      request.ReceivedBy,
		Items:           items,
		Payment:         payment,
	})
	if err != nil {
		return err
	}

	httpx.WriteData(w, r, http.StatusCreated, h.newVisitResponse(*visit))
	return nil
}

// ListStoreVisits: GET /v1/store-visits (riwayat nota toko untuk pemilik toko)
func (h *Handler) ListStoreVisits(w http.ResponseWriter, r *http.Request) error {
	actor, err := h.actor(r)
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

	visits, total, err := h.app.ListStoreVisits(r.Context(), actor, limit, offset)
	if err != nil {
		return err
	}

	httpx.WritePage(w, r, http.StatusOK, h.newVisitResponses(visits), total, limit, offset)
	return nil
}

func (h *Handler) CreateVisit(w http.ResponseWriter, r *http.Request) error {
	businessID, memberID, err := h.scope(r)
	if err != nil {
		return err
	}

	var request createVisitRequest
	if err := httpx.DecodeJSON(w, r, &request); err != nil {
		return err
	}

	items := make([]usecase.VisitItemInput, 0, len(request.Items))
	for _, item := range request.Items {
		items = append(items, usecase.VisitItemInput{
			ProductID: item.ProductID,
			Remaining: item.Remaining,
			Returned:  item.Returned,
			NewDrop:   item.NewDrop,
		})
	}

	var payment *usecase.VisitPaymentInput
	if request.Payment != nil {
		payment = &usecase.VisitPaymentInput{
			ClientRequestID: request.Payment.ClientRequestID,
			Amount:          request.Payment.Amount,
			Method:          request.Payment.Method,
			PaidAt:          request.Payment.PaidAt,
		}
	}

	visit, err := h.app.CreateVisit(r.Context(), businessID, memberID, usecase.CreateVisitInput{
		ClientRequestID: request.ClientRequestID,
		PartnerID:       request.PartnerID,
		VisitedAt:       request.VisitedAt,
		ReceivedBy:      request.ReceivedBy,
		Items:           items,
		Payment:         payment,
	})
	if err != nil {
		return err
	}

	httpx.WriteData(w, r, http.StatusCreated, h.newVisitResponse(*visit))
	return nil
}

func (h *Handler) ListVisits(w http.ResponseWriter, r *http.Request) error {
	actor, err := h.actor(r)
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

	if actor.IsStoreOwner() {
		visits, total, err := h.app.ListStoreVisits(r.Context(), actor, limit, offset)
		if err != nil {
			return err
		}
		httpx.WritePage(w, r, http.StatusOK, h.newVisitResponses(visits), total, limit, offset)
		return nil
	}

	businessID, _, err := h.scope(r)
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

	visits, total, err := h.app.ListVisits(r.Context(), businessID, usecase.VisitFilter{
		PartnerID: partnerID,
		Limit:     limit,
		Offset:    offset,
	})
	if err != nil {
		return err
	}

	httpx.WritePage(w, r, http.StatusOK, h.newVisitResponses(visits), total, limit, offset)
	return nil
}

func (h *Handler) GetVisit(w http.ResponseWriter, r *http.Request) error {
	businessID, _, err := h.scope(r)
	if err != nil {
		return err
	}

	visitID, err := pathUUID(r, "visitID")
	if err != nil {
		return err
	}

	visit, err := h.app.Visit(r.Context(), businessID, visitID)
	if err != nil {
		return err
	}

	httpx.WriteData(w, r, http.StatusOK, h.newVisitResponse(*visit))
	return nil
}
