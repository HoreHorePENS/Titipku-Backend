package identity

import (
	"context"

	"github.com/google/uuid"
)

type Principal struct {
	UserID      uuid.UUID
	Email       string
	Name        string
	Role        Role
	AccountType AccountType
	BusinessID  uuid.UUID
	MemberID    uuid.UUID
}

func (p Principal) HasBusinessScope() bool {
	return p.BusinessID != uuid.Nil && p.MemberID != uuid.Nil
}

// IsPenitip mengembalikan true bila akun adalah produsen/penitip.
func (p Principal) IsPenitip() bool {
	return p.AccountType == AccountTypePenitip
}

// IsPemilikToko mengembalikan true bila akun pemilik toko.
func (p Principal) IsPemilikToko() bool {
	return p.AccountType == AccountTypePemilikToko
}

type principalContextKey struct{}

func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, principalContextKey{}, principal)
}

func PrincipalFrom(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalContextKey{}).(Principal)
	return principal, ok
}
