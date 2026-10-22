package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/titipku/titipku-backend/internal/platform/health"
	"github.com/titipku/titipku-backend/internal/platform/httpx"
)

func NewRouter(
	handler *Handler,
	logger *slog.Logger,
	verifier httpx.TokenVerifier,
	pinger health.Pinger,
) http.Handler {
	router := httpx.NewRouter(logger)
	health.Register(router, pinger)

	router.Route("/v1/profiles", func(router chi.Router) {
		router.Use(httpx.Authenticate(verifier))
		router.Get("/me", httpx.Handle(logger, handler.GetMine))
		router.Put("/me", httpx.Handle(logger, handler.UpsertMine))
	})

	return router
}
