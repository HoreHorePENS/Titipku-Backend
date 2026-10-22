package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/titipku/titipku-backend/internal/platform/identity"
	"github.com/titipku/titipku-backend/services/auth-service/internal/domain"
)

const userColumns = `id::text, email, full_name, password_hash, role, created_at, updated_at`

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

func (r *UserRepository) Create(ctx context.Context, user *domain.User) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO users (id, email, full_name, password_hash, role, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`,
		user.ID.String(),
		user.Email,
		user.FullName,
		user.PasswordHash,
		string(user.Role),
		user.CreatedAt,
		user.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrEmailTaken
		}
		return fmt.Errorf("insert user: %w", err)
	}

	return nil
}

func (r *UserRepository) ByEmail(ctx context.Context, email string) (*domain.User, error) {
	user, err := scanUser(r.pool.QueryRow(ctx, `
		SELECT `+userColumns+`
		FROM users
		WHERE lower(email) = $1
	`, email))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("find user by email: %w", err)
	}

	return user, nil
}

func (r *UserRepository) ByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	user, err := scanUser(r.pool.QueryRow(ctx, `
		SELECT `+userColumns+`
		FROM users
		WHERE id = $1
	`, id.String()))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("find user by id: %w", err)
	}

	return user, nil
}

func scanUser(row pgx.Row) (*domain.User, error) {
	var (
		id           string
		email        string
		fullName     string
		passwordHash string
		role         string
		createdAt    time.Time
		updatedAt    time.Time
	)

	if err := row.Scan(&id, &email, &fullName, &passwordHash, &role, &createdAt, &updatedAt); err != nil {
		return nil, err
	}

	userID, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("parse user id: %w", err)
	}

	return &domain.User{
		ID:           userID,
		Email:        email,
		FullName:     fullName,
		PasswordHash: passwordHash,
		Role:         identity.Role(role),
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
