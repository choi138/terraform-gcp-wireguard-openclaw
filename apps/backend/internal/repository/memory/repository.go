package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/domain"
	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/repository"
)

type Store struct {
	mu                           sync.RWMutex
	accounts                     []accountRecord
	conversations                []domain.Conversation
	messages                     []domain.Message
	attempts                     []domain.RequestAttempt
	snapshots                    []domain.InfraSnapshot
	audits                       []domain.AuditEvent
	securityFindings             []domain.SecurityFinding
	messageRawPayload            map[int64][]byte
	nextAccountID                int64
	nextConversationID           int64
	nextMessageID                int64
	nextAttemptID                int64
	nextSnapshotID               int64
	nextSecurityFindingID        int64
	conversationByExternal       map[string]int64
	messageByExternal            map[string]int64
	attemptByExternal            map[string]int64
	snapshotByEvent              map[string]int64
	ingestEvents                 map[string]domain.IngestEventRecord
	securityFindingByFingerprint map[string]int64
}

func NewStore() *Store {
	now := time.Now().UTC()
	start := now.Add(-2 * time.Hour)

	conv := domain.Conversation{
		ID:        1,
		AccountID: 1001,
		Channel:   "telegram",
		Status:    "completed",
		StartedAt: start,
	}

	msg := domain.Message{
		ID:             1,
		ConversationID: 1,
		Role:           "user",
		ContentMasked:  "hello",
		CreatedAt:      start.Add(10 * time.Second),
	}

	attempt := domain.RequestAttempt{
		ID:             1,
		ConversationID: 1,
		Provider:       "anthropic",
		Model:          "claude-opus-4-6",
		TokensIn:       120,
		TokensOut:      240,
		CostUSD:        0.02,
		LatencyMS:      420,
		Success:        true,
		CreatedAt:      start.Add(20 * time.Second),
	}

	snapshot := domain.InfraSnapshot{
		ID:           1,
		VPNPeerCount: 3,
		OpenClawUp:   true,
		CPUPct:       22.4,
		MemPct:       48.2,
		CapturedAt:   now.Add(-1 * time.Minute),
	}

	return &Store{
		accounts:                     []accountRecord{{ID: 1001, ExternalID: "seed-account", Email: "seed@example.com", Status: "active"}},
		conversations:                []domain.Conversation{conv},
		messages:                     []domain.Message{msg},
		attempts:                     []domain.RequestAttempt{attempt},
		snapshots:                    []domain.InfraSnapshot{snapshot},
		audits:                       make([]domain.AuditEvent, 0),
		securityFindings:             make([]domain.SecurityFinding, 0),
		messageRawPayload:            map[int64][]byte{1: []byte("seed-raw-payload")},
		nextAccountID:                1002,
		nextConversationID:           2,
		nextMessageID:                2,
		nextAttemptID:                2,
		nextSnapshotID:               2,
		nextSecurityFindingID:        1,
		conversationByExternal:       make(map[string]int64),
		messageByExternal:            make(map[string]int64),
		attemptByExternal:            make(map[string]int64),
		snapshotByEvent:              make(map[string]int64),
		ingestEvents:                 make(map[string]domain.IngestEventRecord),
		securityFindingByFingerprint: make(map[string]int64),
	}
}

func (s *Store) Ping(context.Context) error {
	return nil
}

func (s *Store) GetSummary(_ context.Context, from, to time.Time) (domain.DashboardSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var (
		requests int64
		tokens   int64
		cost     float64
		errors   int64
	)

	active := map[int64]struct{}{}
	for _, c := range s.conversations {
		if !c.StartedAt.Before(from) && !c.StartedAt.After(to) {
			active[c.AccountID] = struct{}{}
		}
	}

	for _, a := range s.attempts {
		if a.CreatedAt.Before(from) || a.CreatedAt.After(to) {
			continue
		}
		requests++
		tokens += a.TokensIn + a.TokensOut
		cost += a.CostUSD
		if !a.Success {
			errors++
		}
	}

	errorRate := 0.0
	if requests > 0 {
		errorRate = float64(errors) / float64(requests)
	}

	return domain.DashboardSummary{
		RequestsTotal:  requests,
		TokensTotal:    tokens,
		CostUSD:        cost,
		ErrorRate:      errorRate,
		ActiveAccounts: int64(len(active)),
	}, nil
}

func (s *Store) GetTimeseries(_ context.Context, metric, bucket string, from, to time.Time) ([]domain.DashboardPoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	d, ok := bucketDuration(bucket)
	if !ok {
		return nil, nil
	}

	buckets := map[time.Time]float64{}
	for _, a := range s.attempts {
		if a.CreatedAt.Before(from) || a.CreatedAt.After(to) {
			continue
		}
		start := floorTime(a.CreatedAt.UTC(), d)
		switch metric {
		case "requests":
			buckets[start] += 1
		case "tokens":
			buckets[start] += float64(a.TokensIn + a.TokensOut)
		case "cost":
			buckets[start] += a.CostUSD
		case "errors":
			if !a.Success {
				buckets[start] += 1
			}
		}
	}

	points := make([]domain.DashboardPoint, 0, len(buckets))
	for ts, value := range buckets {
		points = append(points, domain.DashboardPoint{BucketStart: ts, Value: value})
	}

	sort.Slice(points, func(i, j int) bool {
		return points[i].BucketStart.Before(points[j].BucketStart)
	})

	return points, nil
}

