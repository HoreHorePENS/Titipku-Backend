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
		INSERT INTO users (id, email, full_name, password_hash, role, account_type, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`,
		account.User.ID,
		account.User.Email,
		account.User.FullName,
		passwordHash,
		account.User.Role,
		account.User.AccountType,
		account.User.CreatedAt,
		account.User.UpdatedAt,
	); err != nil {
		if isUniqueViolation(err) {
			return domain.ErrEmailTaken
		}
		return fmt.Errorf("insert user: %w", err)
	}

	// Akun pemilik toko tidak memiliki business/member; mereka punya toko.
	if account.User.AccountType == string(accountTypePemilikToko) {
		if account.Store == nil {
			return fmt.Errorf("insert store: store data is required for pemilik_toko account")
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO stores (id, owner_user_id, name, category, description, address, whatsapp,
			                   latitude, longitude, photo_url, is_active, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, TRUE, $11, $12)
		`,
			account.Store.ID,
			account.Store.OwnerUserID,
			account.Store.Name,
			account.Store.Category,
			account.Store.Description,
			account.Store.Address,
			account.Store.WhatsApp,
			account.Store.Latitude,
			account.Store.Longitude,
			account.Store.PhotoURL,
			account.Store.CreatedAt,
			account.Store.UpdatedAt,
		); err != nil {
			if isUniqueViolation(err) {
				return domain.ErrStoreExists
			}
			return fmt.Errorf("insert store: %w", err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit create account: %w", err)
		}
		return nil
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
	user, err := scanUser(s.pool.QueryRow(ctx, userByEmailQuery, email))
	if err != nil {
		return nil, err
	}
	return s.loadAccountScope(ctx, *user)
}

func (s *Store) AccountByUserID(ctx context.Context, userID uuid.UUID) (*domain.Account, error) {
	user, err := scanUser(s.pool.QueryRow(ctx, userByIDQuery, userID))
	if err != nil {
		return nil, err
	}
	return s.loadAccountScope(ctx, *user)
}

// loadAccountScope melengkapi user dengan business/member (penitip)
// atau toko (pemilik_toko) sesuai account_type.
func (s *Store) loadAccountScope(ctx context.Context, user domain.User) (*domain.Account, error) {
	account := domain.Account{User: user}

	if user.AccountType == string(accountTypePemilikToko) {
		store, err := s.StoreByOwnerUserID(ctx, user.ID)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				// Toko belum dibuat; akun tetap valid untuk login.
				return &account, nil
			}
			return nil, err
		}
		account.Store = store
		return &account, nil
	}

	err := s.pool.QueryRow(ctx, memberBusinessQuery+`
		WHERE m.user_id = $1
		  AND m.is_active
		ORDER BY m.created_at
		LIMIT 1
	`, user.ID).Scan(
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
		return nil, fmt.Errorf("load account scope: %w", err)
	}
	return &account, nil
}

const userByEmailQuery = `
	SELECT u.id, u.email, u.full_name, u.password_hash, u.role, u.account_type, u.created_at, u.updated_at
	FROM users u
	WHERE lower(u.email) = $1
`

const userByIDQuery = `
	SELECT u.id, u.email, u.full_name, u.password_hash, u.role, u.account_type, u.created_at, u.updated_at
	FROM users u
	WHERE u.id = $1
`

const memberBusinessQuery = `
	SELECT
		m.id, m.business_id, m.user_id, m.role, m.is_active, m.created_at, m.updated_at,
		b.id, b.name, b.owner_name, b.whatsapp, b.address, b.visit_sequence, b.created_at, b.updated_at
	FROM members m
	JOIN businesses b ON b.id = m.business_id
`

func scanUser(row pgx.Row) (*domain.User, error) {
	var user domain.User
	err := row.Scan(
		&user.ID,
		&user.Email,
		&user.FullName,
		&user.PasswordHash,
		&user.Role,
		&user.AccountType,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("scan user: %w", err)
	}
	return &user, nil
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
