## Why

The project currently provisions infrastructure and a static frontend, but it lacks a backend API that serves real operational data. We need an internal-only Ops API to power dashboard, conversation, and infra views from trusted data sources.

## What Changes

- Bootstrap a Go backend workspace under `apps/backend/` with a production-ready API entrypoint, shared config, and JSON logging.
- Add PostgreSQL schema and migration baseline for accounts, conversations, messages, request attempts, infra snapshots, and audit events.
- Implement read APIs for health/readiness, dashboard summary/timeseries, conversations (list/detail/messages/attempts), and infra status/snapshots.
- Enforce VPN-only network exposure and bearer-token authentication for all non-health endpoints.
- Document API contracts and operational assumptions for frontend integration and internal deployment.

## Capabilities

### New Capabilities
- `ops-api-core`: Internal Go Ops API for dashboard, conversation timeline/detail, infra status reads, and foundational auth/audit behavior.

### Modified Capabilities
- (none)

## Impact

- New backend code and structure under `apps/backend/` (`cmd`, `internal`, `migrations`, `api/openapi.yaml`, `Makefile`).
- Terraform deployment docs and environment wiring for internal API runtime and DB connectivity.
- Frontend integration points move from static placeholders to API-backed data fetch paths.
- Operational dependency on PostgreSQL availability for API reads.
