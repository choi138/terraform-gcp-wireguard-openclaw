# Backend Ops API Contract (MVP)

## Network and access assumptions
- The API is **internal-only** and must be reachable only from VPN-connected clients.
- Every endpoint except `/v1/healthz` and `/v1/readyz` requires `Authorization: Bearer <admin-token>`.
- Authenticated GET requests are written to `audit_events` with actor/action/resource metadata.

## Endpoints
- `GET /v1/healthz`: process liveness check
- `GET /v1/readyz`: dependency readiness check
- `GET /v1/dashboard/summary?from=...&to=...`
- `GET /v1/dashboard/timeseries?metric=requests&bucket=1h&from=...&to=...`
- `GET /v1/conversations?channel=telegram&status=failed&page=1&page_size=50`
- `GET /v1/conversations/{id}`
- `GET /v1/conversations/{id}/messages?page=1&page_size=50`
- `GET /v1/conversations/{id}/attempts?page=1&page_size=50`
- `GET /v1/infra/status`
- `GET /v1/infra/snapshots?from=...&to=...&page=1&page_size=50`

## Query validation rules
- `from`/`to` are required for dashboard and infra ranged endpoints.
- Accepted time formats: RFC3339 (`2026-03-04T12:00:00Z`) or date (`2026-03-04`).
- `bucket` must be one of `1m`, `5m`, `1h`, `day`.
- `metric` must be one of `requests`, `tokens`, `cost`, `errors`.
- Pagination defaults: `page=1`, `page_size=50`; max page size is `200`.

## Local run (fallback memory mode)
When `OPS_API_DB_DSN` is not set, the API starts with in-memory repositories for smoke testing.

```bash
cd apps/backend
OPS_API_ADMIN_TOKEN=dev-token go run ./cmd/api
```

## Database mode
Set DB environment variables and run migrations before starting the API.

```bash
cd apps/backend
OPS_API_DB_DSN='postgres://user:pass@host:5432/db?sslmode=require' \
OPS_API_DB_DRIVER=postgres \
go run ./cmd/migrate

OPS_API_ADMIN_TOKEN='your-token' \
OPS_API_DB_DSN='postgres://user:pass@host:5432/db?sslmode=require' \
OPS_API_DB_DRIVER=postgres \
go run ./cmd/api
```
