package httpapi

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/titipku/titipku-backend/internal/platform/apperror"
	"github.com/titipku/titipku-backend/internal/platform/httpx"
	"github.com/titipku/titipku-backend/internal/platform/imageupload"
	"github.com/titipku/titipku-backend/services/core-service/internal/usecase"
)

type Handler struct {
	app            *usecase.App
	webBaseURL     string
	maxUploadBytes int64
}

func NewHandler(app *usecase.App, webBaseURL string) *Handler {
	return &Handler{
		app:            app,
		webBaseURL:     strings.TrimRight(webBaseURL, "/"),
		maxUploadBytes: imageupload.DefaultMaxBytes,
	}
}

// WithMaxUploadBytes mengatur batas ukuran file foto yang diterima.
func (h *Handler) WithMaxUploadBytes(maxBytes int64) *Handler {
	if maxBytes > 0 {
		h.maxUploadBytes = maxBytes
	}
	return h
}

func (h *Handler) scope(r *http.Request) (uuid.UUID, uuid.UUID, error) {
	principal, err := httpx.PrincipalFromRequest(r)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	if !principal.HasBusinessScope() {
		return uuid.Nil, uuid.Nil, apperror.Unauthorized("BUSINESS_SCOPE_REQUIRED", "access token is not scoped to a business")
	}
	return principal.BusinessID, principal.MemberID, nil
}

func pathUUID(r *http.Request, key string) (uuid.UUID, error) {
	value, err := uuid.Parse(chi.URLParam(r, key))
	if err != nil {
		return uuid.Nil, apperror.Validation(map[string]string{key: "must be a valid UUID"})
	}
	return value, nil
}

func (h *Handler) publicInvoiceURL(token string) string {
	return h.webBaseURL + "/i/" + url.PathEscape(token)
}

func (h *Handler) whatsappURL(phone string, visitNumber, businessName string, total int64, token string) string {
	message := "Halo, ini nota titipan " + businessName + " nomor " + visitNumber +
		". Total: " + formatRupiah(total) + ". Lihat nota: " + h.publicInvoiceURL(token)
	return "https://wa.me/" + url.PathEscape(phone) + "?text=" + url.QueryEscape(message)
}
