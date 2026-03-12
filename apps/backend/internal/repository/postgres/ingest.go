package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"

	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/domain"
	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/repository"
)

func (s *Store) ClaimEvent(ctx context.Context, event domain.IngestEventRecord) (domain.IngestEventRecord, bool, error) {
	const insertQuery = `
INSERT INTO ingest_events (
  event_type, source, event_id, schema_version, status, payload_json, attempt_count, first_seen_at, last_attempt_at
)
VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7, $8, $9)
ON CONFLICT (event_type, source, event_id) DO NOTHING
RETURNING event_type, source, event_id, schema_version, status, last_error, attempt_count, first_seen_at, last_attempt_at, next_retry_at
	`

	var stored domain.IngestEventRecord
	var nextRetryAt sql.NullTime
	var lastError sql.NullString
	err := s.db.QueryRowContext(
		ctx,
		insertQuery,
		event.EventType,
		event.Source,
		event.EventID,
		event.SchemaVersion,
		event.Status,
		string(event.Payload),
		event.AttemptCount,
		event.FirstSeenAt,
		event.LastAttemptAt,
	).Scan(
		&stored.EventType,
		&stored.Source,
		&stored.EventID,
		&stored.SchemaVersion,
		&stored.Status,
		&lastError,
		&stored.AttemptCount,
		&stored.FirstSeenAt,
		&stored.LastAttemptAt,
		&nextRetryAt,
	)
	switch {
	case err == nil:
		stored.Payload = event.Payload
		if lastError.Valid {
			stored.LastError = lastError.String
		}
		if nextRetryAt.Valid {
			stored.NextRetryAt = nextRetryAt.Time.UTC()
		}
		return stored, true, nil
	case errors.Is(err, sql.ErrNoRows):
		existing, lookupErr := s.lookupIngestEvent(ctx, event.EventType, event.Source, event.EventID, false)
		return existing, false, lookupErr
	default:
		return domain.IngestEventRecord{}, false, wrapDBError(err)
	}
}

func (s *Store) PersistConversationEvent(ctx context.Context, event domain.ConversationEventInput) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return wrapDBError(err)
	}

	accountID, err := upsertAccountTx(ctx, tx, event.Source, event.Account)
	if err != nil {
		_ = tx.Rollback()
		return wrapDBError(err)
	}

	conversationID, err := upsertConversationTx(ctx, tx, event.Source, accountID, event.Conversation)
	if err != nil {
		_ = tx.Rollback()
		return wrapDBError(err)
	}

	if event.Message != nil {
		if err := upsertMessageTx(ctx, tx, event.Source, conversationID, *event.Message); err != nil {
			_ = tx.Rollback()
			return wrapDBError(err)
		}
	}

	if err := tx.Commit(); err != nil {
		return wrapDBError(err)
	}
	return nil
}

func (s *Store) PersistInfraSnapshot(ctx context.Context, snapshot domain.InfraSnapshotInput) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return wrapDBError(err)
	}

	const insertSnapshot = `
INSERT INTO infra_snapshots (source, vpn_peer_count, openclaw_up, cpu_pct, mem_pct, captured_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id
`

	var snapshotID int64
	if err := tx.QueryRowContext(
		ctx,
		insertSnapshot,
		snapshot.Source,
		snapshot.VPNPeerCount,
		snapshot.OpenClawUp,
		snapshot.CPUPct,
		snapshot.MemPct,
		snapshot.CapturedAt,
	).Scan(&snapshotID); err != nil {
		_ = tx.Rollback()
		return wrapDBError(err)
	}

	const upsertLatest = `
INSERT INTO infra_status_latest (source, snapshot_id, captured_at, updated_at)
VALUES ($1, $2, $3, NOW())
ON CONFLICT (source) DO UPDATE
SET snapshot_id = EXCLUDED.snapshot_id,
    captured_at = EXCLUDED.captured_at,
    updated_at = NOW()
WHERE infra_status_latest.captured_at IS NULL
   OR EXCLUDED.captured_at >= infra_status_latest.captured_at
	`
	if _, err := tx.ExecContext(ctx, upsertLatest, snapshot.Source, snapshotID, snapshot.CapturedAt); err != nil {
		_ = tx.Rollback()
		return wrapDBError(err)
	}

	if err := tx.Commit(); err != nil {
		return wrapDBError(err)
	}
	return nil
}

