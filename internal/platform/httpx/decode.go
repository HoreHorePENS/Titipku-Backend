package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/titipku/titipku-backend/internal/platform/apperror"
)

const maxRequestBodyBytes = 1 << 20

func DecodeJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	if r.Body == nil {
		return apperror.Validation(map[string]string{"body": "request body is required"})
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(destination); err != nil {
		return decodeError(err)
	}

	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return apperror.Validation(map[string]string{"body": "body must contain a single JSON object"})
	}

	return nil
}

func decodeError(err error) error {
	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) {
		return apperror.Validation(map[string]string{"body": "request body contains invalid JSON"})
	}

	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		field := typeErr.Field
		if field == "" {
			field = "body"
		}
		return apperror.Validation(map[string]string{
			field: fmt.Sprintf("must be of type %s", typeErr.Type),
		})
	}

	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		return apperror.Validation(map[string]string{
			"body": fmt.Sprintf("must not exceed %d bytes", maxBytesErr.Limit),
		})
	}

	if errors.Is(err, io.EOF) {
		return apperror.Validation(map[string]string{"body": "request body is required"})
	}

	// encoding/json reports DisallowUnknownFields violations only as text:
	// `json: unknown field "name"`. Report them against the offending field.
	if field, ok := strings.CutPrefix(err.Error(), "json: unknown field "); ok {
		if name, unquoteErr := strconv.Unquote(field); unquoteErr == nil && name != "" {
			return apperror.Validation(map[string]string{name: apperror.MessageUnknownField})
		}
	}

	// Never echo raw decoder errors to clients.
	return apperror.Validation(map[string]string{"body": "request body contains invalid JSON"})
}
