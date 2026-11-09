# Titipku Backend

Go backend for the Titipku UMKM consignment application.

## Services

| Service | Port | Database | Responsibility |
| --- | --- | --- | --- |
| core-service | 8080 | `titipku` | Auth, business, products, partners, visits, stock, invoices, payments, public invoice, dashboard |

`core-service` is a **modular monolith**. The `auth-service`, `user-service`, and `order-service` directories are the original marketplace scaffold, retained for reference and not deployed by the current infrastructure.

## Architecture

```text
cmd -> adapter -> usecase -> domain
                    |
                    +-> repository ports (implemented by adapter)
```

- `domain` contains entities, validation, and invariants.
- `usecase` contains application workflows and defines the ports it needs.
- `adapter` contains HTTP handlers, DTOs, PostgreSQL repositories, and PostgreSQL function calls.
- `cmd` wires configuration, infrastructure, and application dependencies.
- `internal/platform` contains reusable technical building blocks.

The critical visit and payment flows run through PostgreSQL functions inside one transaction. See [the architecture document](../Titipku-Infrastructure/docs/architecture.md) and [the ERD](../Titipku-Infrastructure/docs/erd.md).

## Requirements

- Go 1.26 or newer
- PostgreSQL 17
- Docker with Compose for the recommended local environment

## Quick start

From the workspace root:

```powershell
Set-Location Titipku-Infrastructure
Copy-Item .env.example .env
docker compose up -d --build
```

Health checks:

```text
http://localhost:8080/healthz
http://localhost:8080/readyz
```

## Local development without Docker

Create a PostgreSQL database, copy the environment file, and run the service:

```powershell
Copy-Item services/core-service/.env.example services/core-service/.env
go run ./services/core-service/cmd/server
```

Migrations run automatically at startup.

| Variable | Required | Default | Purpose |
| --- | --- | --- | --- |
| `APP_ENV` | No | `development` | Log verbosity and environment label |
| `HTTP_PORT` | No | `8080` | HTTP listen port |
| `DATABASE_URL` | Yes | - | PostgreSQL connection string |
| `JWT_SECRET` | Yes | - | HMAC secret, at least 32 characters |
| `JWT_ISSUER` | No | `titipku` | Expected JWT issuer |
| `JWT_TTL` | No | `24h` | Access token lifetime |
| `HTTP_SHUTDOWN_TIMEOUT` | No | `10s` | Graceful shutdown deadline |
| `PUBLIC_WEB_URL` | No | `http://localhost:3000` | Frontend base URL for invoice links |
| `CORS_ALLOWED_ORIGINS` | No | `http://localhost:3000` | Comma-separated browser origins |
| `PUBLIC_RATE_LIMIT_PER_MINUTE` | No | `60` | Per-IP public invoice limit; `0` disables it |
| `UPLOAD_DIR` | No | `/data/uploads` | Directory for store/product photos, served at `/uploads/*`. **Mount it as a volume in Docker.** |

## API documentation

The OpenAPI contract lives in [`Titipku-Infrastructure/specs/openapi/core-service.yaml`](../Titipku-Infrastructure/specs/openapi/core-service.yaml).

Conventions:

- Versioned paths under `/v1`.
- Successful responses use a `data` envelope with optional `meta`.
- Errors use an `error` object with a stable `code`, a developer-facing `message`, and on `400` per-field `fields` plus machine-readable `field_codes`. See [docs/api-errors.md](docs/api-errors.md).
- Protected endpoints require `Authorization: Bearer <token>`.
- List endpoints use `limit` and `offset`.
- Public invoice links use `/v1/public/invoices/{token}` and require no authentication. They are rate-limited per client IP.

### Consignment proposals & producers

Two-way partnership requests between a store (`pemilik_toko`) and a producer (`penitip`):

| Method & path | Who | Purpose |
| --- | --- | --- |
| `GET /v1/proposals?direction=&status=` | both | Proposals involving the caller (`store_to_penitip` = offer, `penitip_to_store` = request) |
| `POST /v1/proposals` | both | Store owner sends `business_id` (+ message, shelf_location, estimated_capacity); producer sends `store_id` (+ message, items `[{product_id, qty}]`) |
| `POST /v1/proposals/{id}/accept` / `reject` | recipient | Accepting links the store as an active partner of the producer |
| `POST /v1/proposals/{id}/cancel` | sender | Cancel while pending |
| `GET /v1/producers?q=` | store owner | Producers with at least one active product, their products and relationship status |

Conflicts: `409 PROPOSAL_PENDING`, `409 ALREADY_PARTNERS`, `409 PROPOSAL_NOT_PENDING`, `409 STORE_INACTIVE`.