func (s *Store) PersistRequestAttempt(ctx context.Context, event domain.RequestAttemptEventInput) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return wrapDBError(err)
	}

	accountID, err := upsertAccountTx(ctx, tx, event.Source, event.Account)
	if err != nil {
		_ = tx.Rollback()
		return wrapDBError(err)
	}

	conversationID, err := upsertConversationTx(ctx, tx, event.Source, accountID, event.Conversation)
	if err != nil {
		_ = tx.Rollback()
		return wrapDBError(err)
	}

	const upsertAttempt = `
INSERT INTO request_attempts (
  conversation_id, source, external_id, provider, model, tokens_in, tokens_out, cost_usd, latency_ms, success, error_code, created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NULLIF($11, ''), $12)
ON CONFLICT (source, external_id) DO UPDATE
SET conversation_id = EXCLUDED.conversation_id,
    provider = EXCLUDED.provider,
    model = EXCLUDED.model,
    tokens_in = EXCLUDED.tokens_in,
    tokens_out = EXCLUDED.tokens_out,
    cost_usd = EXCLUDED.cost_usd,
    latency_ms = EXCLUDED.latency_ms,
    success = EXCLUDED.success,
    error_code = EXCLUDED.error_code,
    created_at = EXCLUDED.created_at
`
	errorCode := ""
	if event.Attempt.ErrorCode != nil {
		errorCode = *event.Attempt.ErrorCode
	}
	if _, err := tx.ExecContext(
		ctx,
		upsertAttempt,
		conversationID,
		event.Source,
		event.Attempt.ExternalID,
		event.Attempt.Provider,
		event.Attempt.Model,
		event.Attempt.TokensIn,
		event.Attempt.TokensOut,
		event.Attempt.CostUSD,
		event.Attempt.LatencyMS,
		event.Attempt.Success,
		errorCode,
		event.Attempt.CreatedAt,
	); err != nil {
		_ = tx.Rollback()
		return wrapDBError(err)
	}

	if err := tx.Commit(); err != nil {
		return wrapDBError(err)
	}
	return nil
}

func (s *Store) MarkEventCompleted(ctx context.Context, key domain.EventKey, processedAt time.Time) error {
	const q = `
UPDATE ingest_events
SET status = $1,
    processed_at = $2,
    next_retry_at = NULL,
    last_error = NULL,
    dead_lettered_at = NULL
WHERE event_type = $3 AND source = $4 AND event_id = $5
	`
	_, err := s.db.ExecContext(ctx, q, domain.IngestEventStatusCompleted, processedAt, key.EventType, key.Source, key.EventID)
	return wrapDBError(err)
}

func (s *Store) RecordEventFailure(ctx context.Context, key domain.EventKey, lastError string, nextRetryAt time.Time, maxAttempts int) (domain.IngestResult, error) {
	const updateFailure = `
UPDATE ingest_events
SET status = CASE
      WHEN $1::timestamptz IS NULL OR attempt_count >= $2 THEN $3
      ELSE $4
    END,
    last_error = $5,
    next_retry_at = CASE
      WHEN $1::timestamptz IS NULL OR attempt_count >= $2 THEN NULL
      ELSE $1
    END,
    dead_lettered_at = CASE
      WHEN $1::timestamptz IS NULL OR attempt_count >= $2 THEN NOW()
      ELSE NULL
    END
WHERE event_type = $6 AND source = $7 AND event_id = $8
RETURNING attempt_count, status
	`

	var (
		scheduledRetryAt any
		attemptCount     int
		status           domain.IngestEventStatus
	)
	if !nextRetryAt.IsZero() {
		scheduledRetryAt = nextRetryAt.UTC()
	}
	if err := s.db.QueryRowContext(
		ctx,
		updateFailure,
		scheduledRetryAt,
		maxAttempts,
		domain.IngestEventStatusDeadLetter,
		domain.IngestEventStatusRetryScheduled,
		lastError,
		key.EventType,
		key.Source,
		key.EventID,
	).Scan(&attemptCount, &status); err != nil {
		return domain.IngestResult{}, wrapDBError(err)
	}

	outcome := domain.IngestOutcomeRetryScheduled
	if status == domain.IngestEventStatusDeadLetter {
		outcome = domain.IngestOutcomeDeadLetter
	}

	return domain.IngestResult{
		EventKey:     key,
		Outcome:      outcome,
		Queued:       outcome == domain.IngestOutcomeRetryScheduled,
		AttemptCount: attemptCount,
	}, nil
}

