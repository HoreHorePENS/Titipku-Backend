package httpx

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/titipku/titipku-backend/internal/platform/apperror"
)

type response struct {
	Data      any            `json:"data,omitempty"`
	Meta      any            `json:"meta,omitempty"`
	Error     *errorResponse `json:"error,omitempty"`
	RequestID string         `json:"request_id,omitempty"`
}

type errorResponse struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

type PageMeta struct {
	Total  int64 `json:"total"`
	Limit  int   `json:"limit"`
	Offset int   `json:"offset"`
}

func WriteData(w http.ResponseWriter, r *http.Request, status int, data any) {
	write(w, status, response{
		Data:      data,
		RequestID: requestIDFromContext(r.Context()),
	})
}

func WritePage(w http.ResponseWriter, r *http.Request, status int, data any, total int64, limit, offset int) {
	write(w, status, response{
		Data: data,
		Meta: PageMeta{
			Total:  total,
			Limit:  limit,
			Offset: offset,
		},
		RequestID: requestIDFromContext(r.Context()),
	})
}

func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	appErr := apperror.From(err)

	write(w, statusForKind(appErr.Kind), response{
		Error: &errorResponse{
			Code:    appErr.Code,
			Message: appErr.Message,
			Fields:  appErr.Fields,
		},
		RequestID: requestIDFromContext(r.Context()),
	})
}

func write(w http.ResponseWriter, status int, payload response) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(payload)
}

func statusForKind(kind apperror.Kind) int {
	switch kind {
	case apperror.KindInvalid:
		return http.StatusBadRequest
	case apperror.KindUnauthorized:
		return http.StatusUnauthorized
	case apperror.KindForbidden:
		return http.StatusForbidden
	case apperror.KindNotFound:
		return http.StatusNotFound
	case apperror.KindConflict:
		return http.StatusConflict
	case apperror.KindUnavailable:
		return http.StatusServiceUnavailable
	case apperror.KindMethodNotAllowed:
		return http.StatusMethodNotAllowed
	case apperror.KindTooManyRequests:
		return http.StatusTooManyRequests
	default:
		return http.StatusInternalServerError
	}
}

func requestIDFromContext(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDContextKey{}).(string)
	return requestID
}
