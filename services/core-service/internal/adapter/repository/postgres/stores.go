package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/titipku/titipku-backend/services/core-service/internal/domain"
)

const storeQuery = `
	SELECT
		s.id, s.owner_user_id, s.name, s.category, s.description, s.address, s.whatsapp,
		s.latitude, s.longitude, s.photo_url, s.is_active, s.created_at, s.updated_at,
		$1::uuid IS NOT NULL AND EXISTS (
			SELECT 1 FROM partners p
			WHERE p.store_id = s.id
			  AND p.business_id = $1
			  AND p.is_active
		) AS has_consigned,
		$1::uuid IS NOT NULL AND EXISTS (
			SELECT 1 FROM consignment_proposals cp
			WHERE cp.store_id = s.id
			  AND cp.business_id = $1
			  AND cp.status = 'pending'
		) AS has_pending_proposal
	FROM stores s
`

func (s *Store) ListStores(ctx context.Context, filter domain.StoreFilter) ([]domain.Store, error) {
	query := storeQuery
	args := []any{filter.ViewerBusinessID}
	conditions := []string{"TRUE"}

	if filter.ActiveOnly {
		conditions = append(conditions, "s.is_active")
	}
	if filter.MinLat != nil && filter.MaxLat != nil && filter.MinLng != nil && filter.MaxLng != nil {
		args = append(args, *filter.MinLat, *filter.MaxLat, *filter.MinLng, *filter.MaxLng)
		conditions = append(conditions, fmt.Sprintf(
			"s.latitude BETWEEN $%d AND $%d AND s.longitude BETWEEN $%d AND $%d",
			len(args)-3, len(args)-2, len(args)-1, len(args),
		))
	}
	if filter.Category != "" {
		args = append(args, filter.Category)
		conditions = append(conditions, fmt.Sprintf("s.category = $%d", len(args)))
	}
	if search := strings.TrimSpace(filter.Search); search != "" {
		args = append(args, "%"+strings.ToLower(search)+"%")
		conditions = append(conditions, fmt.Sprintf(
			"(lower(s.name) LIKE $%d OR lower(s.address) LIKE $%d)",
			len(args), len(args),
		))
	}

	query += " WHERE " + strings.Join(conditions, " AND ")
	query += " ORDER BY s.name"

	if filter.Limit > 0 {
		args = append(args, filter.Limit)
		query += fmt.Sprintf(" LIMIT $%d", len(args))
	}
	if filter.Offset > 0 {
		args = append(args, filter.Offset)
		query += fmt.Sprintf(" OFFSET $%d", len(args))
	}

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list stores: %w", err)
	}
	defer rows.Close()

	stores := make([]domain.Store, 0)
	for rows.Next() {
		store, err := scanStore(rows)
		if err != nil {
			return nil, fmt.Errorf("scan store: %w", err)
		}
		stores = append(stores, *store)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate stores: %w", err)
	}
	return stores, nil
}

func (s *Store) StoreByID(ctx context.Context, storeID uuid.UUID, viewerBusinessID *uuid.UUID) (*domain.Store, error) {
	row := s.pool.QueryRow(ctx, storeQuery+` WHERE s.id = $2`, viewerBusinessID, storeID)
	store, err := scanStore(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("store by id: %w", err)
	}
	return store, nil
}

func (s *Store) StoreByOwnerUserID(ctx context.Context, userID uuid.UUID) (*domain.Store, error) {
	row := s.pool.QueryRow(ctx, storeQuery+` WHERE s.owner_user_id = $2`, (*uuid.UUID)(nil), userID)
	store, err := scanStore(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("store by owner: %w", err)
	}
	return store, nil
}

func (s *Store) CreateStore(ctx context.Context, store domain.Store) (*domain.Store, error) {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO stores (id, owner_user_id, name, category, description, address, whatsapp,
		                    latitude, longitude, photo_url, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`,
		store.ID,
		store.OwnerUserID,
		store.Name,
		store.Category,
		store.Description,
		store.Address,
		store.WhatsApp,
		store.Latitude,
		store.Longitude,
		store.PhotoURL,
		store.IsActive,
		store.CreatedAt,
		store.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, domain.ErrStoreExists
		}
		return nil, fmt.Errorf("create store: %w", err)
	}
	return s.StoreByID(ctx, store.ID, nil)
}

func (s *Store) UpdateStore(ctx context.Context, store domain.Store) (*domain.Store, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE stores
		SET name = $2, category = $3, description = $4, address = $5, whatsapp = $6,
		    latitude = $7, longitude = $8, photo_url = $9, updated_at = now()
		WHERE id = $1 AND owner_user_id = $10
		RETURNING id
	`,
		store.ID,
		store.Name,
		store.Category,
		store.Description,
		store.Address,
		store.WhatsApp,
		store.Latitude,
		store.Longitude,
		store.PhotoURL,
		store.OwnerUserID,
	)
	var id uuid.UUID
	if err := row.Scan(&id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrStoreNotOwned
		}
		return nil, fmt.Errorf("update store: %w", err)
	}
	return s.StoreByID(ctx, id, nil)
}

func (s *Store) SetStoreActive(ctx context.Context, ownerUserID, storeID uuid.UUID, active bool) (*domain.Store, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE stores
		SET is_active = $3, updated_at = now()
		WHERE id = $1 AND owner_user_id = $2
		RETURNING id
	`, storeID, ownerUserID, active)

	var id uuid.UUID
	if err := row.Scan(&id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrStoreNotOwned
		}
		return nil, fmt.Errorf("set store active: %w", err)
	}
	return s.StoreByID(ctx, id, nil)
}

func scanStore(row pgx.Row) (*domain.Store, error) {
	var store domain.Store
	err := row.Scan(
		&store.ID,
		&store.OwnerUserID,
		&store.Name,
		&store.Category,
		&store.Description,
		&store.Address,
		&store.WhatsApp,
		&store.Latitude,
		&store.Longitude,
		&store.PhotoURL,
		&store.IsActive,
		&store.CreatedAt,
		&store.UpdatedAt,
		&store.HasConsigned,
		&store.HasPendingProposal,
	)
	if err != nil {
		return nil, err
	}
	return &store, nil
}
