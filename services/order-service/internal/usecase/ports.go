package usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/titipku/titipku-backend/services/order-service/internal/domain"
)

var ErrOrderConflict = errors.New("order was modified concurrently")

type OrderRepository interface {
	Create(ctx context.Context, order *domain.Order) error
	ByID(ctx context.Context, id uuid.UUID) (*domain.Order, error)
	Update(ctx context.Context, order *domain.Order, expectedStatus domain.Status) error
	List(ctx context.Context, filter ListFilter) ([]domain.Order, int64, error)
}

type ListFilter struct {
	CustomerID *uuid.UUID
	CourierID  *uuid.UUID
	Limit      int
	Offset     int
}
