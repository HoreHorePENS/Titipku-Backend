package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/titipku/titipku-backend/internal/platform/health"
	"github.com/titipku/titipku-backend/internal/platform/httpx"
	"github.com/titipku/titipku-backend/internal/platform/identity"
)

type RouterOptions struct {
	AllowedOrigins           []string
	PublicRateLimitPerMinute int
}

func NewRouter(
	handler *Handler,
	logger *slog.Logger,
	verifier httpx.TokenVerifier,
	pinger health.Pinger,
	options RouterOptions,
) http.Handler {
	router := httpx.NewRouter(logger, options.AllowedOrigins...)
	health.Register(router, pinger)

	router.Route("/v1", func(router chi.Router) {
		router.Post("/auth/register", httpx.Handle(logger, handler.Register))
		router.Post("/auth/login", httpx.Handle(logger, handler.Login))

		public := router
		if options.PublicRateLimitPerMinute > 0 {
			public = router.With(httpx.RateLimit(options.PublicRateLimitPerMinute, time.Minute))
		}
		public.Get("/public/invoices/{token}", httpx.Handle(logger, handler.PublicInvoice))

		router.Group(func(router chi.Router) {
			router.Use(httpx.Authenticate(verifier))
			router.Use(httpx.RequireRoles(identity.RoleOwner))

			router.Get("/auth/me", httpx.Handle(logger, handler.Me))

			router.Get("/business", httpx.Handle(logger, handler.GetBusiness))
			router.Patch("/business", httpx.Handle(logger, handler.UpdateBusiness))

			router.Get("/products", httpx.Handle(logger, handler.ListProducts))
			router.Post("/products", httpx.Handle(logger, handler.CreateProduct))
			router.Put("/products/{productID}", httpx.Handle(logger, handler.UpdateProduct))
			router.Patch("/products/{productID}/status", httpx.Handle(logger, handler.SetProductActive))

			router.Get("/partners", httpx.Handle(logger, handler.ListPartners))
			router.Post("/partners", httpx.Handle(logger, handler.CreatePartner))
			router.Put("/partners/{partnerID}", httpx.Handle(logger, handler.UpdatePartner))
			router.Patch("/partners/{partnerID}/status", httpx.Handle(logger, handler.SetPartnerActive))

			router.Get("/visits", httpx.Handle(logger, handler.ListVisits))
			router.Post("/visits", httpx.Handle(logger, handler.CreateVisit))
			router.Get("/visits/{visitID}", httpx.Handle(logger, handler.GetVisit))

			router.Get("/invoices", httpx.Handle(logger, handler.ListInvoices))
			router.Get("/invoices/{invoiceID}", httpx.Handle(logger, handler.GetInvoice))

			router.Post("/payments", httpx.Handle(logger, handler.CreatePayment))
			router.Post("/payments/{paymentID}/void", httpx.Handle(logger, handler.VoidPayment))

			router.Get("/dashboard", httpx.Handle(logger, handler.Dashboard))
		})
	})

	return router
}
