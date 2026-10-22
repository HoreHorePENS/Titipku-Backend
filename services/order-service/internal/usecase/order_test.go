package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/titipku/titipku-backend/internal/platform/apperror"
	"github.com/titipku/titipku-backend/internal/platform/identity"
	"github.com/titipku/titipku-backend/services/order-service/internal/domain"
)

func TestCreateOrder(t *testing.T) {
	now := time.Date(2026, time.March, 3, 10, 0, 0, 0, time.UTC)
	repository := newFakeOrderRepository()
	service := NewService(repository, func() time.Time { return now })
	customerID := uuid.New()

	order, err := service.Create(context.Background(), validCreateInput(customerID))
	if err != nil {
		t.Fatalf("create returned error: %v", err)
	}

	if order.Status != domain.StatusPending {
		t.Fatalf("unexpected status: %s", order.Status)
	}
	if order.CustomerID != customerID {
		t.Fatalf("unexpected customer id: %s", order.CustomerID)
	}
	if !order.CreatedAt.Equal(now) {
		t.Fatalf("unexpected created at: %s", order.CreatedAt)
	}
	if len(repository.orders) != 1 {
		t.Fatal("order was not persisted")
	}
}

func TestCreateOrderRequiresCustomerRole(t *testing.T) {
	service := NewService(newFakeOrderRepository(), time.Now)

	input := validCreateInput(uuid.New())
	input.CustomerRole = identity.RoleCourier

	_, err := service.Create(context.Background(), input)

	appErr := requireAppError(t, err)
	if appErr.Kind != apperror.KindForbidden {
		t.Fatalf("expected forbidden kind, got %v", appErr.Kind)
	}
}

func TestCourierAcceptsPendingOrder(t *testing.T) {
	now := time.Date(2026, time.March, 3, 10, 0, 0, 0, time.UTC)
	service, order := seededService(t, now)
	courierID := uuid.New()

	updated, err := service.UpdateStatus(context.Background(), UpdateStatusInput{
		OrderID:    order.ID,
		ActorID:    courierID,
		ActorRole:  identity.RoleCourier,
		NextStatus: domain.StatusAccepted,
	})
	if err != nil {
		t.Fatalf("accept returned error: %v", err)
	}

	if updated.Status != domain.StatusAccepted {
		t.Fatalf("unexpected status: %s", updated.Status)
	}
	if updated.CourierID == nil || *updated.CourierID != courierID {
		t.Fatal("courier was not assigned")
	}
}

func TestCustomerCanCancelOwnPendingOrder(t *testing.T) {
	service, order := seededService(t, time.Now())

	updated, err := service.UpdateStatus(context.Background(), UpdateStatusInput{
		OrderID:    order.ID,
		ActorID:    order.CustomerID,
		ActorRole:  identity.RoleCustomer,
		NextStatus: domain.StatusCancelled,
	})
	if err != nil {
		t.Fatalf("cancel returned error: %v", err)
	}
	if updated.Status != domain.StatusCancelled {
		t.Fatalf("unexpected status: %s", updated.Status)
	}
}

func TestCustomerCannotCancelAnotherCustomersOrder(t *testing.T) {
	service, order := seededService(t, time.Now())

	_, err := service.UpdateStatus(context.Background(), UpdateStatusInput{
		OrderID:    order.ID,
		ActorID:    uuid.New(),
		ActorRole:  identity.RoleCustomer,
		NextStatus: domain.StatusCancelled,
	})

	appErr := requireAppError(t, err)
	if appErr.Kind != apperror.KindForbidden {
		t.Fatalf("expected forbidden kind, got %v", appErr.Kind)
	}
}

