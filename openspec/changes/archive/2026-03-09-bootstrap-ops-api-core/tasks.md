## 1. Backend Foundation

- [x] 1.1 Create `backend/` module structure (`cmd/api`, `internal/*`, `migrations`, `api/openapi.yaml`, `Makefile`)
- [x] 1.2 Implement shared config loading, structured JSON logging, and startup wiring
- [x] 1.3 Add API router skeleton with `/v1/healthz` and `/v1/readyz`

## 2. Data Model and Persistence

- [x] 2.1 Author initial Postgres migrations for accounts, conversations, messages, request_attempts, infra_snapshots, and audit_events
- [x] 2.2 Add sqlc query definitions for dashboard aggregates, conversation reads, and infra status reads
- [x] 2.3 Implement repository interfaces and postgres adapters used by service layer

## 3. Core Read APIs

- [x] 3.1 Implement dashboard summary and timeseries endpoints with time-range validation
- [x] 3.2 Implement conversation list/detail/messages/attempts endpoints with pagination
- [x] 3.3 Implement infra status/snapshot endpoints backed by latest and ranged queries

## 4. Security, Audit, and Verification

- [x] 4.1 Add bearer-token authentication middleware for all non-health routes
- [x] 4.2 Insert audit_events for authenticated read requests with actor/action/resource metadata
- [x] 4.3 Add integration/unit tests and docs covering endpoint contracts and VPN-only deployment assumptions
