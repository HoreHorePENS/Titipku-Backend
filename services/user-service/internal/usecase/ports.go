package usecase

import (
	"context"

	"github.com/google/uuid"
	"github.com/titipku/titipku-backend/services/user-service/internal/domain"
)

type ProfileRepository interface {
	ByUserID(ctx context.Context, userID uuid.UUID) (*domain.Profile, error)
	Save(ctx context.Context, profile *domain.Profile) error
}
