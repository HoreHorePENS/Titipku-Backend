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
	// Uploads menyajikan foto unggahan di /uploads/* (opsional).
	Uploads http.Handler
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

	if options.Uploads != nil {
		// Foto toko/produk bersifat publik (ditampilkan di kartu) dan bernama acak.
		router.Handle("/uploads/*", options.Uploads)
	}

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

			// Katalog toko untuk peta: terbuka bagi semua persona yang login.
			router.Get("/stores", httpx.Handle(logger, handler.ListStores))
			router.Get("/stores/{storeID}", httpx.Handle(logger, handler.GetStore))

			// Proposal kemitraan dua arah: tawaran toko & pengajuan penitip.
			router.Get("/proposals", httpx.Handle(logger, handler.ListProposals))
			router.Post("/proposals", httpx.Handle(logger, handler.CreateProposal))
			router.Post("/proposals/{proposalID}/items", httpx.Handle(logger, handler.SetProposalItems))
			router.Post("/proposals/{proposalID}/accept", httpx.Handle(logger, handler.AcceptProposal))
			router.Post("/proposals/{proposalID}/reject", httpx.Handle(logger, handler.RejectProposal))
			router.Post("/proposals/{proposalID}/cancel", httpx.Handle(logger, handler.CancelProposal))
			router.Post("/proposals/{proposalID}/terminate", httpx.Handle(logger, handler.TerminatePartnership))

			// Riwayat kunjungan & nota konsinyasi: terbaca oleh kedua belah pihak.
			router.Get("/visits", httpx.Handle(logger, handler.ListVisits))
			router.Get("/visits/{visitID}", httpx.Handle(logger, handler.GetVisit))

			// Area pemilik toko.
			router.Group(func(router chi.Router) {
				router.Use(httpx.RequireAccountTypes(identity.AccountTypePemilikToko))

				router.Get("/stores/mine", httpx.Handle(logger, handler.MyStore))
				router.Post("/stores", httpx.Handle(logger, handler.CreateStore))
				router.Put("/stores/mine", httpx.Handle(logger, handler.UpdateMyStore))
				router.Patch("/stores/mine/status", httpx.Handle(logger, handler.SetMyStoreActive))
				router.Put("/stores/mine/photo", httpx.Handle(logger, handler.PutMyStorePhoto))
				router.Delete("/stores/mine/photo", httpx.Handle(logger, handler.DeleteMyStorePhoto))

				// Katalog produsen penitip (beserta produk aktif) untuk ditawari.
				router.Get("/producers", httpx.Handle(logger, handler.ListProducers))

				// Nota penjualan & kunjungan buatan pemilik toko.
				router.Post("/store-visits", httpx.Handle(logger, handler.CreateStoreVisit))
				router.Get("/store-visits", httpx.Handle(logger, handler.ListStoreVisits))
			})

			// Area penitip (produsen): seluruh alur bisnis yang sudah ada.
			router.Group(func(router chi.Router) {
				router.Use(httpx.RequireAccountTypes(identity.AccountTypePenitip))

				router.Get("/business", httpx.Handle(logger, handler.GetBusiness))
				router.Patch("/business", httpx.Handle(logger, handler.UpdateBusiness))

				router.Get("/products", httpx.Handle(logger, handler.ListProducts))
				router.Post("/products", httpx.Handle(logger, handler.CreateProduct))
				router.Put("/products/{productID}", httpx.Handle(logger, handler.UpdateProduct))
				router.Patch("/products/{productID}/status", httpx.Handle(logger, handler.SetProductActive))
				router.Put("/products/{productID}/photo", httpx.Handle(logger, handler.PutProductPhoto))
				router.Delete("/products/{productID}/photo", httpx.Handle(logger, handler.DeleteProductPhoto))

				router.Get("/partners", httpx.Handle(logger, handler.ListPartners))
				router.Post("/partners", httpx.Handle(logger, handler.CreatePartner))
				router.Put("/partners/{partnerID}", httpx.Handle(logger, handler.UpdatePartner))
				router.Patch("/partners/{partnerID}/status", httpx.Handle(logger, handler.SetPartnerActive))

				router.Post("/visits", httpx.Handle(logger, handler.CreateVisit))

				router.Get("/invoices", httpx.Handle(logger, handler.ListInvoices))
				router.Get("/invoices/{invoiceID}", httpx.Handle(logger, handler.GetInvoice))

				router.Post("/payments", httpx.Handle(logger, handler.CreatePayment))
				router.Post("/payments/{paymentID}/void", httpx.Handle(logger, handler.VoidPayment))

				router.Get("/dashboard", httpx.Handle(logger, handler.Dashboard))
			})
		})
	})

	return router
}
