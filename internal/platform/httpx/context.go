package httpx

import (
	"net/http"

	"github.com/titipku/titipku-backend/internal/platform/apperror"
	"github.com/titipku/titipku-backend/internal/platform/identity"
)

func PrincipalFromRequest(r *http.Request) (identity.Principal, error) {
	principal, ok := identity.PrincipalFrom(r.Context())
	if !ok {
		return identity.Principal{}, apperror.Unauthorized(apperror.CodeUnauthorized, "authentication is required")
	}
	return principal, nil
}
