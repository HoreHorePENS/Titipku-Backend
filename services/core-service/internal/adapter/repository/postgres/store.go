package postgres

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/titipku/titipku-backend/services/core-service/internal/domain"
)

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}

func mapFunctionError(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}

	switch pgErr.Message {
	case "IDEMPOTENCY_MISMATCH":
		return domain.ErrIdempotencyMismatch
	case "STOCK_MISMATCH":
		return domain.ErrStockMismatch
	case "PAYMENT_EXCEEDS_RECEIVABLE":
		return domain.ErrPaymentExceedsReceivable
	case "PAYMENT_ALLOCATION_FAILED", "PAYMENT_ALLOCATION_INCONSISTENT":
		return domain.ErrPaymentAllocation
	case "MEMBER_NOT_FOUND", "BUSINESS_NOT_FOUND", "PARTNER_NOT_FOUND", "PRODUCT_NOT_FOUND", "PAYMENT_NOT_FOUND":
		return domain.ErrNotFound
	case "INVALID_VISIT_PAYLOAD", "INVALID_PAYMENT_PAYLOAD",
		"VISIT_CLIENT_REQUEST_ID_REQUIRED", "PAYMENT_CLIENT_REQUEST_ID_REQUIRED",
		"PARTNER_REQUIRED", "PRODUCT_REQUIRED", "VISIT_ITEMS_REQUIRED",
		"DUPLICATE_VISIT_PRODUCT", "VISIT_QUANTITY_INVALID", "PAYMENT_AMOUNT_INVALID",
		"PAYMENT_METHOD_INVALID":
		return domain.ErrConflict
	default:
		return fmt.Errorf("database function failed: %w", err)
	}
}
