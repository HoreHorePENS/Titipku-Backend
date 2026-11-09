package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/titipku/titipku-backend/internal/platform/apperror"
	"github.com/titipku/titipku-backend/internal/platform/httpx"
	"github.com/titipku/titipku-backend/internal/platform/identity"
	"github.com/titipku/titipku-backend/services/core-service/internal/domain"
	"github.com/titipku/titipku-backend/services/core-service/internal/usecase"
)

type proposalItemRequest struct {
	ProductID string `json:"product_id"`
	Qty       int32  `json:"qty"`
}

type createProposalRequest struct {
	// Pemilik toko mengisi business_id; penitip mengisi store_id (+ items opsional).
	BusinessID        string                `json:"business_id"`
	StoreID           string                `json:"store_id"`
	Message           string                `json:"message"`
	ShelfLocation     string                `json:"shelf_location"`
	EstimatedCapacity string                `json:"estimated_capacity"`
	Items             []proposalItemRequest `json:"items"`
}

type proposalStoreResponse struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Category  string  `json:"category"`
	Address   string  `json:"address"`
	WhatsApp  string  `json:"whatsapp"`
	PhotoURL  *string `json:"photo_url"`
	OwnerName string  `json:"owner_name"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type proposalBusinessResponse struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	OwnerName string  `json:"owner_name"`
	WhatsApp  *string `json:"whatsapp"`
	Address   *string `json:"address"`
}

type proposalItemResponse struct {
	ProductID     string  `json:"product_id"`
	Name          string  `json:"name"`
	PhotoPath     *string `json:"photo_path"`
	ConsignPrice  int64   `json:"consign_price"`
	RetailPrice   int64   `json:"retail_price"`
	ShelfLifeDays int32   `json:"shelf_life_days"`
	Qty           int32   `json:"qty"`
}

type proposalResponse struct {
	ID                string                   `json:"id"`
	Direction         string                   `json:"direction"`
	Status            string                   `json:"status"`
	Message           *string                  `json:"message"`
	ShelfLocation     *string                  `json:"shelf_location"`
	EstimatedCapacity *string                  `json:"estimated_capacity"`
	CreatedAt         time.Time                `json:"created_at"`
	RespondedAt       *time.Time               `json:"responded_at"`
	Store             proposalStoreResponse    `json:"store"`
	Business          proposalBusinessResponse `json:"business"`
	Items             []proposalItemResponse   `json:"items"`
}

type producerResponse struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	OwnerName      string            `json:"owner_name"`
	WhatsApp       *string           `json:"whatsapp"`
	Address        *string           `json:"address"`
	ProductCount   int               `json:"product_count"`
	Products       []productResponse `json:"products"`
	IsPartner      bool              `json:"is_partner"`
	LatestProposal *proposalResponse `json:"latest_proposal"`
}

func newProposalResponse(p domain.Proposal) proposalResponse {
	items := make([]proposalItemResponse, 0, len(p.Items))
	for _, item := range p.Items {
		items = append(items, proposalItemResponse{
			ProductID:     item.ProductID.String(),
			Name:          item.ProductName,
			PhotoPath:     item.PhotoPath,
			ConsignPrice:  item.ConsignPrice,
			RetailPrice:   item.RetailPrice,
			ShelfLifeDays: item.ShelfLifeDays,
			Qty:           item.Qty,
		})
	}
	return proposalResponse{
		ID:                p.ID.String(),
		Direction:         p.Direction,
		Status:            p.Status,
		Message:           p.Message,
		ShelfLocation:     p.ShelfLocation,
		EstimatedCapacity: p.EstimatedCapacity,
		CreatedAt:         p.CreatedAt,
		RespondedAt:       p.RespondedAt,
		Store: proposalStoreResponse{
			ID:        p.StoreID.String(),
			Name:      p.StoreName,
			Category:  p.StoreCategory,
			Address:   p.StoreAddress,
			WhatsApp:  p.StoreWhatsApp,
			PhotoURL:  p.StorePhotoURL,
			OwnerName: p.StoreOwnerName,
			Latitude:  p.StoreLatitude,
			Longitude: p.StoreLongitude,
		},
		Business: proposalBusinessResponse{
			ID:        p.BusinessID.String(),
			Name:      p.BusinessName,
			OwnerName: p.BusinessOwnerName,
			WhatsApp:  p.BusinessWhatsApp,
			Address:   p.BusinessAddress,
		},
		Items: items,
	}
}

// actor membangun identitas pemanggil dari token.
func (h *Handler) actor(r *http.Request) (usecase.Actor, error) {
	principal, err := httpx.PrincipalFromRequest(r)
	if err != nil {
		return usecase.Actor{}, err
	}
	actor := usecase.Actor{UserID: principal.UserID, AccountType: principalAccountType(r)}
	if actor.AccountType == identity.AccountTypePenitip {
		businessID, _, err := h.scope(r)
		if err != nil {
			return usecase.Actor{}, err
		}
		actor.BusinessID = businessID
	}
	return actor, nil
}

func optionalUUID(value, field string, fields map[string]string) uuid.UUID {
	if value == "" {
		return uuid.Nil
	}
	parsed, err := uuid.Parse(value)
	if err != nil {
		fields[field] = "must be a valid UUID"
		return uuid.Nil
	}
	return parsed
}

// CreateProposal: POST /v1/proposals
func (h *Handler) CreateProposal(w http.ResponseWriter, r *http.Request) error {
	actor, err := h.actor(r)
	if err != nil {
		return err
	}
	var request createProposalRequest
	if err := httpx.DecodeJSON(w, r, &request); err != nil {
		return err
	}

	fields := map[string]string{}
	input := usecase.CreateProposalInput{
		BusinessID:        optionalUUID(request.BusinessID, "business_id", fields),
		StoreID:           optionalUUID(request.StoreID, "store_id", fields),
		Message:           request.Message,
		ShelfLocation:     request.ShelfLocation,
		EstimatedCapacity: request.EstimatedCapacity,
	}
	for _, item := range request.Items {
		input.Items = append(input.Items, usecase.ProposalItemInput{
			ProductID: optionalUUID(item.ProductID, "items", fields),
			Qty:       item.Qty,
		})
	}
	if len(fields) > 0 {
		return apperror.Validation(fields)
	}

	proposal, err := h.app.CreateProposal(r.Context(), actor, input)
	if err != nil {
		return err
	}
	httpx.WriteData(w, r, http.StatusCreated, newProposalResponse(*proposal))
	return nil
}

// ListProposals: GET /v1/proposals?direction=&status=
func (h *Handler) ListProposals(w http.ResponseWriter, r *http.Request) error {
	actor, err := h.actor(r)
	if err != nil {
		return err
	}
	proposals, err := h.app.ListProposals(r.Context(), actor, usecase.ProposalListInput{
		Direction: r.URL.Query().Get("direction"),
		Status:    r.URL.Query().Get("status"),
	})
	if err != nil {
		return err
	}
	responses := make([]proposalResponse, 0, len(proposals))
	for _, proposal := range proposals {
		responses = append(responses, newProposalResponse(proposal))
	}
	httpx.WriteData(w, r, http.StatusOK, responses)
	return nil
}

func (h *Handler) resolveProposal(w http.ResponseWriter, r *http.Request, action string) error {
	actor, err := h.actor(r)
	if err != nil {
		return err
	}
	proposalID, err := pathUUID(r, "proposalID")
	if err != nil {
		return err
	}

	var proposal *domain.Proposal
	switch action {
	case "accept":
		proposal, err = h.app.RespondProposal(r.Context(), actor, proposalID, true)
	case "reject":
		proposal, err = h.app.RespondProposal(r.Context(), actor, proposalID, false)
	default:
		proposal, err = h.app.CancelProposal(r.Context(), actor, proposalID)
	}
	if err != nil {
		return err
	}
	httpx.WriteData(w, r, http.StatusOK, newProposalResponse(*proposal))
	return nil
}

func (h *Handler) AcceptProposal(w http.ResponseWriter, r *http.Request) error {
	return h.resolveProposal(w, r, "accept")
}

func (h *Handler) RejectProposal(w http.ResponseWriter, r *http.Request) error {
	return h.resolveProposal(w, r, "reject")
}

func (h *Handler) CancelProposal(w http.ResponseWriter, r *http.Request) error {
	return h.resolveProposal(w, r, "cancel")
}

// ListProducers: GET /v1/producers?q=&limit=&offset= (pemilik toko)
func (h *Handler) ListProducers(w http.ResponseWriter, r *http.Request) error {
	actor, err := h.actor(r)
	if err != nil {
		return err
	}
	input := usecase.ProducerListInput{Search: r.URL.Query().Get("q")}
	if value, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil {
		input.Limit = value
	}
	if value, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil {
		input.Offset = value
	}

	producers, err := h.app.ListProducers(r.Context(), actor, input)
	if err != nil {
		return err
	}
	responses := make([]producerResponse, 0, len(producers))
	for _, producer := range producers {
		response := producerResponse{
			ID:           producer.BusinessID.String(),
			Name:         producer.Name,
			OwnerName:    producer.OwnerName,
			WhatsApp:     producer.WhatsApp,
			Address:      producer.Address,
			ProductCount: producer.ProductCount,
			Products:     newProductResponses(producer.Products),
			IsPartner:    producer.IsPartner,
		}
		if producer.LatestProposal != nil {
			latest := newProposalResponse(*producer.LatestProposal)
			response.LatestProposal = &latest
		}
		responses = append(responses, response)
	}
	httpx.WriteData(w, r, http.StatusOK, responses)
	return nil
}
