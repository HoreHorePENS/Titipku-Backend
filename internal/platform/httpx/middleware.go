package httpx

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/titipku/titipku-backend/internal/platform/apperror"
	"github.com/titipku/titipku-backend/internal/platform/identity"
)

type requestIDContextKey struct{}

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if requestID == "" || len(requestID) > 128 {
			requestID = uuid.NewString()
		}

		w.Header().Set("X-Request-ID", requestID)
		ctx := context.WithValue(r.Context(), requestIDContextKey{}, requestID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func AccessLog(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startedAt := time.Now()
			recorder := &responseRecorder{ResponseWriter: w}

			next.ServeHTTP(recorder, r)

			logger.Info(
				"http request",
				"request_id", requestIDFromContext(r.Context()),
				"method", r.Method,
				"path", routePath(r),
				"status", recorder.statusCode(),
				"duration_ms", time.Since(startedAt).Milliseconds(),
				"remote_addr", r.RemoteAddr,
			)
		})
	}
}

func Recoverer(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				recovered := recover()
				if recovered == nil {
					return
				}
				if recoveredErr, ok := recovered.(error); ok && errors.Is(recoveredErr, http.ErrAbortHandler) {
					panic(recovered)
				}

				logger.Error(
					"panic recovered",
					"request_id", requestIDFromContext(r.Context()),
					"method", r.Method,
					"path", routePath(r),
					"panic", fmt.Sprint(recovered),
					"stack", string(debug.Stack()),
				)

				WriteError(w, r, apperror.Internal(apperror.CodeInternal, "internal server error"))
			}()

			next.ServeHTTP(w, r)
		})
	}
}

func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")

		next.ServeHTTP(w, r)
	})
}

type TokenVerifier interface {
	Verify(rawToken string) (identity.Principal, error)
}

func Authenticate(verifier TokenVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rawToken, err := bearerToken(r)
			if err != nil {
				WriteError(w, r, err)
				return
			}

			principal, err := verifier.Verify(rawToken)
			if err != nil {
				WriteError(w, r, apperror.Unauthorized(apperror.CodeUnauthorized, "invalid or expired access token"))
				return
			}

			next.ServeHTTP(w, r.WithContext(identity.WithPrincipal(r.Context(), principal)))
		})
	}
}

func RequireRoles(roles ...identity.Role) func(http.Handler) http.Handler {
	allowed := make(map[identity.Role]struct{}, len(roles))
	for _, role := range roles {
		allowed[role] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := identity.PrincipalFrom(r.Context())
			if !ok {
				WriteError(w, r, apperror.Unauthorized(apperror.CodeUnauthorized, "authentication is required"))
				return
			}

			if _, ok := allowed[principal.Role]; !ok {
				WriteError(w, r, apperror.Forbidden(apperror.CodeForbidden, "you do not have permission to perform this action"))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func bearerToken(r *http.Request) (string, error) {
	authorization := strings.TrimSpace(r.Header.Get("Authorization"))
	parts := strings.Fields(authorization)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", apperror.Unauthorized(apperror.CodeUnauthorized, "a bearer access token is required")
	}
	return parts[1], nil
}

type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (r *responseRecorder) WriteHeader(status int) {
	if r.status != 0 {
		return
	}
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *responseRecorder) Write(data []byte) (int, error) {
	if r.status == 0 {
		r.WriteHeader(http.StatusOK)
	}

	return r.ResponseWriter.Write(data)
}

func (r *responseRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

func (r *responseRecorder) statusCode() int {
	if r.status == 0 {
		return http.StatusOK
	}
	return r.status
}

func routePath(r *http.Request) string {
	if routeContext := chi.RouteContext(r.Context()); routeContext != nil {
		if pattern := routeContext.RoutePattern(); pattern != "" {
			return pattern
		}
	}
	return r.URL.Path
}

func RateLimit(limit int, window time.Duration) func(http.Handler) http.Handler {
	type clientWindow struct {
		start time.Time
		count int
	}

	var (
		mu          sync.Mutex
		clients     = make(map[string]*clientWindow)
		nextCleanup time.Time
	)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if limit <= 0 || window <= 0 {
				next.ServeHTTP(w, r)
				return
			}

			now := time.Now()
			client := clientIP(r)

			mu.Lock()
			if nextCleanup.IsZero() || now.After(nextCleanup) {
				for key, entry := range clients {
					if now.Sub(entry.start) >= window {
						delete(clients, key)
					}
				}
				nextCleanup = now.Add(window)
			}

			entry, ok := clients[client]
			if !ok || now.Sub(entry.start) >= window {
				entry = &clientWindow{start: now}
				clients[client] = entry
			}
			entry.count++
			allowed := entry.count <= limit
			retryAfter := int(window.Seconds())
			mu.Unlock()

			if !allowed {
				w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
				WriteError(w, r, apperror.New(apperror.KindTooManyRequests, apperror.CodeRateLimited, "too many requests, please try again later"))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			allowed[origin] = struct{}{}
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := strings.TrimSpace(r.Header.Get("Origin"))
			if origin != "" {
				if _, ok := allowed[origin]; ok {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Vary", "Origin")
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
					w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID")
					w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID, Retry-After")
					w.Header().Set("Access-Control-Max-Age", "600")
				}
			}

			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
