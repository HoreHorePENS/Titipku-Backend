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

	router.Route("/v1/auth", func(router chi.Router) {
		router.Post("/register", httpx.Handle(logger, handler.Register))
		router.Post("/login", httpx.Handle(logger, handler.Login))

		router.Group(func(router chi.Router) {
			router.Use(httpx.Authenticate(verifier))
			router.Get("/me", httpx.Handle(logger, handler.Me))
		})
	})

	return router
}
