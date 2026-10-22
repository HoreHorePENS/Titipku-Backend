package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/titipku/titipku-backend/services/order-service/internal/domain"
	"github.com/titipku/titipku-backend/services/order-service/internal/usecase"
)

const orderColumns = `id::text, customer_id::text, courier_id::text, item_name, description, pickup_address, dropoff_address, item_price, service_fee, status, created_at, updated_at`

type OrderRepository struct {
	pool *pgxpool.Pool
}

func NewOrderRepository(pool *pgxpool.Pool) *OrderRepository {
	return &OrderRepository{pool: pool}
}

func (r *OrderRepository) Create(ctx context.Context, order *domain.Order) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO orders (
			id, customer_id, courier_id, item_name, description,
			pickup_address, dropoff_address, item_price, service_fee,
			status, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`,
		order.ID.String(),
		order.CustomerID.String(),
		nullableUUID(order.CourierID),
		order.ItemName,
		order.Description,
		order.PickupAddress,
		order.DropoffAddress,
		order.ItemPrice,
		order.ServiceFee,
		string(order.Status),
		order.CreatedAt,
		order.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert order: %w", err)
	}

	return nil
}

func (r *OrderRepository) ByID(ctx context.Context, id uuid.UUID) (*domain.Order, error) {
	order, err := scanOrder(r.pool.QueryRow(ctx, `
		SELECT `+orderColumns+`
		FROM orders
		WHERE id = $1
	`, id.String()))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrOrderNotFound
		}
		return nil, fmt.Errorf("find order: %w", err)
	}

	return order, nil
}

func (r *OrderRepository) Update(ctx context.Context, order *domain.Order, expectedStatus domain.Status) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE orders
		SET courier_id = $2,
			status = $3,
			updated_at = $4
		WHERE id = $1 AND status = $5
	`,
		order.ID.String(),
		nullableUUID(order.CourierID),
		string(order.Status),
		order.UpdatedAt,
		string(expectedStatus),
	)
	if err != nil {
		return fmt.Errorf("update order: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return usecase.ErrOrderConflict
	}

	return nil
}

func (r *OrderRepository) List(ctx context.Context, filter usecase.ListFilter) ([]domain.Order, int64, error) {
	conditions := []string{"1 = 1"}
	arguments := make([]any, 0, 2)

	if filter.CustomerID != nil {
		arguments = append(arguments, filter.CustomerID.String())
		conditions = append(conditions, fmt.Sprintf("customer_id = $%d", len(arguments)))
	}
	if filter.CourierID != nil {
		arguments = append(arguments, filter.CourierID.String())
		conditions = append(conditions, fmt.Sprintf("courier_id = $%d", len(arguments)))
	}

	whereClause := strings.Join(conditions, " AND ")

	var total int64
	if err := r.pool.QueryRow(
		ctx,
		"SELECT count(*) FROM orders WHERE "+whereClause,
		arguments...,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count orders: %w", err)
	}

	queryArguments := append(append([]any{}, arguments...), filter.Limit, filter.Offset)
	query := fmt.Sprintf(`
		SELECT %s
		FROM orders
		WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, orderColumns, whereClause, len(arguments)+1, len(arguments)+2)

	rows, err := r.pool.Query(ctx, query, queryArguments...)
	if err != nil {
		return nil, 0, fmt.Errorf("list orders: %w", err)
	}
	defer rows.Close()

	orders := make([]domain.Order, 0)
	for rows.Next() {
		order, err := scanOrder(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan order: %w", err)
		}
		orders = append(orders, *order)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate orders: %w", err)
	}

	return orders, total, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanOrder(row scanner) (*domain.Order, error) {
	var (
		id             string
		customerID     string
		courierID      pgtype.Text
		itemName       string
		description    string
		pickupAddress  string
		dropoffAddress string
		itemPrice      int64
		serviceFee     int64
		status         string
		createdAt      time.Time
		updatedAt      time.Time
	)

	if err := row.Scan(
		&id,
		&customerID,
		&courierID,
		&itemName,
		&description,
		&pickupAddress,
		&dropoffAddress,
		&itemPrice,
		&serviceFee,
		&status,
		&createdAt,
		&updatedAt,
	); err != nil {
		return nil, err
	}

	parsedID, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("parse order id: %w", err)
	}
	parsedCustomerID, err := uuid.Parse(customerID)
	if err != nil {
		return nil, fmt.Errorf("parse customer id: %w", err)
	}

	var parsedCourierID *uuid.UUID
	if courierID.Valid {
		value, err := uuid.Parse(courierID.String)
		if err != nil {
			return nil, fmt.Errorf("parse courier id: %w", err)
		}
		parsedCourierID = &value
	}

	return &domain.Order{
		ID:             parsedID,
		CustomerID:     parsedCustomerID,
		CourierID:      parsedCourierID,
		ItemName:       itemName,
		Description:    description,
		PickupAddress:  pickupAddress,
		DropoffAddress: dropoffAddress,
		ItemPrice:      itemPrice,
		ServiceFee:     serviceFee,
		Status:         domain.Status(status),
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
	}, nil
}

func nullableUUID(id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	return id.String()
}
