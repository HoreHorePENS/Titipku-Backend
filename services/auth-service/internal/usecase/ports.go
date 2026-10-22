package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/titipku/titipku-backend/internal/platform/identity"
	"github.com/titipku/titipku-backend/services/auth-service/internal/domain"
)

type UserRepository interface {
	Create(ctx context.Context, user *domain.User) error
	ByEmail(ctx context.Context, email string) (*domain.User, error)
	ByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
}

type PasswordHasher interface {
	Hash(plainPassword string) (string, error)
	Compare(hash, plainPassword string) error
}

type TokenIssuer interface {
	Issue(userID uuid.UUID, email, name string, role identity.Role) (string, time.Time, error)
}