### Photos

| Method & path | Who |
| --- | --- |
| `PUT` / `DELETE /v1/stores/mine/photo` | store owner |
| `PUT` / `DELETE /v1/products/{id}/photo` | producer owning the product |

`PUT` takes `multipart/form-data` with field `file` (JPG/PNG/WebP, max 5 MB, max 8000 px per side). The type is detected from the file content, EXIF/GPS/XMP metadata is removed, and the file gets a random name. Files are served publicly at `/uploads/<random>.<ext>` (`nosniff`, immutable cache); replaced photos are deleted. `photo_url` / `photo_path` in the JSON store/product endpoints are ignored; photos change only through these endpoints. Validation errors use `field_codes.file` (`invalid_choice`, `invalid_length`, `out_of_range`, `invalid_format`).

## Common commands

```powershell
go test ./...
go vet ./...
go build ./...
gofmt -w .
```

Or with Make:

```text
make test
make build
make run SERVICE=core-service
make lint
```

## Core flows

### Register and login

`POST /v1/auth/register` creates the owner, business, and membership atomically. The returned JWT is scoped to `business_id` and `member_id`.

### Record a visit

`POST /v1/visits` accepts client-calculated field quantities only. The server re-reads previous stock and unit price, then saves the visit, FIFO batch movements, invoice, and optional payment in one transaction.

`client_request_id` makes retries safe. A duplicate key returns the existing visit; the same key with a different payload returns `409`.

### Record a payment

`POST /v1/payments` allocates the amount to the oldest unpaid invoices first. It rejects overpayment. `POST /v1/payments/{paymentID}/void` reverses allocations without deleting history.

### Share an invoice

The visit response includes a public invoice URL and a `wa.me` share URL. Warung partners open the public link without an account.

## Legacy scaffold

The `services/auth-service`, `services/user-service`, and `services/order-service` packages remain only for reference. Their customer/courier/order model does not match the consignment domain and they are excluded from the Compose stack.


## Docker & CI

The `Dockerfile` builds a single service (default `core-service`) into a static binary on `golang:1.26.10-alpine3.24` and runs it on `alpine:3.24.4` as the non-root user `app` (UID 10001). The image contains only the binary — no source and no `.env`; configuration comes from environment variables at runtime. The container listens on **8080** and has a `HEALTHCHECK` against `/healthz`.

### Build & run locally

```bash
docker build -t titipku-backend:local .
```

The service needs PostgreSQL at startup (migrations run automatically):

```bash
docker run --rm -p 8080:8080 \
  -v titipku-uploads:/data/uploads \
  -e DATABASE_URL="postgres://titipku:titipku@host.docker.internal:5432/titipku?sslmode=disable" \
  -e JWT_SECRET="<at-least-32-characters>" \
  -e CORS_ALLOWED_ORIGINS="http://localhost:3000" \
  titipku-backend:local
```

Photos are written to `UPLOAD_DIR` (default `/data/uploads`, declared as a `VOLUME`). Always mount a named volume or host directory there, otherwise uploaded photos are lost when the container is recreated. In Compose:

```yaml
services:
  core-service:
    image: ghcr.io/horehorepens/titipku-backend:latest
    volumes:
      - titipku-uploads:/data/uploads
volumes:
  titipku-uploads:
```

Then check `http://localhost:8080/healthz` and `http://localhost:8080/readyz`. To build a legacy scaffold service instead, pass `--build-arg SERVICE=auth-service` (or `user-service`, `order-service`).

### Image on GitHub Container Registry

`.github/workflows/docker.yml` builds `core-service` on every push/PR and publishes it as:

```
ghcr.io/horehorepens/titipku-backend
```

| Event | Pushed to GHCR? | Tags |
| --- | --- | --- |
| push to `main` | yes | `main`, `sha-<7 chars>`, `latest` |
| push to `dev` | yes | `dev`, `sha-<7 chars>` |
| `workflow_dispatch` (manual) | yes | `<branch-name>`, `sha-<7 chars>` (+ `latest` when run on `main`) |
| pull request to `main`/`dev` | no (build only) | — |

### Pull the image

```bash
# For a private package, log in first with a PAT that has read:packages
echo "$GHCR_TOKEN" | docker login ghcr.io -u <github-username> --password-stdin

docker pull ghcr.io/horehorepens/titipku-backend:latest   # latest main build
docker pull ghcr.io/horehorepens/titipku-backend:dev      # latest dev build
```

No build-time variables or secrets are needed. Runtime variables (`DATABASE_URL`, `JWT_SECRET`, …) are set by the deployment environment — see the table above.
