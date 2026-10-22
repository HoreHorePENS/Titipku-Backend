package httpapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/titipku/titipku-backend/internal/platform/apperror"
	"github.com/titipku/titipku-backend/internal/platform/httpx"
)

func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) error {
	businessID, _, err := h.scope(r)
	if err != nil {
		return err
	}

	var day time.Time
	if value := strings.TrimSpace(r.URL.Query().Get("date")); value != "" {
		parsed, err := time.ParseInLocation("2006-01-02", value, time.FixedZone("WIB", 7*60*60))
		if err != nil {
			return apperror.Validation(map[string]string{"date": "must use YYYY-MM-DD format"})
		}
		day = parsed
	}

	dashboard, err := h.app.Dashboard(r.Context(), businessID, day)
	if err != nil {
		return err
	}

	httpx.WriteData(w, r, http.StatusOK, newDashboardResponse(*dashboard))
	return nil
}