func TestUpdateStatusReportsConcurrentModification(t *testing.T) {
	now := time.Now()
	repository := newFakeOrderRepository()
	service := NewService(repository, func() time.Time { return now })

	order, err := service.Create(context.Background(), validCreateInput(uuid.New()))
	if err != nil {
		t.Fatalf("seed order: %v", err)
	}

	repository.conflictOnUpdate = true

	_, err = service.UpdateStatus(context.Background(), UpdateStatusInput{
		OrderID:    order.ID,
		ActorID:    order.CustomerID,
		ActorRole:  identity.RoleCustomer,
		NextStatus: domain.StatusCancelled,
	})

	appErr := requireAppError(t, err)
	if appErr.Kind != apperror.KindConflict || appErr.Code != "ORDER_CONFLICT" {
		t.Fatalf("unexpected error: %#v", appErr)
	}
}

func TestInvalidStatusTransitionReturnsConflict(t *testing.T) {
	service, order := seededService(t, time.Now())

	_, err := service.UpdateStatus(context.Background(), UpdateStatusInput{
		OrderID:    order.ID,
		ActorID:    uuid.New(),
		ActorRole:  identity.RoleAdmin,
		NextStatus: domain.StatusDelivered,
	})

	appErr := requireAppError(t, err)
	if appErr.Kind != apperror.KindConflict || appErr.Code != "INVALID_STATUS_TRANSITION" {
		t.Fatalf("unexpected error: %#v", appErr)
	}
}

func seededService(t *testing.T, now time.Time) (*Service, *domain.Order) {
	t.Helper()

	repository := newFakeOrderRepository()
	service := NewService(repository, func() time.Time { return now })

	order, err := service.Create(context.Background(), validCreateInput(uuid.New()))
	if err != nil {
		t.Fatalf("seed order: %v", err)
	}

	return service, order
}

func validCreateInput(customerID uuid.UUID) CreateInput {
	return CreateInput{
		CustomerID:     customerID,
		CustomerRole:   identity.RoleCustomer,
		ItemName:       "Package from office",
		Description:    "Fragile item",
		PickupAddress:  "Jl. Sudirman No. 1, Jakarta",
		DropoffAddress: "Jl. Thamrin No. 2, Jakarta",
		ItemPrice:      150000,
		ServiceFee:     15000,
	}
}

func requireAppError(t *testing.T, err error) *apperror.Error {
	t.Helper()

	if err == nil {
		t.Fatal("expected an error")
	}

	var appErr *apperror.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("expected apperror, got %T", err)
	}
	return appErr
}

type fakeOrderRepository struct {
	orders           map[uuid.UUID]domain.Order
	conflictOnUpdate bool
}

func newFakeOrderRepository() *fakeOrderRepository {
	return &fakeOrderRepository{orders: make(map[uuid.UUID]domain.Order)}
}

func (r *fakeOrderRepository) Create(_ context.Context, order *domain.Order) error {
	r.orders[order.ID] = *order
	return nil
}

func (r *fakeOrderRepository) ByID(_ context.Context, id uuid.UUID) (*domain.Order, error) {
	order, exists := r.orders[id]
	if !exists {
		return nil, domain.ErrOrderNotFound
	}
	return &order, nil
}

func (r *fakeOrderRepository) Update(_ context.Context, order *domain.Order, expectedStatus domain.Status) error {
	stored, exists := r.orders[order.ID]
	if !exists {
		return domain.ErrOrderNotFound
	}
	if r.conflictOnUpdate || stored.Status != expectedStatus {
		return ErrOrderConflict
	}
	r.orders[order.ID] = *order
	return nil
}

func (r *fakeOrderRepository) List(_ context.Context, filter ListFilter) ([]domain.Order, int64, error) {
	orders := make([]domain.Order, 0)
	for _, order := range r.orders {
		if filter.CustomerID != nil && order.CustomerID != *filter.CustomerID {
			continue
		}
		if filter.CourierID != nil && (order.CourierID == nil || *order.CourierID != *filter.CourierID) {
			continue
		}
		orders = append(orders, order)
	}

	total := int64(len(orders))
	start := filter.Offset
	if start > len(orders) {
		start = len(orders)
	}
	end := start + filter.Limit
	if end > len(orders) {
		end = len(orders)
	}

	return orders[start:end], total, nil
}
