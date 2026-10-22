package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/titipku/titipku-backend/services/user-service/internal/domain"
)

const profileColumns = `user_id::text, full_name, phone, avatar_url, bio, created_at, updated_at`

type ProfileRepository struct {
	pool *pgxpool.Pool
}

func NewProfileRepository(pool *pgxpool.Pool) *ProfileRepository {
	return &ProfileRepository{pool: pool}
}

func (r *ProfileRepository) ByUserID(ctx context.Context, userID uuid.UUID) (*domain.Profile, error) {
	profile, err := scanProfile(r.pool.QueryRow(ctx, `
		SELECT `+profileColumns+`
		FROM profiles
		WHERE user_id = $1
	`, userID.String()))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrProfileNotFound
		}
		return nil, fmt.Errorf("find profile: %w", err)
	}

	return profile, nil
}

func (r *ProfileRepository) Save(ctx context.Context, profile *domain.Profile) error {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO profiles (user_id, full_name, phone, avatar_url, bio, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (user_id) DO UPDATE
		SET full_name = EXCLUDED.full_name,
			phone = EXCLUDED.phone,
			avatar_url = EXCLUDED.avatar_url,
			bio = EXCLUDED.bio,
			updated_at = EXCLUDED.updated_at
		RETURNING created_at, updated_at
	`,
		profile.UserID.String(),
		profile.FullName,
		profile.Phone,
		profile.AvatarURL,
		profile.Bio,
		profile.CreatedAt,
		profile.UpdatedAt,
	)
	if err := row.Scan(&profile.CreatedAt, &profile.UpdatedAt); err != nil {
		return fmt.Errorf("save profile: %w", err)
	}

	return nil
}

func scanProfile(row pgx.Row) (*domain.Profile, error) {
	var (
		userID    string
		fullName  string
		phone     string
		avatarURL string
		bio       string
		createdAt time.Time
		updatedAt time.Time
	)

	if err := row.Scan(&userID, &fullName, &phone, &avatarURL, &bio, &createdAt, &updatedAt); err != nil {
		return nil, err
	}

	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("parse user id: %w", err)
	}

	return &domain.Profile{
		UserID:    parsedUserID,
		FullName:  fullName,
		Phone:     phone,
		AvatarURL: avatarURL,
		Bio:       bio,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}, nil
}
