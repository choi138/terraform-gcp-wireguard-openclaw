package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/domain"
	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/repository"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *Store) GetSummary(ctx context.Context, from, to time.Time) (domain.DashboardSummary, error) {
	const q = `
SELECT
  COUNT(*)::bigint,
  COALESCE(SUM(tokens_in + tokens_out), 0)::bigint,
  COALESCE(SUM(cost_usd), 0)::float8,
  COALESCE(AVG(CASE WHEN success THEN 0 ELSE 1 END), 0)::float8,
  (
    SELECT COUNT(DISTINCT account_id)::bigint
    FROM conversations
    WHERE started_at BETWEEN $1 AND $2
  )
FROM request_attempts
WHERE created_at BETWEEN $1 AND $2
`

	var out domain.DashboardSummary
	if err := s.db.QueryRowContext(ctx, q, from, to).Scan(
		&out.RequestsTotal,
		&out.TokensTotal,
		&out.CostUSD,
		&out.ErrorRate,
		&out.ActiveAccounts,
	); err != nil {
		return domain.DashboardSummary{}, err
	}
	return out, nil
}

func (s *Store) GetTimeseries(ctx context.Context, metric, bucket string, from, to time.Time) ([]domain.DashboardPoint, error) {
	if !domain.IsAllowedDashboardMetric(metric) {
		return nil, fmt.Errorf("%w: unsupported metric %q", repository.ErrInvalidInput, metric)
	}
	if !domain.IsAllowedDashboardBucket(bucket) {
		return nil, fmt.Errorf("%w: unsupported bucket %q", repository.ErrInvalidInput, bucket)
	}

	metricExpr, ok := metricExpression(metric)
	if !ok {
		return nil, fmt.Errorf("%w: unsupported metric %q", repository.ErrInvalidInput, metric)
	}
	bucketExpr, ok := bucketExpression(bucket)
	if !ok {
		return nil, fmt.Errorf("%w: unsupported bucket %q", repository.ErrInvalidInput, bucket)
	}

	query := fmt.Sprintf(`
SELECT %s AS bucket_start, %s AS value
FROM request_attempts
WHERE created_at BETWEEN $1 AND $2
GROUP BY 1
ORDER BY 1 ASC
`, bucketExpr, metricExpr)

	rows, err := s.db.QueryContext(ctx, query, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	points := make([]domain.DashboardPoint, 0)
	for rows.Next() {
		var p domain.DashboardPoint
		if err := rows.Scan(&p.BucketStart, &p.Value); err != nil {
			return nil, err
		}
		points = append(points, p)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return points, nil
}

func (s *Store) ListConversations(ctx context.Context, filter domain.ConversationFilter, pagination domain.Pagination) ([]domain.Conversation, error) {
	if pagination.Page < 1 {
		pagination.Page = 1
	}
	if pagination.PageSize <= 0 {
		pagination.PageSize = 50
	}
	offset := (pagination.Page - 1) * pagination.PageSize

	args := []any{}
	where := make([]string, 0)
	idx := 1
	if filter.Channel != "" {
		where = append(where, fmt.Sprintf("channel = $%d", idx))
		args = append(args, filter.Channel)
		idx++
	}
	if filter.Status != "" {
		where = append(where, fmt.Sprintf("status = $%d", idx))
		args = append(args, filter.Status)
		idx++
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = "WHERE " + strings.Join(where, " AND ")
	}

	query := fmt.Sprintf(`
SELECT id, account_id, channel, status, started_at, ended_at
FROM conversations
%s
ORDER BY started_at DESC, id DESC
LIMIT $%d OFFSET $%d
`, whereClause, idx, idx+1)
	args = append(args, pagination.PageSize, offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.Conversation, 0)
	for rows.Next() {
		var c domain.Conversation
		if err := rows.Scan(&c.ID, &c.AccountID, &c.Channel, &c.Status, &c.StartedAt, &c.EndedAt); err != nil {
			return nil, err
		}
		items = append(items, c)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) GetConversation(ctx context.Context, conversationID int64) (domain.Conversation, error) {
	const q = `
SELECT id, account_id, channel, status, started_at, ended_at
FROM conversations
WHERE id = $1
`
	var out domain.Conversation
	if err := s.db.QueryRowContext(ctx, q, conversationID).Scan(
		&out.ID,
		&out.AccountID,
		&out.Channel,
		&out.Status,
		&out.StartedAt,
		&out.EndedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Conversation{}, repository.ErrNotFound
		}
		return domain.Conversation{}, err
	}
	return out, nil
}

func (s *Store) ListMessages(ctx context.Context, conversationID int64, pagination domain.Pagination) ([]domain.Message, error) {
	if pagination.Page < 1 {
		pagination.Page = 1
	}
	if pagination.PageSize <= 0 {
		pagination.PageSize = 50
	}
	offset := (pagination.Page - 1) * pagination.PageSize

	const q = `
SELECT id, conversation_id, role, content_masked, created_at
FROM messages
WHERE conversation_id = $1
ORDER BY created_at ASC, id ASC
LIMIT $2 OFFSET $3
`
	rows, err := s.db.QueryContext(ctx, q, conversationID, pagination.PageSize, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.Message, 0)
	for rows.Next() {
		var m domain.Message
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Role, &m.ContentMasked, &m.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, m)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) ListAttempts(ctx context.Context, conversationID int64, pagination domain.Pagination) ([]domain.RequestAttempt, error) {
	if pagination.Page < 1 {
		pagination.Page = 1
	}
	if pagination.PageSize <= 0 {
		pagination.PageSize = 50
	}
	offset := (pagination.Page - 1) * pagination.PageSize

	const q = `
SELECT id, conversation_id, provider, model, tokens_in, tokens_out, cost_usd, latency_ms, success, error_code, created_at
FROM request_attempts
WHERE conversation_id = $1
ORDER BY created_at ASC, id ASC
LIMIT $2 OFFSET $3
`
	rows, err := s.db.QueryContext(ctx, q, conversationID, pagination.PageSize, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.RequestAttempt, 0)
	for rows.Next() {
		var a domain.RequestAttempt
		if err := rows.Scan(
			&a.ID,
			&a.ConversationID,
			&a.Provider,
			&a.Model,
			&a.TokensIn,
			&a.TokensOut,
			&a.CostUSD,
			&a.LatencyMS,
			&a.Success,
			&a.ErrorCode,
			&a.CreatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, a)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) GetLatestStatus(ctx context.Context) (domain.InfraSnapshot, error) {
	const q = `
SELECT id, vpn_peer_count, openclaw_up, cpu_pct, mem_pct, captured_at
FROM infra_snapshots
ORDER BY captured_at DESC, id DESC
LIMIT 1
`
	var out domain.InfraSnapshot
	if err := s.db.QueryRowContext(ctx, q).Scan(
		&out.ID,
		&out.VPNPeerCount,
		&out.OpenClawUp,
		&out.CPUPct,
		&out.MemPct,
		&out.CapturedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.InfraSnapshot{}, repository.ErrNotFound
		}
		return domain.InfraSnapshot{}, err
	}
	return out, nil
}

func (s *Store) ListSnapshots(ctx context.Context, from, to time.Time, pagination domain.Pagination) ([]domain.InfraSnapshot, error) {
	if pagination.Page < 1 {
		pagination.Page = 1
	}
	if pagination.PageSize <= 0 {
		pagination.PageSize = 50
	}
	offset := (pagination.Page - 1) * pagination.PageSize

	const q = `
SELECT id, vpn_peer_count, openclaw_up, cpu_pct, mem_pct, captured_at
FROM infra_snapshots
WHERE captured_at BETWEEN $1 AND $2
ORDER BY captured_at DESC, id DESC
LIMIT $3 OFFSET $4
`

	rows, err := s.db.QueryContext(ctx, q, from, to, pagination.PageSize, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.InfraSnapshot, 0)
	for rows.Next() {
		var snap domain.InfraSnapshot
		if err := rows.Scan(
			&snap.ID,
			&snap.VPNPeerCount,
			&snap.OpenClawUp,
			&snap.CPUPct,
			&snap.MemPct,
			&snap.CapturedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, snap)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) InsertReadAudit(ctx context.Context, event domain.AuditEvent) error {
	const q = `
INSERT INTO audit_events (actor, action, resource_type, resource_id, metadata_json, created_at)
VALUES ($1, $2, $3, NULLIF($4, ''), $5::jsonb, $6)
`

	metadata := event.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}

	payload, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx, q,
		event.Actor,
		event.Action,
		event.ResourceType,
		event.ResourceID,
		string(payload),
		event.CreatedAt,
	)
	return err
}

func metricExpression(metric string) (string, bool) {
	switch metric {
	case "requests":
		return "COUNT(*)::float8", true
	case "tokens":
		return "COALESCE(SUM(tokens_in + tokens_out), 0)::float8", true
	case "cost":
		return "COALESCE(SUM(cost_usd), 0)::float8", true
	case "errors":
		return "SUM(CASE WHEN success THEN 0 ELSE 1 END)::float8", true
	default:
		return "", false
	}
}

func bucketExpression(bucket string) (string, bool) {
	const utcTimestamp = "(created_at AT TIME ZONE 'UTC')"

	switch bucket {
	case "1m":
		return "date_trunc('minute', " + utcTimestamp + ") AT TIME ZONE 'UTC'", true
	case "5m":
		return "((date_trunc('hour', " + utcTimestamp + ") + (floor(date_part('minute', " + utcTimestamp + ") / 5) * interval '5 minutes')) AT TIME ZONE 'UTC')", true
	case "1h":
		return "date_trunc('hour', " + utcTimestamp + ") AT TIME ZONE 'UTC'", true
	case "day":
		return "date_trunc('day', " + utcTimestamp + ") AT TIME ZONE 'UTC'", true
	default:
		return "", false
	}
}
