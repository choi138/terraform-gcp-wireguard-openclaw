# Ops API Operations Runbook

## Identity Configuration

Operator routes support two modes:

- Legacy mode: `OPS_API_ADMIN_TOKEN` protects operator routes when OIDC is not configured.
- OIDC mode: set `OPS_API_OIDC_ISSUER`, `OPS_API_OIDC_AUDIENCE`, and `OPS_API_OIDC_JWKS_URL`.

Supported operator roles:

- `viewer`: read-only dashboard, conversation, infra, ingest status, and `/metrics`
- `auditor`: everything a viewer can read plus `GET /v1/security/findings`
- `admin`: all operator routes including `POST /v1/security/analyze-tfvars`

`OPS_API_INGEST_TOKEN` always remains a separate service principal for ingest writes.

Optional migration controls:

- `OPS_API_ADMIN_TOKEN_COMPATIBILITY=true` keeps the legacy admin token active while OIDC is rolled out.
- Leave compatibility disabled once operators have moved to OIDC.

## Break-Glass Access

Break-glass is disabled by default. To enable it, set:

- `OPS_API_BREAK_GLASS_ENABLED=true`
- `OPS_API_BREAK_GLASS_TOKEN`
- `OPS_API_BREAK_GLASS_ROLE`
- `OPS_API_BREAK_GLASS_EXPIRES_AT`
- `OPS_API_BREAK_GLASS_ALLOWED_PATHS`

Recommended practice:

1. Use a short expiry window.
2. Limit allowed paths to the smallest emergency surface.
3. Set `OPS_API_BREAK_GLASS_REASON` and `OPS_API_BREAK_GLASS_APPROVER`.
4. Rotate or disable the token immediately after use.

Every successful break-glass request writes an `auth.break_glass` audit event.

## Observability

The API exposes protected internal metrics at `GET /metrics`.

Key signals:

- `ops_api_http_requests_total`
- `ops_api_http_request_errors_total`
- `ops_api_http_request_duration_ms`
- `ops_api_ingest_queue_depth`
- `ops_api_ingest_oldest_retry_age_seconds`
- `ops_api_service_up`

Structured request logs include:

- `request_id`
- `principal_id`
- `path`
- `latency_ms`
- `status`
- `error_code`

Tracing controls:

- `OPS_API_TRACE_EXPORTER=stdout|none`
- `OPS_API_TRACE_SAMPLE_RATE`
- `OPS_API_TRACE_SERVICE_NAME`

Recommended alerts:

- Error rate: derive from `ops_api_http_request_errors_total / ops_api_http_requests_total`
- Ingest lag: `ops_api_ingest_oldest_retry_age_seconds > 120`
- Service down: scrape failure or missing `ops_api_service_up`

## Retention Rollout

Retention is disabled by default. Enable it with:

- `OPS_API_RETENTION_ENABLED=true`
- `OPS_API_RETENTION_DRY_RUN=true`
- `OPS_API_RETENTION_INTERVAL_SEC`
- `OPS_API_RETENTION_MAX_ROWS_PER_RUN`
- retention windows for raw messages, audit events, infra snapshots, and ingest events

Rollout order:

1. Enable dry-run mode.
2. Review `retention.run` audit events and worker logs.
3. Confirm candidate counts are reasonable.
4. Set `OPS_API_RETENTION_DRY_RUN=false`.

Safety guards:

- Each retention window must be at least `OPS_API_RETENTION_MIN_WINDOW_HOURS`.
- Each run is capped by `OPS_API_RETENTION_MAX_ROWS_PER_RUN`.
- Infra retention preserves the latest snapshot.

## Verification

OIDC validation:

```bash
curl -s \
  -H "Authorization: Bearer <oidc-access-token>" \
  "http://localhost:8080/v1/dashboard/summary?from=2026-03-11T00:00:00Z&to=2026-03-12T00:00:00Z"
```

Metrics scrape:

```bash
curl -s \
  -H "Authorization: Bearer <viewer-or-admin-token>" \
  http://localhost:8080/metrics
```

Local regression suite:

```bash
cd apps/backend
go test ./...
```