func (s *Store) ListConversations(_ context.Context, filter domain.ConversationFilter, pagination domain.Pagination) ([]domain.Conversation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filtered := make([]domain.Conversation, 0)
	for _, c := range s.conversations {
		if filter.Channel != "" && c.Channel != filter.Channel {
			continue
		}
		if filter.Status != "" && c.Status != filter.Status {
			continue
		}
		filtered = append(filtered, c)
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].StartedAt.After(filtered[j].StartedAt)
	})

	return paginate(filtered, pagination), nil
}

func (s *Store) GetConversation(_ context.Context, conversationID int64) (domain.Conversation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, c := range s.conversations {
		if c.ID == conversationID {
			return c, nil
		}
	}
	return domain.Conversation{}, repository.ErrNotFound
}

func (s *Store) ListMessages(_ context.Context, conversationID int64, pagination domain.Pagination) ([]domain.Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filtered := make([]domain.Message, 0)
	for _, m := range s.messages {
		if m.ConversationID == conversationID {
			filtered = append(filtered, m)
		}
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].CreatedAt.Before(filtered[j].CreatedAt)
	})

	return paginate(filtered, pagination), nil
}

func (s *Store) ListAttempts(_ context.Context, conversationID int64, pagination domain.Pagination) ([]domain.RequestAttempt, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filtered := make([]domain.RequestAttempt, 0)
	for _, a := range s.attempts {
		if a.ConversationID == conversationID {
			filtered = append(filtered, a)
		}
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].CreatedAt.Before(filtered[j].CreatedAt)
	})

	return paginate(filtered, pagination), nil
}

func (s *Store) GetLatestStatus(_ context.Context) (domain.InfraSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.snapshots) == 0 {
		return domain.InfraSnapshot{}, repository.ErrNotFound
	}

	latest := s.snapshots[0]
	for _, snap := range s.snapshots[1:] {
		if snap.CapturedAt.After(latest.CapturedAt) {
			latest = snap
		}
	}

	return latest, nil
}

func (s *Store) ListSnapshots(_ context.Context, from, to time.Time, pagination domain.Pagination) ([]domain.InfraSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filtered := make([]domain.InfraSnapshot, 0)
	for _, snap := range s.snapshots {
		if snap.CapturedAt.Before(from) || snap.CapturedAt.After(to) {
			continue
		}
		filtered = append(filtered, snap)
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].CapturedAt.After(filtered[j].CapturedAt)
	})

	return paginate(filtered, pagination), nil
}

func (s *Store) InsertReadAudit(ctx context.Context, event domain.AuditEvent) error {
	return s.InsertAuditEvent(ctx, event)
}

func (s *Store) InsertAuditEvent(_ context.Context, event domain.AuditEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.audits = append(s.audits, cloneAuditEvent(event))
	return nil
}

func (s *Store) AuditEvents() []domain.AuditEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.AuditEvent, len(s.audits))
	for i, event := range s.audits {
		out[i] = cloneAuditEvent(event)
	}
	return out
}

func bucketDuration(bucket string) (time.Duration, bool) {
	switch bucket {
	case "1m":
		return time.Minute, true
	case "5m":
		return 5 * time.Minute, true
	case "1h":
		return time.Hour, true
	case "day":
		return 24 * time.Hour, true
	default:
		return 0, false
	}
}

func floorTime(t time.Time, d time.Duration) time.Time {
	seconds := int64(d / time.Second)
	if seconds <= 0 {
		return t
	}
	unix := t.Unix()
	return time.Unix((unix/seconds)*seconds, 0).UTC()
}

func paginate[T any](items []T, p domain.Pagination) []T {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PageSize <= 0 {
		p.PageSize = 50
	}
	start := (p.Page - 1) * p.PageSize
	if start >= len(items) {
		return []T{}
	}
	end := start + p.PageSize
	if end > len(items) {
		end = len(items)
	}
	return items[start:end]
}

func cloneAuditEvent(event domain.AuditEvent) domain.AuditEvent {
	event.Metadata = cloneMap(event.Metadata)
	return event
}

func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = cloneValue(value)
	}
	return out
}

func cloneSlice(in []any) []any {
	if in == nil {
		return nil
	}
	out := make([]any, len(in))
	for i, value := range in {
		out[i] = cloneValue(value)
	}
	return out
}

func cloneValue(v any) any {
	switch value := v.(type) {
	case map[string]any:
		return cloneMap(value)
	case []any:
		return cloneSlice(value)
	default:
		return value
	}
}
