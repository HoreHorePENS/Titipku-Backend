package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/titipku/titipku-backend/services/core-service/internal/domain"
)

const productColumns = `
	id, business_id, name, photo_path, consign_price, retail_price,
	shelf_life_days, is_active, created_at, updated_at
`

func (s *Store) ListProducts(ctx context.Context, businessID uuid.UUID, activeOnly bool) ([]domain.Product, error) {
	query := `
		SELECT ` + productColumns + `
		FROM products
		WHERE business_id = $1
	`
	if activeOnly {
		query += ` AND is_active`
	}
	query += ` ORDER BY name`

	rows, err := s.pool.Query(ctx, query, businessID)
	if err != nil {
		return nil, fmt.Errorf("list products: %w", err)
	}
	defer rows.Close()

	products := make([]domain.Product, 0)
	for rows.Next() {
		product, err := scanProduct(rows)
		if err != nil {
			return nil, err
		}
		products = append(products, *product)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate products: %w", err)
	}
	return products, nil
}

func (s *Store) UpsertProduct(ctx context.Context, product domain.Product) (*domain.Product, error) {
	result, err := scanProduct(s.pool.QueryRow(ctx, `
		INSERT INTO products (
			id, business_id, name, photo_path, consign_price, retail_price,
			shelf_life_days, is_active, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now(), now())
		ON CONFLICT (id) DO UPDATE
		SET name = EXCLUDED.name,
			photo_path = products.photo_path, -- foto hanya diubah lewat endpoint foto
			consign_price = EXCLUDED.consign_price,
			retail_price = EXCLUDED.retail_price,
			shelf_life_days = EXCLUDED.shelf_life_days,
			is_active = EXCLUDED.is_active,
			updated_at = now()
		WHERE products.business_id = EXCLUDED.business_id
		RETURNING `+productColumns,
		product.ID,
		product.BusinessID,
		product.Name,
		product.PhotoPath,
		product.ConsignPrice,
		product.RetailPrice,
		product.ShelfLifeDays,
		product.IsActive,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrConflict
		}
		return nil, fmt.Errorf("upsert product: %w", err)
	}
	return result, nil
}

func (s *Store) SetProductActive(ctx context.Context, businessID, productID uuid.UUID, active bool) (*domain.Product, error) {
	result, err := scanProduct(s.pool.QueryRow(ctx, `
		UPDATE products
		SET is_active = $3, updated_at = now()
		WHERE id = $2 AND business_id = $1
		RETURNING `+productColumns,
		businessID, productID, active,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("set product active: %w", err)
	}
	return result, nil
}

func scanProduct(row pgx.Row) (*domain.Product, error) {
	var product domain.Product
	err := row.Scan(
		&product.ID,
		&product.BusinessID,
		&product.Name,
		&product.PhotoPath,
		&product.ConsignPrice,
		&product.RetailPrice,
		&product.ShelfLifeDays,
		&product.IsActive,
		&product.CreatedAt,
		&product.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &product, nil
}
