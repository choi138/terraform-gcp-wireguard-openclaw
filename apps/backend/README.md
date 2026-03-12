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
- `OPS_API_ADMIN_TOKEN` (required unless OIDC is configured)
- `OPS_API_INGEST_TOKEN` (required for ingest routes; must differ from `OPS_API_ADMIN_TOKEN`)
- `OPS_API_ALLOW_MEMORY_FALLBACK` (optional; default `false`)
- `OPS_API_DB_DSN` (required unless `OPS_API_ALLOW_MEMORY_FALLBACK=true`)
- `OPS_API_DB_DRIVER` (default `postgres`)
- `OPS_API_READ_TIMEOUT_SEC` (default `10`)
- `OPS_API_WRITE_TIMEOUT_SEC` (default `10`)
- `OPS_API_INGEST_MAX_BODY_BYTES` (default `1048576`)
- `OPS_API_SECURITY_MAX_BODY_BYTES` (default `1048576`)
- `OPS_API_INGEST_RETRY_BASE_DELAY_MS` (default `1000`)
- `OPS_API_INGEST_RETRY_MAX_DELAY_MS` (default `30000`)
- `OPS_API_INGEST_RETRY_MAX_ATTEMPTS` (default `5`)
- `OPS_API_INGEST_RETRY_WORKER_INTERVAL_MS` (default `1000`)
- `OPS_API_INGEST_RETRY_BATCH_SIZE` (default `20`)
- `OPS_API_OIDC_ISSUER`, `OPS_API_OIDC_AUDIENCE`, `OPS_API_OIDC_JWKS_URL` (enable operator OIDC mode)
- `OPS_API_OIDC_ROLES_CLAIM` (default `roles`)
- `OPS_API_OIDC_SUBJECT_CLAIM` (default `sub`)
- `OPS_API_OIDC_CLOCK_SKEW_SEC` (default `60`)
- `OPS_API_ADMIN_TOKEN_COMPATIBILITY` (optional coexistence switch for legacy admin token)
- `OPS_API_BREAK_GLASS_*` (optional emergency token controls; requires OIDC)
- `OPS_API_TRACE_EXPORTER` (`stdout` or `none`)
- `OPS_API_TRACE_SAMPLE_RATE` (default `0.1`)
- `OPS_API_TRACE_SERVICE_NAME` (default `ops-api`)
- `OPS_API_RETENTION_ENABLED`, `OPS_API_RETENTION_DRY_RUN`
- `OPS_API_RETENTION_INTERVAL_SEC`, `OPS_API_RETENTION_MAX_ROWS_PER_RUN`
- `OPS_API_RETENTION_MIN_WINDOW_HOURS`
- `OPS_API_RETENTION_RAW_MESSAGE_HOURS`
- `OPS_API_RETENTION_AUDIT_EVENT_HOURS`
- `OPS_API_RETENTION_INFRA_SNAPSHOT_HOURS`
- `OPS_API_RETENTION_INGEST_EVENT_HOURS`

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

## Operator access modes

- Legacy mode: `OPS_API_ADMIN_TOKEN` protects operator routes.
- OIDC mode: configure issuer, audience, and JWKS URL; operator roles are `viewer`, `auditor`, and `admin`.
- `OPS_API_INGEST_TOKEN` remains a distinct `ingest` service principal for producer-only routes.

Role matrix:

- `viewer`: dashboard, conversations, infra, ingest status, `/metrics`
- `auditor`: viewer access plus `GET /v1/security/findings`
- `admin`: all operator routes including `POST /v1/security/analyze-tfvars`

When OIDC is enabled:

- plain bearer tokens are rejected on operator routes by default
- `OPS_API_ADMIN_TOKEN_COMPATIBILITY=true` keeps the legacy admin token active during migration
- break-glass access can be enabled explicitly with `OPS_API_BREAK_GLASS_*`

Every successful break-glass request writes an `auth.break_glass` audit event.

## Commands
```bash
cd apps/backend
make run-api
make migrate
make test
```

## Ingest producer contract

Internal producers authenticate with `Authorization: Bearer <OPS_API_INGEST_TOKEN>`.
`OPS_API_INGEST_TOKEN` is required and must be distinct from `OPS_API_ADMIN_TOKEN` so producer credentials cannot call admin-only routes.

All ingest payloads must include:
- `schema_version`: currently `1`
- `source`: stable producer identifier such as `openclaw` or `wireguard`
- `event_id`: producer-unique idempotency key within each `event_type`

### `POST /v1/ingest/conversation-events`
- Upserts account and conversation state.
- Optionally upserts one masked message record.
- Re-delivering the same `source + event_id` is treated as duplicate and will not create a second logical write.

Example:
```json
{
  "schema_version": 1,
  "source": "openclaw",
  "event_id": "evt-123",
  "occurred_at": "2026-03-11T08:00:05Z",
  "account": {
    "external_id": "acct-1",
    "email": "ops@example.com",
    "status": "active"
  },
  "conversation": {
    "external_id": "conv-1",
    "channel": "telegram",
    "status": "completed",
    "started_at": "2026-03-11T08:00:00Z"
  },
  "message": {
    "external_id": "msg-1",
    "role": "user",
    "content_masked": "deploy wireguard",
    "created_at": "2026-03-11T08:00:05Z"
  }
}
```

