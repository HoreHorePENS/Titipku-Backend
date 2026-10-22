# syntax=docker/dockerfile:1

ARG GO_VERSION=1.27

FROM golang:${GO_VERSION}-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG SERVICE
RUN test -n "$SERVICE"
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./services/${SERVICE}/cmd/server

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app \
    && adduser -S -G app app

WORKDIR /app

COPY --from=builder /out/server /app/server

USER app

ENTRYPOINT ["/app/server"]
