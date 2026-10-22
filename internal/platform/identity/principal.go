package identity

import (
	"context"

	"github.com/google/uuid"
)

type Principal struct {
	UserID     uuid.UUID
	Email      string
	Name       string
	Role       Role
	BusinessID uuid.UUID
	MemberID   uuid.UUID
}

func (p Principal) HasBusinessScope() bool {
	return p.BusinessID != uuid.Nil && p.MemberID != uuid.Nil
}

type principalContextKey struct{}

func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, principalContextKey{}, principal)
}

func PrincipalFrom(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalContextKey{}).(Principal)
	return principal, ok
}
