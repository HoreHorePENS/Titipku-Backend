package httpx

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/titipku/titipku-backend/internal/platform/apperror"
)

func NewRouter(logger *slog.Logger, allowedOrigins ...string) *chi.Mux {
	router := chi.NewRouter()

	router.Use(
		RequestID,
		AccessLog(logger),
		Recoverer(logger),
		SecurityHeaders,
	)

	if len(allowedOrigins) > 0 {
		router.Use(CORS(allowedOrigins))
	}

	router.NotFound(func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, r, apperror.NotFound("ROUTE_NOT_FOUND", "route not found"))
	})

	router.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, r, apperror.MethodNotAllowed())
	})

	return router
}
