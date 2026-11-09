package httpx

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/titipku/titipku-backend/internal/platform/apperror"
)

type decodeTarget struct {
	Email string `json:"email"`
}

func decode(t *testing.T, body string) *apperror.Error {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	var target decodeTarget
	err := DecodeJSON(httptest.NewRecorder(), request, &target)
	if err == nil {
		return nil
	}
	var appErr *apperror.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *apperror.Error, got %T", err)
	}
	return appErr
}

func TestDecodeJSONUnknownFieldIsReportedPerField(t *testing.T) {
	appErr := decode(t, `{"email":"a@b.co","account_type":"penitip"}`)
	if appErr == nil {
		t.Fatal("expected an error for unknown field")
	}
	if got := appErr.Fields["account_type"]; got != apperror.MessageUnknownField {
		t.Fatalf("fields[account_type] = %q, want %q (fields=%v)", got, apperror.MessageUnknownField, appErr.Fields)
	}
	if _, leaked := appErr.Fields["body"]; leaked {
		t.Fatalf("unknown field must not be reported under body: %v", appErr.Fields)
	}
	if code := appErr.FieldCodes()["account_type"]; code != apperror.FieldUnknown {
		t.Fatalf("field code = %q, want %q", code, apperror.FieldUnknown)
	}
}

func TestDecodeJSONDoesNotLeakDecoderText(t *testing.T) {
	for _, body := range []string{`{"email":`, `[1,2]`, `{"email":"a"} {"email":"b"}`} {
		appErr := decode(t, body)
		if appErr == nil {
			t.Fatalf("expected error for %q", body)
		}
		for _, message := range appErr.Fields {
			if strings.Contains(message, "json:") {
				t.Fatalf("raw decoder text leaked for %q: %q", body, message)
			}
		}
	}
}

func TestWriteErrorIncludesFieldCodes(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/", nil)
	WriteError(recorder, request, apperror.Validation(map[string]string{
		"whatsapp": "must be a valid Indonesian whatsapp number",
		"password": "must be between 8 and 72 characters",
	}))

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", recorder.Code)
	}

	var body struct {
		Error struct {
			Code       string            `json:"code"`
			Fields     map[string]string `json:"fields"`
			FieldCodes map[string]string `json:"field_codes"`
		} `json:"error"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Error.Code != apperror.CodeValidation {
		t.Fatalf("code = %q", body.Error.Code)
	}
	if body.Error.FieldCodes["whatsapp"] != apperror.FieldInvalidFormat ||
		body.Error.FieldCodes["password"] != apperror.FieldInvalidLength {
		t.Fatalf("unexpected field_codes: %v", body.Error.FieldCodes)
	}
	if body.Error.Fields["whatsapp"] == "" {
		t.Fatal("fields must be kept for backward compatibility")
	}
}
