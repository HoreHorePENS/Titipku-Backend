package httpx

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/titipku/titipku-backend/internal/platform/apperror"
)

type HandlerFunc func(w http.ResponseWriter, r *http.Request) error

func Handle(logger *slog.Logger, handler HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := handler(w, r); err != nil {
			appErr := apperror.From(err)

			if appErr.Kind == apperror.KindInternal {
				attributes := []any{
					"request_id", requestIDFromContext(r.Context()),
					"method", r.Method,
					"path", r.URL.Path,
					"code", appErr.Code,
				}
				if cause := errors.Unwrap(appErr); cause != nil {
					attributes = append(attributes, "error", cause)
				} else {
					attributes = append(attributes, "error", appErr.Message)
				}
				logger.Error("request failed", attributes...)
			}

			WriteError(w, r, appErr)
		}
	}
}
