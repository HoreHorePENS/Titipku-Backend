package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/titipku/titipku-backend/internal/platform/apperror"
	"github.com/titipku/titipku-backend/internal/platform/identity"
	"github.com/titipku/titipku-backend/services/order-service/internal/domain"
)

type CreateInput struct {
	CustomerID     uuid.UUID
	CustomerRole   identity.Role
	ItemName       string
	Description    string
	PickupAddress  string
	DropoffAddress string
	ItemPrice      int64
	ServiceFee     int64
}

type UpdateStatusInput struct {
	OrderID    uuid.UUID
	ActorID    uuid.UUID
	ActorRole  identity.Role
	NextStatus domain.Status
}

type Service struct {
	orders OrderRepository
	now    func() time.Time
}

func NewService(orders OrderRepository, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}

	return &Service{
		orders: orders,
		now:    now,
	}
}

func (s *Service) Create(ctx context.Context, input CreateInput) (*domain.Order, error) {
	if input.CustomerRole != identity.RoleCustomer {
		return nil, apperror.Forbidden("CUSTOMER_ONLY", "only customers can create orders")
	}

	draft := domain.NewOrderInput{
		ItemName:       input.ItemName,
		Description:    input.Description,
		PickupAddress:  input.PickupAddress,
		DropoffAddress: input.DropoffAddress,
		ItemPrice:      input.ItemPrice,
		ServiceFee:     input.ServiceFee,
	}
	if fields := draft.Validate(); len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}

	order, err := domain.NewOrder(uuid.New(), input.CustomerID, draft, s.now())
	if err != nil {
		return nil, apperror.Internal("ORDER_CREATION_FAILED", "unable to create order").WithCause(err)
	}

	if err := s.orders.Create(ctx, &order); err != nil {
		return nil, apperror.Internal("ORDER_CREATION_FAILED", "unable to create order").WithCause(err)
	}

	return &order, nil
}

func (s *Service) Get(ctx context.Context, orderID, actorID uuid.UUID, actorRole identity.Role) (*domain.Order, error) {
	order, err := s.orders.ByID(ctx, orderID)
	if err != nil {
		if errors.Is(err, domain.ErrOrderNotFound) {
			return nil, apperror.NotFound("ORDER_NOT_FOUND", "order not found")
		}
		return nil, apperror.Internal("ORDER_LOOKUP_FAILED", "unable to load order").WithCause(err)
	}

	if !canView(order, actorID, actorRole) {
		return nil, apperror.Forbidden("ORDER_ACCESS_DENIED", "you do not have access to this order")
	}

	return order, nil
}

func (s *Service) List(
	ctx context.Context,
	actorID uuid.UUID,
	actorRole identity.Role,
	limit, offset int,
) ([]domain.Order, int64, error) {
	if limit < 1 || limit > 100 {
		return nil, 0, apperror.Validation(map[string]string{"limit": "must be between 1 and 100"})
	}
	if offset < 0 {
		return nil, 0, apperror.Validation(map[string]string{"offset": "must not be negative"})
	}

	filter := ListFilter{
		Limit:  limit,
		Offset: offset,
	}

	switch actorRole {
	case identity.RoleCustomer:
		filter.CustomerID = &actorID
	case identity.RoleCourier:
		filter.CourierID = &actorID
	case identity.RoleAdmin:
	default:
		return nil, 0, apperror.Forbidden(apperror.CodeForbidden, "you do not have permission to list orders")
	}

	orders, total, err := s.orders.List(ctx, filter)
	if err != nil {
		return nil, 0, apperror.Internal("ORDER_LIST_FAILED", "unable to list orders").WithCause(err)
	}

	return orders, total, nil
}

func (s *Service) UpdateStatus(ctx context.Context, input UpdateStatusInput) (*domain.Order, error) {
	if !input.NextStatus.Valid() || input.NextStatus == domain.StatusPending {
		return nil, apperror.Validation(map[string]string{
			"status": "must be one of accepted, picked_up, delivered, cancelled",
		})
	}

	order, err := s.orders.ByID(ctx, input.OrderID)
	if err != nil {
		if errors.Is(err, domain.ErrOrderNotFound) {
			return nil, apperror.NotFound("ORDER_NOT_FOUND", "order not found")
		}
		return nil, apperror.Internal("ORDER_LOOKUP_FAILED", "unable to load order").WithCause(err)
	}

	now := s.now()
	previousStatus := order.Status
	var transitionErr error

	switch input.ActorRole {
	case identity.RoleAdmin:
		if input.NextStatus == domain.StatusAccepted {
			return nil, apperror.Forbidden("COURIER_ONLY", "only couriers can accept orders")
		}
		transitionErr = order.ChangeStatus(input.NextStatus, now)

	case identity.RoleCourier:
		if input.NextStatus == domain.StatusAccepted {
			transitionErr = order.AssignCourier(input.ActorID, now)
			break
		}
		if order.CourierID == nil || *order.CourierID != input.ActorID {
			return nil, apperror.Forbidden("ORDER_NOT_ASSIGNED", "order is not assigned to you")
		}
		transitionErr = order.ChangeStatus(input.NextStatus, now)

	case identity.RoleCustomer:
		if order.CustomerID != input.ActorID {
			return nil, apperror.Forbidden("ORDER_ACCESS_DENIED", "you do not have access to this order")
		}
		if input.NextStatus != domain.StatusCancelled {
			return nil, apperror.Forbidden("CANCELLATION_ONLY", "customers can only cancel orders")
		}
		transitionErr = order.ChangeStatus(input.NextStatus, now)

	default:
		return nil, apperror.Forbidden(apperror.CodeForbidden, "you do not have permission to update this order")
	}

	if transitionErr != nil {
		switch {
		case errors.Is(transitionErr, domain.ErrInvalidTransition):
			return nil, apperror.Conflict(
				"INVALID_STATUS_TRANSITION",
				fmt.Sprintf("cannot change status from %s to %s", order.Status, input.NextStatus),
			)
		case errors.Is(transitionErr, domain.ErrOrderNotAvailable):
			return nil, apperror.Conflict("ORDER_NOT_AVAILABLE", "order is no longer available")
		case errors.Is(transitionErr, domain.ErrInvalidStatus):
			return nil, apperror.Validation(map[string]string{"status": "is not a valid status"})
		default:
			return nil, apperror.Internal("ORDER_UPDATE_FAILED", "unable to update order").WithCause(transitionErr)
		}
	}

	if err := s.orders.Update(ctx, order, previousStatus); err != nil {
		switch {
		case errors.Is(err, ErrOrderConflict):
			return nil, apperror.Conflict("ORDER_CONFLICT", "order was modified by another request")
		case errors.Is(err, domain.ErrOrderNotFound):
			return nil, apperror.NotFound("ORDER_NOT_FOUND", "order not found")
		default:
			return nil, apperror.Internal("ORDER_UPDATE_FAILED", "unable to update order").WithCause(err)
		}
	}

	return order, nil
}

func canView(order *domain.Order, actorID uuid.UUID, actorRole identity.Role) bool {
	switch actorRole {
	case identity.RoleAdmin:
		return true
	case identity.RoleCustomer:
		return order.CustomerID == actorID
	case identity.RoleCourier:
		return order.CourierID != nil && *order.CourierID == actorID
	default:
		return false
	}
}
