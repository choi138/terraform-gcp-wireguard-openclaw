-- name: GetDashboardSummary :one
SELECT
  COUNT(*)::bigint AS requests_total,
  COALESCE(SUM(tokens_in + tokens_out), 0)::bigint AS tokens_total,
  COALESCE(SUM(cost_usd), 0)::float8 AS cost_usd,
  COALESCE(AVG(CASE WHEN success THEN 0 ELSE 1 END), 0)::float8 AS error_rate
FROM request_attempts
WHERE created_at BETWEEN $1 AND $2;

-- name: GetDashboardTimeseries :many
SELECT
  date_trunc('hour', created_at) AS bucket_start,
  COUNT(*)::bigint AS requests
FROM request_attempts
WHERE created_at BETWEEN $1 AND $2
GROUP BY 1
ORDER BY 1 ASC;

-- name: ListConversations :many
SELECT id, account_id, channel, status, started_at, ended_at
FROM conversations
WHERE ($1::text = '' OR channel = $1)
  AND ($2::text = '' OR status = $2)
ORDER BY started_at DESC
LIMIT $3 OFFSET $4;

-- name: GetConversation :one
SELECT id, account_id, channel, status, started_at, ended_at
FROM conversations
WHERE id = $1;

-- name: ListConversationMessages :many
SELECT id, conversation_id, role, content_masked, created_at
FROM messages
WHERE conversation_id = $1
ORDER BY created_at ASC
LIMIT $2 OFFSET $3;

-- name: ListConversationAttempts :many
SELECT id, conversation_id, provider, model, tokens_in, tokens_out, cost_usd, latency_ms, success, error_code, created_at
FROM request_attempts
WHERE conversation_id = $1
ORDER BY created_at ASC
LIMIT $2 OFFSET $3;

-- name: GetLatestInfraSnapshot :one
SELECT id, vpn_peer_count, openclaw_up, cpu_pct, mem_pct, captured_at
FROM infra_snapshots
ORDER BY captured_at DESC
LIMIT 1;

-- name: ListInfraSnapshots :many
SELECT id, vpn_peer_count, openclaw_up, cpu_pct, mem_pct, captured_at
FROM infra_snapshots
WHERE captured_at BETWEEN $1 AND $2
ORDER BY captured_at DESC
LIMIT $3 OFFSET $4;

-- name: InsertAuditEvent :exec
INSERT INTO audit_events (actor, action, resource_type, resource_id, metadata_json, created_at)
VALUES ($1, $2, $3, $4, $5::jsonb, $6);