func (s *Store) LeaseRetryBatch(ctx context.Context, now, staleBefore time.Time, limit int) ([]domain.IngestEventRecord, error) {
	if limit <= 0 {
		limit = 10
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, wrapDBError(err)
	}

	const leaseQuery = `
WITH due AS (
  SELECT event_type, source, event_id
  FROM ingest_events
  WHERE (
      status = $1
      AND next_retry_at IS NOT NULL
      AND next_retry_at <= $2
    )
    OR (
      status = $3
      AND last_attempt_at <= $4
    )
  ORDER BY COALESCE(next_retry_at, last_attempt_at) ASC
  LIMIT $5
  FOR UPDATE SKIP LOCKED
)
UPDATE ingest_events AS ie
SET status = $3,
    attempt_count = ie.attempt_count + 1,
    last_attempt_at = $2
FROM due
WHERE ie.event_type = due.event_type
  AND ie.source = due.source
  AND ie.event_id = due.event_id
RETURNING ie.event_type, ie.source, ie.event_id, ie.schema_version, ie.status, ie.payload_json, ie.last_error, ie.attempt_count, ie.first_seen_at, ie.last_attempt_at, ie.next_retry_at
	`

	rows, err := tx.QueryContext(ctx, leaseQuery,
		domain.IngestEventStatusRetryScheduled,
		now,
		domain.IngestEventStatusProcessing,
		staleBefore,
		limit,
	)
	if err != nil {
		_ = tx.Rollback()
		return nil, wrapDBError(err)
	}
	defer rows.Close()

	items := make([]domain.IngestEventRecord, 0)
	for rows.Next() {
		item, err := scanIngestEvent(rows, true)
		if err != nil {
			_ = tx.Rollback()
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		_ = tx.Rollback()
		return nil, wrapDBError(err)
	}
	if err := tx.Commit(); err != nil {
		return nil, wrapDBError(err)
	}
	return items, nil
}

func (s *Store) GetIngestStatus(ctx context.Context, now time.Time) (domain.IngestStatus, error) {
	const q = `
SELECT
  COUNT(*) FILTER (WHERE status = $1)::int AS retry_scheduled,
  COUNT(*) FILTER (WHERE status = $2)::int AS processing,
  COUNT(*) FILTER (WHERE status = $3)::int AS dead_letter,
  COUNT(*) FILTER (WHERE status IN ($1, $2))::int AS queue_depth,
  COALESCE(MAX(EXTRACT(EPOCH FROM ($4 - first_seen_at))) FILTER (WHERE status IN ($1, $2)), 0)::float8 AS oldest_pending_age_seconds,
  COALESCE(MAX(GREATEST(EXTRACT(EPOCH FROM ($4 - next_retry_at)), 0)) FILTER (WHERE status = $1 AND next_retry_at IS NOT NULL), 0)::float8 AS oldest_retry_age_seconds
FROM ingest_events
`

	var status domain.IngestStatus
	if err := s.db.QueryRowContext(ctx, q,
		domain.IngestEventStatusRetryScheduled,
		domain.IngestEventStatusProcessing,
		domain.IngestEventStatusDeadLetter,
		now,
	).Scan(
		&status.RetryScheduled,
		&status.Processing,
		&status.DeadLetter,
		&status.QueueDepth,
		&status.OldestPendingAgeSeconds,
		&status.OldestRetryAgeSeconds,
	); err != nil {
		return domain.IngestStatus{}, wrapDBError(err)
	}
	return status, nil
}

func upsertAccountTx(ctx context.Context, tx *sql.Tx, source string, account domain.AccountInput) (int64, error) {
	const q = `
INSERT INTO accounts (source, external_id, email, status)
VALUES ($1, $2, $3, $4)
ON CONFLICT (source, external_id) DO UPDATE
SET email = EXCLUDED.email,
    status = EXCLUDED.status
RETURNING id
`

	var id int64
	err := tx.QueryRowContext(ctx, q, source, account.ExternalID, account.Email, account.Status).Scan(&id)
	return id, err
}

func upsertConversationTx(ctx context.Context, tx *sql.Tx, source string, accountID int64, conversation domain.ConversationInput) (int64, error) {
	const q = `
INSERT INTO conversations (account_id, source, external_id, channel, status, started_at, ended_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (source, external_id) DO UPDATE
SET account_id = EXCLUDED.account_id,
    channel = EXCLUDED.channel,
    status = EXCLUDED.status,
    started_at = EXCLUDED.started_at,
    ended_at = EXCLUDED.ended_at
RETURNING id
`

	var id int64
	err := tx.QueryRowContext(
		ctx,
		q,
		accountID,
		source,
		conversation.ExternalID,
		conversation.Channel,
		conversation.Status,
		conversation.StartedAt,
		conversation.EndedAt,
	).Scan(&id)
	return id, err
}

func upsertMessageTx(ctx context.Context, tx *sql.Tx, source string, conversationID int64, message domain.MessageInput) error {
	const q = `
INSERT INTO messages (conversation_id, source, external_id, role, content_masked, created_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (source, external_id) DO UPDATE
SET conversation_id = EXCLUDED.conversation_id,
    role = EXCLUDED.role,
    content_masked = EXCLUDED.content_masked,
    created_at = EXCLUDED.created_at
`

	_, err := tx.ExecContext(
		ctx,
		q,
		conversationID,
		source,
		message.ExternalID,
		message.Role,
		message.ContentMasked,
		message.CreatedAt,
	)
	return err
}

func (s *Store) lookupIngestEvent(ctx context.Context, eventType, source, eventID string, includePayload bool) (domain.IngestEventRecord, error) {
	var query string
	if includePayload {
		query = `
SELECT event_type, source, event_id, schema_version, status, payload_json, last_error, attempt_count, first_seen_at, last_attempt_at, next_retry_at
FROM ingest_events
WHERE event_type = $1 AND source = $2 AND event_id = $3
	`
	} else {
		query = `
SELECT event_type, source, event_id, schema_version, status, last_error, attempt_count, first_seen_at, last_attempt_at, next_retry_at
FROM ingest_events
WHERE event_type = $1 AND source = $2 AND event_id = $3
	`
	}

	row := s.db.QueryRowContext(ctx, query, eventType, source, eventID)
	if includePayload {
		event, err := scanIngestEvent(row, true)
		return event, wrapDBError(err)
	}
	event, err := scanIngestEvent(row, false)
	return event, wrapDBError(err)
}

func scanIngestEvent(scanner interface{ Scan(dest ...any) error }, includePayload bool) (domain.IngestEventRecord, error) {
	var (
		event       domain.IngestEventRecord
		payload     []byte
		lastError   sql.NullString
		nextRetryAt sql.NullTime
	)

	if includePayload {
		if err := scanner.Scan(
			&event.EventType,
			&event.Source,
			&event.EventID,
			&event.SchemaVersion,
			&event.Status,
			&payload,
			&lastError,
			&event.AttemptCount,
			&event.FirstSeenAt,
			&event.LastAttemptAt,
			&nextRetryAt,
		); err != nil {
			return domain.IngestEventRecord{}, err
		}
		event.Payload = append([]byte(nil), payload...)
	} else {
		if err := scanner.Scan(
			&event.EventType,
			&event.Source,
			&event.EventID,
			&event.SchemaVersion,
			&event.Status,
			&lastError,
			&event.AttemptCount,
			&event.FirstSeenAt,
			&event.LastAttemptAt,
			&nextRetryAt,
		); err != nil {
			return domain.IngestEventRecord{}, err
		}
	}
	if lastError.Valid {
		event.LastError = lastError.String
	}
	if nextRetryAt.Valid {
		event.NextRetryAt = nextRetryAt.Time.UTC()
	}
	return event, nil
}

func wrapDBError(err error) error {
	if err == nil {
		return nil
	}
	if isTransientDBError(err) {
		return fmt.Errorf("%w: %v", repository.ErrTransient, err)
	}
	return err
}

func isTransientDBError(err error) bool {
	if errors.Is(err, sql.ErrConnDone) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	var pqErr *pq.Error
	if !errors.As(err, &pqErr) {
		return false
	}

	switch pqErr.Code.Class().Name() {
	case "Connection Exception", "Transaction Rollback", "Insufficient Resources", "Operator Intervention", "System Error":
		return true
	}

	switch string(pqErr.Code) {
	case "40001", "40P01", "55P03", "57P01", "53300":
		return true
	default:
		return false
	}
}
