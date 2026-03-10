# Backend (Ops API)

This directory contains the Go backend for internal Ops Console APIs.

## Runtime
- Go 1.26.0

## Structure
- `cmd/api`: HTTP server entrypoint
- `cmd/migrate`: SQL migration runner
- `internal/config`: environment config loader
- `internal/http`: routes, handlers, middleware
- `internal/repository`: interfaces + memory/postgres adapters
- `migrations`: SQL migration files
- `sql`: sqlc query definitions
- `api/openapi.yaml`: API contract

## Environment variables
- `OPS_API_ADDR` (default `:8080`)
- `OPS_API_ADMIN_TOKEN` (required for protected routes)
- `OPS_API_ALLOW_MEMORY_FALLBACK` (optional; default `false`)
- `OPS_API_DB_DSN` (required unless `OPS_API_ALLOW_MEMORY_FALLBACK=true`)
- `OPS_API_DB_DRIVER` (default `postgres`)
- `OPS_API_READ_TIMEOUT_SEC` (default `10`)
- `OPS_API_WRITE_TIMEOUT_SEC` (default `10`)

### .env usage
1) Create local env file:
```bash
cd apps/backend
cp .env.example .env
```

2) Load env values into current shell:
```bash
set -a
source .env
set +a
```

3) Start API:
```bash
go run ./cmd/api
```

Use memory mode only for local smoke tests. In normal environments, leave
`OPS_API_ALLOW_MEMORY_FALLBACK` unset and provide `OPS_API_DB_DSN`.

## Commands
```bash
cd apps/backend
make run-api
make migrate
make test
```