### `POST /v1/ingest/infra-snapshot`
- Appends a historical infra snapshot row.
- Upserts `infra_status_latest` for the most recent source state.

### `POST /v1/ingest/request-attempt`
- Upserts account and conversation context.
- Persists provider/model/token/cost/latency metrics keyed by producer attempt identity.

## Retry and status behavior

- Transient persistence failures are scheduled with bounded exponential backoff.
- Events that exhaust `OPS_API_INGEST_RETRY_MAX_ATTEMPTS` move to a dead-letter state.
- Operators can inspect queue depth, retry lag, and dead-letter counters via `GET /v1/ingest/status`.

## Tfvars security analysis

The API provides an internal tfvars analysis workflow:

- `POST /v1/security/analyze-tfvars`: analyze a tfvars JSON payload, return normalized findings, and upsert persisted lifecycle records.
- `GET /v1/security/findings`: read persisted findings with status/severity filtering, pagination, and ordering.
- `POST /v1/security/analyze-tfvars` requires `admin`.
- `GET /v1/security/findings` requires `auditor` or `admin`.

Example request:

```json
{
  "schema_version": 1,
  "tfvars": {
    "openclaw_enable_public_ip": true,
    "ui_source_ranges": ["0.0.0.0/0"],
    "ssh_source_ranges": ["0.0.0.0/0"],
    "enable_project_oslogin": false,
    "wgeasy_password_secret": "projects/demo/secrets/plain-openclaw-password/versions/latest",
    "openclaw_openai_api_key_secret": "projects/demo/secrets/openai-api-token/versions/latest",
    "wg_port": 51820,
    "wgeasy_ui_port": 51821,
    "openclaw_gateway_port": 18789
  }
}
```

Example invocation:

```bash
curl -s \
  -H 'Authorization: Bearer admin-token' \
  -H 'Content-Type: application/json' \
  -X POST http://localhost:8080/v1/security/analyze-tfvars \
  -d @tfvars-analysis.json
```

### Baseline rule catalog

- `openclaw-public-ip-enabled`: flags `openclaw_enable_public_ip=true` as `critical`.
- `wgeasy-ui-exposed-to-world`: flags `ui_source_ranges` containing `0.0.0.0/0` or `::/0` as `high`.
- `ssh-exposed-to-world`: flags `ssh_source_ranges` containing `0.0.0.0/0` or `::/0` as `high`.
- `project-oslogin-disabled`: flags disabled or unset `enable_project_oslogin` as `medium`.
- `wgeasy-password-secret-used`: flags use of `wgeasy_password_secret` instead of the hash secret as `medium`.
- `secret-reference-not-version-pinned`: flags Secret Manager references that are unpinned or pinned to `latest` as `info`.
- `default-management-port-retained`: flags default `wg_port`, `wgeasy_ui_port`, and `openclaw_gateway_port` values as `info`.

### Severity semantics

- `critical`: breaks the intended VPN-only boundary or creates directly exploitable exposure.
- `high`: materially weakens network or operator access controls.
- `medium`: increases compromise impact or weakens control-plane hygiene.
- `info`: worth triaging for drift, exposure reduction, or hardening intent.

New findings are persisted with lifecycle state `open`. Existing findings keep their current state unless they were previously `resolved`, in which case a new detection reopens them as `open`.

### Triage workflow

1. Run `POST /v1/security/analyze-tfvars` with the candidate tfvars payload.
2. Review returned findings and prioritize `critical` and `high` severities first.
3. Query `GET /v1/security/findings?status=open&severity=critical,high` to drive the security panel or operator review.
4. Track remediation in your operational workflow; the current MVP persists lifecycle state and timestamps but does not yet expose mutation endpoints for acknowledgement or resolution changes.
5. Re-run analysis after tfvars changes. Matching fingerprints are upserted rather than duplicated, and previously resolved findings reopen if the unsafe condition returns.

### Audit and redaction behavior

- `POST /v1/security/analyze-tfvars` writes an audit event with action `security.analyze`.
- `GET /v1/security/findings` writes a standard read-audit event through the protected read middleware.
- Raw sensitive tfvars values are not persisted in finding titles, descriptions, or metadata. Secret references are masked before storage and response serialization.

## Observability and retention

- `GET /metrics` is available for authenticated internal observers with `viewer`, `auditor`, or `admin` access.
- Metrics include request volume, request errors, request latency histogram, ingest queue depth, retry lag, and service-up state.
- Structured request logs include `request_id`, `principal_id`, `path`, `latency_ms`, `status`, and `error_code`.
- Trace spans are emitted according to `OPS_API_TRACE_EXPORTER` and `OPS_API_TRACE_SAMPLE_RATE`.
- The retention worker compacts raw message payloads and deletes aged audit, infra snapshot, and ingest-event records on a schedule.
- Enable retention in dry-run mode first, review `retention.run` audit events, then switch to enforcement mode.

Operational rollout and verification guidance lives in `docs/operations.md`.

## Throughput validation

The repository includes an automated throughput guard in
`internal/ingest/service_test.go` that verifies 100 ingest events complete within
1 second in the in-memory representative test path:

```bash
cd apps/backend
go test ./internal/ingest -run TestIngestThroughputTarget100EventsPerSecond
```
