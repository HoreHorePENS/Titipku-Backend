package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/titipku/titipku-backend/services/core-service/internal/domain"
)

func (s *Store) CreateAccount(ctx context.Context, account domain.Account, passwordHash string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin create account: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `
		INSERT INTO users (id, email, full_name, password_hash, role, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`,
		account.User.ID,
		account.User.Email,
		account.User.FullName,
		passwordHash,
		account.User.Role,
		account.User.CreatedAt,
		account.User.UpdatedAt,
	); err != nil {
		if isUniqueViolation(err) {
			return domain.ErrEmailTaken
		}
		return fmt.Errorf("insert user: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO businesses (id, name, owner_name, whatsapp, address, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`,
		account.Business.ID,
		account.Business.Name,
		account.Business.OwnerName,
		account.Business.WhatsApp,
		account.Business.Address,
		account.Business.CreatedAt,
		account.Business.UpdatedAt,
	); err != nil {
		return fmt.Errorf("insert business: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO members (id, business_id, user_id, role, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, TRUE, $5, $6)
	`,
		account.Member.ID,
		account.Member.BusinessID,
		account.Member.UserID,
		account.Member.Role,
		account.Member.CreatedAt,
		account.Member.UpdatedAt,
	); err != nil {
		return fmt.Errorf("insert member: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit create account: %w", err)
	}
	return nil
}

func (s *Store) AccountByEmail(ctx context.Context, email string) (*domain.Account, error) {
	return scanAccount(s.pool.QueryRow(ctx, accountQuery+`
		WHERE lower(u.email) = $1
		  AND m.is_active
		ORDER BY m.created_at
		LIMIT 1
	`, email))
}

func (s *Store) AccountByUserID(ctx context.Context, userID uuid.UUID) (*domain.Account, error) {
	return scanAccount(s.pool.QueryRow(ctx, accountQuery+`
		WHERE u.id = $1
		  AND m.is_active
		ORDER BY m.created_at
		LIMIT 1
	`, userID))
}

const accountQuery = `
	SELECT
		u.id, u.email, u.full_name, u.password_hash, u.role, u.created_at, u.updated_at,
		m.id, m.business_id, m.user_id, m.role, m.is_active, m.created_at, m.updated_at,
		b.id, b.name, b.owner_name, b.whatsapp, b.address, b.visit_sequence, b.created_at, b.updated_at
	FROM users u
	JOIN members m ON m.user_id = u.id
	JOIN businesses b ON b.id = m.business_id
`

func scanAccount(row pgx.Row) (*domain.Account, error) {
	var account domain.Account
	err := row.Scan(
		&account.User.ID,
		&account.User.Email,
		&account.User.FullName,
		&account.User.PasswordHash,
		&account.User.Role,
		&account.User.CreatedAt,
		&account.User.UpdatedAt,
		&account.Member.ID,
		&account.Member.BusinessID,
		&account.Member.UserID,
		&account.Member.Role,
		&account.Member.IsActive,
		&account.Member.CreatedAt,
		&account.Member.UpdatedAt,
		&account.Business.ID,
		&account.Business.Name,
		&account.Business.OwnerName,
		&account.Business.WhatsApp,
		&account.Business.Address,
		&account.Business.VisitSequence,
		&account.Business.CreatedAt,
		&account.Business.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("scan account: %w", err)
	}
	return &account, nil
}

func (s *Store) UpdateBusiness(ctx context.Context, business domain.Business) (*domain.Business, error) {
	var result domain.Business
	err := s.pool.QueryRow(ctx, `
		UPDATE businesses
		SET name = $2, owner_name = $3, whatsapp = $4, address = $5, updated_at = now()
		WHERE id = $1
		RETURNING id, name, owner_name, whatsapp, address, visit_sequence, created_at, updated_at
	`,
		business.ID,
		business.Name,
		business.OwnerName,
		business.WhatsApp,
		business.Address,
	).Scan(
		&result.ID,
		&result.Name,
		&result.OwnerName,
		&result.WhatsApp,
		&result.Address,
		&result.VisitSequence,
		&result.CreatedAt,
		&result.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("update business: %w", err)
	}
	return &result, nil
}
