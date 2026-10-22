package httpx

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

func TestCORSAllowsConfiguredOrigin(t *testing.T) {
	handler := CORS([]string{"http://localhost:3000", "https://app.titipku.id"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	request := httptest.NewRequest(http.MethodOptions, "/v1/visits", nil)
	request.Header.Set("Origin", "http://localhost:3000")
	request.Header.Set("Access-Control-Request-Method", "POST")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Fatalf("allow origin = %q", got)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, "POST") {
		t.Fatalf("allow methods = %q", got)
	}
	if got := recorder.Header().Get("Access-Control-Expose-Headers"); !strings.Contains(got, "X-Request-ID") {
		t.Fatalf("expose headers = %q", got)
	}
}

func TestCORSRejectsUnknownOrigin(t *testing.T) {
	handler := CORS([]string{"http://localhost:3000"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	request := httptest.NewRequest(http.MethodGet, "/v1/visits", nil)
	request.Header.Set("Origin", "https://evil.example")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("allow origin = %q, want empty", got)
	}
}

func TestRateLimitReturnsTooManyRequests(t *testing.T) {
	handler := RateLimit(2, time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	for i := 0; i < 2; i++ {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/public/invoices/token", nil))
		if recorder.Code != http.StatusNoContent {
			t.Fatalf("request %d status = %d, want %d", i+1, recorder.Code, http.StatusNoContent)
		}
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/public/invoices/token", nil))
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusTooManyRequests)
	}
	if got := recorder.Header().Get("Retry-After"); got != "60" {
		t.Fatalf("retry after = %q, want 60", got)
	}
	if !strings.Contains(recorder.Body.String(), "RATE_LIMITED") {
		t.Fatalf("body = %s", recorder.Body.String())
	}
}

func TestRateLimitTracksClientsSeparately(t *testing.T) {
	handler := RateLimit(1, time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	first := httptest.NewRequest(http.MethodGet, "/v1/public/invoices/token", nil)
	first.RemoteAddr = "192.0.2.1:1234"
	handler.ServeHTTP(httptest.NewRecorder(), first)

	second := httptest.NewRequest(http.MethodGet, "/v1/public/invoices/token", nil)
	second.RemoteAddr = "192.0.2.2:1234"
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, second)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
}

func TestAccessLogUsesRoutePattern(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	router := chi.NewRouter()
	router.Use(AccessLog(logger))
	router.Get("/v1/public/invoices/{token}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/v1/public/invoices/secret-token", nil))

	logged := output.String()
	if !strings.Contains(logged, "/v1/public/invoices/{token}") {
		t.Fatalf("log does not contain route pattern: %s", logged)
	}
	if strings.Contains(logged, "secret-token") {
		t.Fatalf("log leaked raw path: %s", logged)
	}
}
