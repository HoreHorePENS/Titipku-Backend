# syntax=docker/dockerfile:1

# Versi Go mengikuti go.mod (go 1.26.x), dikunci ke patch spesifik.
ARG GO_VERSION=1.26.10
ARG ALPINE_VERSION=3.24

# ── Stage 1: Build ──────────────────────────────────────────────
FROM golang:${GO_VERSION}-alpine${ALPINE_VERSION} AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .

# Service yang di-build. core-service adalah satu-satunya service yang di-deploy;
# service lain (auth/user/order) hanya scaffold lama.
ARG SERVICE=core-service
RUN test -d "./services/${SERVICE}/cmd/server" || (echo "unknown SERVICE=${SERVICE}" && exit 1)
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./services/${SERVICE}/cmd/server

# ── Stage 2: Runtime ────────────────────────────────────────────
FROM alpine:3.24.4

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S -g 10001 app \
    && adduser -S -D -H -u 10001 -G app app \
    && mkdir -p /data/uploads \
    && chown app:app /data/uploads

WORKDIR /app

# Hanya binary yang disalin — tidak ada .env atau source di image.
# Konfigurasi diberikan lewat environment variable saat container dijalankan.
COPY --from=builder /out/server /app/server

USER 10001:10001

ENV HTTP_PORT=8080
# Foto toko/produk disimpan di sini. WAJIB di-mount sebagai volume agar tidak
# hilang saat container dibuat ulang, mis. -v titipku-uploads:/data/uploads
ENV UPLOAD_DIR=/data/uploads
VOLUME ["/data/uploads"]
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=15s --retries=3 \
  CMD wget -q -O /dev/null "http://127.0.0.1:${HTTP_PORT}/healthz" || exit 1

ENTRYPOINT ["/app/server"]
