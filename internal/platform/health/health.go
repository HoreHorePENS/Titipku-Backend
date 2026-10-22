package health

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/titipku/titipku-backend/internal/platform/apperror"
	"github.com/titipku/titipku-backend/internal/platform/httpx"
)

type Pinger interface {
	Ping(ctx context.Context) error
}

func Register(router chi.Router, pinger Pinger) {
	router.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteData(w, r, http.StatusOK, map[string]string{"status": "ok"})
	})

	router.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := pinger.Ping(ctx); err != nil {
			httpx.WriteError(w, r, apperror.Unavailable(
				"DEPENDENCY_UNAVAILABLE",
				"required dependency is unavailable",
			).WithCause(err))
			return
		}

		httpx.WriteData(w, r, http.StatusOK, map[string]string{"status": "ready"})
	})
}
