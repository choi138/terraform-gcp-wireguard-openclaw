package postgres

import (
	"context"
	"fmt"
	"time"
)

func (s *Store) CompactRawMessagePayloads(ctx context.Context, cutoff time.Time, limit int, dryRun bool) (int, error) {
	query := `
WITH candidates AS (
  SELECT id
  FROM messages
  WHERE content_raw_encrypted IS NOT NULL
    AND created_at < $1
  ORDER BY created_at ASC, id ASC
  LIMIT $2
)
SELECT COUNT(*) FROM candidates
`
	if dryRun {
		return countRows(ctx, s, query, cutoff, limit)
	}

	query = `
WITH candidates AS (
  SELECT id
  FROM messages
  WHERE content_raw_encrypted IS NOT NULL
    AND created_at < $1
  ORDER BY created_at ASC, id ASC
  LIMIT $2
),
updated AS (
  UPDATE messages AS m
  SET content_raw_encrypted = NULL
  FROM candidates
  WHERE m.id = candidates.id
  RETURNING m.id
)
SELECT COUNT(*) FROM updated
`
	return countRows(ctx, s, query, cutoff, limit)
}

func (s *Store) DeleteExpiredAuditEvents(ctx context.Context, cutoff time.Time, limit int, dryRun bool) (int, error) {
	query := `
WITH candidates AS (
  SELECT id
  FROM audit_events
  WHERE created_at < $1
  ORDER BY created_at ASC, id ASC
  LIMIT $2
)
SELECT COUNT(*) FROM candidates
`
	if dryRun {
		return countRows(ctx, s, query, cutoff, limit)
	}

	query = `
WITH candidates AS (
  SELECT id
  FROM audit_events
  WHERE created_at < $1
  ORDER BY created_at ASC, id ASC
  LIMIT $2
),
deleted AS (
  DELETE FROM audit_events AS a
  USING candidates
  WHERE a.id = candidates.id
  RETURNING a.id
)
SELECT COUNT(*) FROM deleted
`
	return countRows(ctx, s, query, cutoff, limit)
}

func (s *Store) DeleteExpiredInfraSnapshots(ctx context.Context, cutoff time.Time, limit int, dryRun bool) (int, error) {
	query := `
WITH protected AS (
  SELECT id
  FROM infra_snapshots
  ORDER BY captured_at DESC, id DESC
  LIMIT 1
),
candidates AS (
  SELECT id
  FROM infra_snapshots
  WHERE captured_at < $1
    AND id NOT IN (SELECT id FROM protected)
  ORDER BY captured_at ASC, id ASC
  LIMIT $2
)
SELECT COUNT(*) FROM candidates
`
	if dryRun {
		return countRows(ctx, s, query, cutoff, limit)
	}

	query = `
WITH protected AS (
  SELECT id
  FROM infra_snapshots
  ORDER BY captured_at DESC, id DESC
  LIMIT 1
),
candidates AS (
  SELECT id
  FROM infra_snapshots
  WHERE captured_at < $1
    AND id NOT IN (SELECT id FROM protected)
  ORDER BY captured_at ASC, id ASC
  LIMIT $2
),
deleted AS (
  DELETE FROM infra_snapshots AS snap
  USING candidates
  WHERE snap.id = candidates.id
  RETURNING snap.id
)
SELECT COUNT(*) FROM deleted
`
	return countRows(ctx, s, query, cutoff, limit)
}

func (s *Store) DeleteExpiredIngestEvents(ctx context.Context, cutoff time.Time, limit int, dryRun bool) (int, error) {
	query := `
WITH candidates AS (
  SELECT event_type, source, event_id
  FROM ingest_events
  WHERE status IN ('completed', 'dead_letter')
    AND COALESCE(processed_at, dead_lettered_at, first_seen_at) < $1
  ORDER BY COALESCE(processed_at, dead_lettered_at, first_seen_at) ASC, event_type ASC, source ASC, event_id ASC
  LIMIT $2
)
SELECT COUNT(*) FROM candidates
`
	if dryRun {
		return countRows(ctx, s, query, cutoff, limit)
	}

	query = `
WITH candidates AS (
  SELECT event_type, source, event_id
  FROM ingest_events
  WHERE status IN ('completed', 'dead_letter')
    AND COALESCE(processed_at, dead_lettered_at, first_seen_at) < $1
  ORDER BY COALESCE(processed_at, dead_lettered_at, first_seen_at) ASC, event_type ASC, source ASC, event_id ASC
  LIMIT $2
),
deleted AS (
  DELETE FROM ingest_events AS ie
  USING candidates
  WHERE ie.event_type = candidates.event_type
    AND ie.source = candidates.source
    AND ie.event_id = candidates.event_id
  RETURNING ie.event_type
)
SELECT COUNT(*) FROM deleted
`
	return countRows(ctx, s, query, cutoff, limit)
}

func countRows(ctx context.Context, store *Store, query string, args ...any) (int, error) {
	var count int
	if err := store.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, wrapDBError(fmt.Errorf("retention query failed: %w", err))
	}
	return count, nil
}
