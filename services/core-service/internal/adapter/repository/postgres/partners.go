package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/titipku/titipku-backend/services/core-service/internal/domain"
)

const partnerQuery = `
	SELECT
		p.id, p.business_id, p.name, p.owner_name, p.whatsapp, p.address,
		p.is_active, p.created_at, p.updated_at,
		COALESCE((
			SELECT SUM(i.total - i.paid)
			FROM invoices i
			WHERE i.business_id = p.business_id
			  AND i.partner_id = p.id
			  AND i.paid < i.total
		), 0) AS receivable
	FROM partners p
`

func (s *Store) ListPartners(ctx context.Context, businessID uuid.UUID, activeOnly bool) ([]domain.Partner, error) {
	query := partnerQuery + ` WHERE p.business_id = $1`
	if activeOnly {
		query += ` AND p.is_active`
	}
	query += ` ORDER BY p.name`

	rows, err := s.pool.Query(ctx, query, businessID)
	if err != nil {
		return nil, fmt.Errorf("list partners: %w", err)
	}
	defer rows.Close()

	partners := make([]domain.Partner, 0)
	for rows.Next() {
		partner, err := scanPartner(rows)
		if err != nil {
			return nil, err
		}
		partners = append(partners, *partner)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate partners: %w", err)
	}
	return partners, nil
}

func (s *Store) UpsertPartner(ctx context.Context, partner domain.Partner) (*domain.Partner, error) {
	result, err := scanPartner(s.pool.QueryRow(ctx, `
		INSERT INTO partners (
			id, business_id, name, owner_name, whatsapp, address, is_active, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, now(), now())
		ON CONFLICT (id) DO UPDATE
		SET name = EXCLUDED.name,
			owner_name = EXCLUDED.owner_name,
			whatsapp = EXCLUDED.whatsapp,
			address = EXCLUDED.address,
			is_active = EXCLUDED.is_active,
			updated_at = now()
		WHERE partners.business_id = EXCLUDED.business_id
		RETURNING
			id, business_id, name, owner_name, whatsapp, address, is_active,
			created_at, updated_at, 0::BIGINT
	`,
		partner.ID,
		partner.BusinessID,
		partner.Name,
		partner.OwnerName,
		partner.WhatsApp,
		partner.Address,
		partner.IsActive,
	))
	if err != nil {
		if isUniqueViolation(err) {
			return nil, domain.ErrWhatsAppTaken
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrConflict
		}
		return nil, fmt.Errorf("upsert partner: %w", err)
	}

	return s.partnerByID(ctx, partner.BusinessID, result.ID)
}

func (s *Store) SetPartnerActive(ctx context.Context, businessID, partnerID uuid.UUID, active bool) (*domain.Partner, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE partners
		SET is_active = $3, updated_at = now()
		WHERE id = $2 AND business_id = $1
	`, businessID, partnerID, active)
	if err != nil {
		return nil, fmt.Errorf("set partner active: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, domain.ErrNotFound
	}
	return s.partnerByID(ctx, businessID, partnerID)
}

func (s *Store) partnerByID(ctx context.Context, businessID, partnerID uuid.UUID) (*domain.Partner, error) {
	return scanPartner(s.pool.QueryRow(ctx, partnerQuery+`
		WHERE p.business_id = $1 AND p.id = $2
	`, businessID, partnerID))
}

func scanPartner(row pgx.Row) (*domain.Partner, error) {
	var partner domain.Partner
	err := row.Scan(
		&partner.ID,
		&partner.BusinessID,
		&partner.Name,
		&partner.OwnerName,
		&partner.WhatsApp,
		&partner.Address,
		&partner.IsActive,
		&partner.CreatedAt,
		&partner.UpdatedAt,
		&partner.Receivable,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("scan partner: %w", err)
	}
	return &partner, nil
}
