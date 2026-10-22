package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/titipku/titipku-backend/internal/platform/apperror"
	"github.com/titipku/titipku-backend/internal/platform/httpx"
	"github.com/titipku/titipku-backend/services/order-service/internal/domain"
	"github.com/titipku/titipku-backend/services/order-service/internal/usecase"
)

type Handler struct {
	orders *usecase.Service
}

func NewHandler(orders *usecase.Service) *Handler {
	return &Handler{orders: orders}
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) error {
	principal, err := httpx.PrincipalFromRequest(r)
	if err != nil {
		return err
	}

	var request createOrderRequest
	if err := httpx.DecodeJSON(w, r, &request); err != nil {
		return err
	}
	if request.ItemPrice == nil {
		return apperror.Validation(map[string]string{"item_price": "is required"})
	}

	serviceFee := int64(0)
	if request.ServiceFee != nil {
		serviceFee = *request.ServiceFee
	}

	order, err := h.orders.Create(r.Context(), usecase.CreateInput{
		CustomerID:     principal.UserID,
		CustomerRole:   principal.Role,
		ItemName:       request.ItemName,
		Description:    request.Description,
		PickupAddress:  request.PickupAddress,
		DropoffAddress: request.DropoffAddress,
		ItemPrice:      *request.ItemPrice,
		ServiceFee:     serviceFee,
	})
	if err != nil {
		return err
	}

	httpx.WriteData(w, r, http.StatusCreated, newOrderResponse(*order))
	return nil
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) error {
	principal, err := httpx.PrincipalFromRequest(r)
	if err != nil {
		return err
	}

	orderID, err := orderIDFromRequest(r)
	if err != nil {
		return err
	}

	order, err := h.orders.Get(r.Context(), orderID, principal.UserID, principal.Role)
	if err != nil {
		return err
	}

	httpx.WriteData(w, r, http.StatusOK, newOrderResponse(*order))
	return nil
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) error {
	principal, err := httpx.PrincipalFromRequest(r)
	if err != nil {
		return err
	}

	limit, err := httpx.QueryInt(r, "limit", 20, 1, 100)
	if err != nil {
		return err
	}
	offset, err := httpx.QueryInt(r, "offset", 0, 0, 1_000_000)
	if err != nil {
		return err
	}

	orders, total, err := h.orders.List(r.Context(), principal.UserID, principal.Role, limit, offset)
	if err != nil {
		return err
	}

	httpx.WritePage(w, r, http.StatusOK, newOrderResponses(orders), total, limit, offset)
	return nil
}

func (h *Handler) UpdateStatus(w http.ResponseWriter, r *http.Request) error {
	principal, err := httpx.PrincipalFromRequest(r)
	if err != nil {
		return err
	}

	orderID, err := orderIDFromRequest(r)
	if err != nil {
		return err
	}

	var request updateStatusRequest
	if err := httpx.DecodeJSON(w, r, &request); err != nil {
		return err
	}

	order, err := h.orders.UpdateStatus(r.Context(), usecase.UpdateStatusInput{
		OrderID:    orderID,
		ActorID:    principal.UserID,
		ActorRole:  principal.Role,
		NextStatus: domain.Status(request.Status),
	})
	if err != nil {
		return err
	}

	httpx.WriteData(w, r, http.StatusOK, newOrderResponse(*order))
	return nil
}

func orderIDFromRequest(r *http.Request) (uuid.UUID, error) {
	orderID, err := uuid.Parse(chi.URLParam(r, "orderID"))
	if err != nil {
		return uuid.Nil, apperror.Validation(map[string]string{"order_id": "must be a valid UUID"})
	}
	return orderID, nil
}
