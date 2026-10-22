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

	router.Group(func(router chi.Router) {
		router.Use(httpx.Authenticate(verifier))

		router.Post("/v1/orders", httpx.Handle(logger, handler.Create))
		router.Get("/v1/orders", httpx.Handle(logger, handler.List))
		router.Get("/v1/orders/{orderID}", httpx.Handle(logger, handler.Get))
		router.Patch("/v1/orders/{orderID}/status", httpx.Handle(logger, handler.UpdateStatus))
	})

	return router
}
