package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/domain"
	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/repository"
	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/repository/memory"
)

func TestIngestConversationEventIsIdempotent(t *testing.T) {
	store := memory.NewStore()
	service := NewService(store, Config{})

	event := testConversationEvent("evt-1")

	first, err := service.IngestConversationEvent(t.Context(), event)
	if err != nil {
		t.Fatalf("expected first ingest to succeed, got %v", err)
	}
	if first.Outcome != domain.IngestOutcomeAccepted {
		t.Fatalf("expected accepted outcome, got %s", first.Outcome)
	}

	second, err := service.IngestConversationEvent(t.Context(), event)
	if err != nil {
		t.Fatalf("expected duplicate ingest to succeed, got %v", err)
	}
	if second.Outcome != domain.IngestOutcomeDuplicate {
		t.Fatalf("expected duplicate outcome, got %s", second.Outcome)
	}

	conversations, err := store.ListConversations(context.Background(), domain.ConversationFilter{Channel: "telegram"}, domain.Pagination{Page: 1, PageSize: 100})
	if err != nil {
		t.Fatalf("expected list conversations to succeed, got %v", err)
	}
	found := 0
	for _, item := range conversations {
		if item.Status == "completed" && item.Channel == "telegram" {
			found++
		}
	}
	if found < 1 {
		t.Fatalf("expected at least one ingested conversation")
	}

	var ingestedConversationID int64
	for _, conversation := range conversations {
		if conversation.StartedAt.Equal(event.Conversation.StartedAt) {
			ingestedConversationID = conversation.ID
			break
		}
	}
	if ingestedConversationID == 0 {
		t.Fatal("expected to find ingested conversation id")
	}

	items, err := store.ListMessages(context.Background(), ingestedConversationID, domain.Pagination{Page: 1, PageSize: 100})
	if err != nil {
		t.Fatalf("expected list messages to succeed, got %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one message for ingested conversation, got %d", len(items))
	}
}

func TestDifferentIngestEventTypesCanReuseEventID(t *testing.T) {
	store := memory.NewStore()
	service := NewService(store, Config{})

	conversationResult, err := service.IngestConversationEvent(t.Context(), testConversationEvent("evt-shared"))
	if err != nil {
		t.Fatalf("expected conversation ingest to succeed, got %v", err)
	}
	if conversationResult.Outcome != domain.IngestOutcomeAccepted {
		t.Fatalf("expected accepted conversation ingest, got %s", conversationResult.Outcome)
	}

	attemptResult, err := service.IngestRequestAttempt(t.Context(), testRequestAttemptEvent("evt-shared"))
	if err != nil {
		t.Fatalf("expected request attempt ingest to succeed, got %v", err)
	}
	if attemptResult.Outcome != domain.IngestOutcomeAccepted {
		t.Fatalf("expected accepted request attempt ingest, got %s", attemptResult.Outcome)
	}
}

func TestProcessDueRetriesContinuesAfterUnexpectedError(t *testing.T) {
	repo := &recordFailureErrorRepo{
		Store:          memory.NewStore(),
		failingEventID: "evt-bad",
	}
	service := NewService(repo, Config{
		RetryBaseDelay:         100 * time.Millisecond,
		RetryMaxDelay:          500 * time.Millisecond,
		RetryMaxAttempts:       3,
		ProcessingLeaseTimeout: time.Second,
	})

	now := time.Date(2026, 3, 11, 9, 45, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	seedRetryEvent(t, repo.Store, marshalJSON(t, testConversationEvent("evt-good")), domain.EventKey{
		EventType: EventTypeConversation,
		Source:    "openclaw",
		EventID:   "evt-good",
	}, now.Add(-time.Second))
	seedRetryEvent(t, repo.Store, []byte(`{"broken":`), domain.EventKey{
		EventType: EventTypeConversation,
		Source:    "openclaw",
		EventID:   "evt-bad",
	}, now.Add(-time.Second))

	result, err := service.ProcessDueRetries(t.Context(), 10)
	if err == nil {
		t.Fatal("expected batch processing to return the unexpected error")
	}
	if result.Processed != 2 {
		t.Fatalf("expected two leased events, got %+v", result)
	}
	if result.Completed != 1 {
		t.Fatalf("expected second leased event to complete despite the first failure, got %+v", result)
	}
}

func TestProcessDueRetriesReclaimsStaleProcessingEvents(t *testing.T) {
	store := memory.NewStore()
	service := NewService(store, Config{
		ProcessingLeaseTimeout: time.Second,
	})

	now := time.Date(2026, 3, 11, 10, 15, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	event := testConversationEvent("evt-stale-processing")
	payload := marshalJSON(t, event)
	record := domain.IngestEventRecord{
		EventKey: domain.EventKey{
			EventType: EventTypeConversation,
			Source:    event.Source,
			EventID:   event.EventID,
		},
		SchemaVersion: event.SchemaVersion,
		Status:        domain.IngestEventStatusProcessing,
		Payload:       payload,
		AttemptCount:  1,
		FirstSeenAt:   now.Add(-5 * time.Minute),
		LastAttemptAt: now.Add(-2 * time.Second),
	}
	if _, inserted, err := store.ClaimEvent(t.Context(), record); err != nil {
		t.Fatalf("expected stale processing event to be claimed, got %v", err)
	} else if !inserted {
		t.Fatal("expected stale processing event to be inserted")
	}

	result, err := service.ProcessDueRetries(t.Context(), 10)
	if err != nil {
		t.Fatalf("expected stale processing event to be reclaimed, got %v", err)
	}
	if result.Completed != 1 {
		t.Fatalf("expected one reclaimed event to complete, got %+v", result)
	}

	status, err := service.GetStatus(t.Context())
	if err != nil {
		t.Fatalf("expected ingest status to load, got %v", err)
	}
	if status.QueueDepth != 0 || status.Processing != 0 || status.RetryScheduled != 0 {
		t.Fatalf("expected no stuck events after reclaim, got %+v", status)
	}
}

func TestTransientFailureSchedulesRetryAndEventuallyCompletes(t *testing.T) {
	repo := &flakyConversationRepo{
		Store:        memory.NewStore(),
		failuresLeft: 1,
	}
	service := NewService(repo, Config{
		RetryBaseDelay:   500 * time.Millisecond,
		RetryMaxDelay:    2 * time.Second,
		RetryMaxAttempts: 3,
	})

	now := time.Date(2026, 3, 11, 9, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	result, err := service.IngestConversationEvent(t.Context(), testConversationEvent("evt-retry"))
	if err != nil {
		t.Fatalf("expected transient failure to schedule retry, got %v", err)
	}
	if result.Outcome != domain.IngestOutcomeRetryScheduled {
		t.Fatalf("expected retry scheduled outcome, got %s", result.Outcome)
	}

	status, err := service.GetStatus(t.Context())
	if err != nil {
		t.Fatalf("expected ingest status to load, got %v", err)
	}
	if status.QueueDepth != 1 || status.RetryScheduled != 1 {
		t.Fatalf("unexpected queued status: %+v", status)
	}

	now = now.Add(time.Second)
	retryResult, err := service.ProcessDueRetries(t.Context(), 10)
	if err != nil {
		t.Fatalf("expected retry processing to succeed, got %v", err)
	}
	if retryResult.Completed != 1 {
		t.Fatalf("expected one completed retry, got %+v", retryResult)
	}

	status, err = service.GetStatus(t.Context())
	if err != nil {
		t.Fatalf("expected ingest status after retry to load, got %v", err)
	}
	if status.QueueDepth != 0 || status.DeadLetter != 0 {
		t.Fatalf("unexpected ingest status after retry: %+v", status)
	}
}

func TestRetryBudgetExhaustionDeadLettersEvent(t *testing.T) {
	repo := &flakyConversationRepo{
		Store:        memory.NewStore(),
		failuresLeft: 10,
	}
	service := NewService(repo, Config{
		RetryBaseDelay:   100 * time.Millisecond,
		RetryMaxDelay:    200 * time.Millisecond,
		RetryMaxAttempts: 2,
	})

	now := time.Date(2026, 3, 11, 10, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	result, err := service.IngestConversationEvent(t.Context(), testConversationEvent("evt-dead-letter"))
	if err != nil {
		t.Fatalf("expected initial transient failure to be queued, got %v", err)
	}
	if result.Outcome != domain.IngestOutcomeRetryScheduled {
		t.Fatalf("expected retry scheduled outcome, got %s", result.Outcome)
	}

	now = now.Add(time.Second)
	batch, err := service.ProcessDueRetries(t.Context(), 10)
	if err != nil {
		t.Fatalf("expected retry processing to complete, got %v", err)
	}
	if batch.DeadLettered != 1 {
		t.Fatalf("expected one dead-lettered event, got %+v", batch)
	}

	status, err := service.GetStatus(t.Context())
	if err != nil {
		t.Fatalf("expected ingest status to load, got %v", err)
	}
	if status.DeadLetter != 1 {
		t.Fatalf("expected one dead-letter event, got %+v", status)
	}
}

func TestCompletionFailureSchedulesRetryWithoutReplayingPayload(t *testing.T) {
	repo := &flakyCompletionRepo{
		Store:                memory.NewStore(),
		completeFailuresLeft: 1,
	}
	service := NewService(repo, Config{
		RetryBaseDelay:   500 * time.Millisecond,
		RetryMaxDelay:    2 * time.Second,
		RetryMaxAttempts: 3,
	})

	now := time.Date(2026, 3, 11, 9, 30, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	result, err := service.IngestConversationEvent(t.Context(), testConversationEvent("evt-complete-retry"))
	if err != nil {
		t.Fatalf("expected completion failure to schedule retry, got %v", err)
	}
	if result.Outcome != domain.IngestOutcomeRetryScheduled {
		t.Fatalf("expected retry scheduled outcome, got %s", result.Outcome)
	}
	if repo.persistCalls != 1 {
		t.Fatalf("expected one persist attempt before completion failure, got %d", repo.persistCalls)
	}

	now = now.Add(time.Second)
	batch, err := service.ProcessDueRetries(t.Context(), 10)
	if err != nil {
		t.Fatalf("expected completion retry to succeed, got %v", err)
	}
	if batch.Completed != 1 {
		t.Fatalf("expected one completed retry, got %+v", batch)
	}
	if repo.persistCalls != 1 {
		t.Fatalf("expected completion retry to skip payload replay, got %d persist calls", repo.persistCalls)
	}

	status, err := service.GetStatus(t.Context())
	if err != nil {
		t.Fatalf("expected ingest status to load, got %v", err)
	}
	if status.QueueDepth != 0 || status.RetryScheduled != 0 || status.Processing != 0 {
		t.Fatalf("unexpected ingest status after completion retry: %+v", status)
	}

	duplicate, err := service.IngestConversationEvent(t.Context(), testConversationEvent("evt-complete-retry"))
	if err != nil {
		t.Fatalf("expected duplicate ingest after completion retry, got %v", err)
	}
	if duplicate.Outcome != domain.IngestOutcomeDuplicate {
		t.Fatalf("expected duplicate outcome after completion retry, got %s", duplicate.Outcome)
	}
}

func TestIngestThroughputTarget100EventsPerSecond(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping throughput timing check in short mode")
	}

	store := memory.NewStore()
	service := NewService(store, Config{})
	started := time.Now()

	for i := 0; i < 100; i++ {
		event := testConversationEvent(fmt.Sprintf("evt-throughput-%d", i))
		event.Conversation.ExternalID = fmt.Sprintf("conv-throughput-%d", i)
		event.Message.ExternalID = fmt.Sprintf("msg-throughput-%d", i)
		result, err := service.IngestConversationEvent(t.Context(), event)
		if err != nil {
			t.Fatalf("expected ingest to succeed, got %v", err)
		}
		if result.Outcome != domain.IngestOutcomeAccepted {
			t.Fatalf("expected accepted outcome, got %s", result.Outcome)
		}
	}

	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("expected 100 ingest events to complete within 1s, got %s", elapsed)
	}
}

type flakyConversationRepo struct {
	*memory.Store
	failuresLeft int
}

func (r *flakyConversationRepo) PersistConversationEvent(ctx context.Context, event domain.ConversationEventInput) error {
	if r.failuresLeft > 0 {
		r.failuresLeft--
		return fmt.Errorf("%w: simulated transient failure", repository.ErrTransient)
	}
	return r.Store.PersistConversationEvent(ctx, event)
}

type flakyCompletionRepo struct {
	*memory.Store
	completeFailuresLeft int
	persistCalls         int
}

type recordFailureErrorRepo struct {
	*memory.Store
	failingEventID string
}

func (r *flakyCompletionRepo) PersistConversationEvent(ctx context.Context, event domain.ConversationEventInput) error {
	r.persistCalls++
	return r.Store.PersistConversationEvent(ctx, event)
}

func (r *flakyCompletionRepo) MarkEventCompleted(ctx context.Context, key domain.EventKey, processedAt time.Time) error {
	if r.completeFailuresLeft > 0 {
		r.completeFailuresLeft--
		return fmt.Errorf("%w: simulated completion state write failure", repository.ErrTransient)
	}
	return r.Store.MarkEventCompleted(ctx, key, processedAt)
}

func (r *recordFailureErrorRepo) RecordEventFailure(ctx context.Context, key domain.EventKey, lastError string, nextRetryAt time.Time, maxAttempts int) (domain.IngestResult, error) {
	if key.EventID == r.failingEventID {
		return domain.IngestResult{}, fmt.Errorf("failed to record retry state")
	}
	return r.Store.RecordEventFailure(ctx, key, lastError, nextRetryAt, maxAttempts)
}

func testConversationEvent(eventID string) domain.ConversationEventInput {
	startedAt := time.Date(2026, 3, 11, 8, 0, 0, 0, time.UTC)
	messageAt := startedAt.Add(5 * time.Second)
	return domain.ConversationEventInput{
		SchemaVersion: domain.SupportedIngestSchemaVersion,
		Source:        "openclaw",
		EventID:       eventID,
		OccurredAt:    messageAt,
		Account: domain.AccountInput{
			ExternalID: "acct-100",
			Email:      "ops@example.com",
			Status:     "active",
		},
		Conversation: domain.ConversationInput{
			ExternalID: "conv-100",
			Channel:    "telegram",
			Status:     "completed",
			StartedAt:  startedAt,
		},
		Message: &domain.MessageInput{
			ExternalID:    "msg-100",
			Role:          "user",
			ContentMasked: "deploy wireguard",
			CreatedAt:     messageAt,
		},
	}
}

func testRequestAttemptEvent(eventID string) domain.RequestAttemptEventInput {
	startedAt := time.Date(2026, 3, 11, 8, 0, 0, 0, time.UTC)
	attemptAt := startedAt.Add(8 * time.Second)
	return domain.RequestAttemptEventInput{
		SchemaVersion: domain.SupportedIngestSchemaVersion,
		Source:        "openclaw",
		EventID:       eventID,
		OccurredAt:    attemptAt,
		Account: domain.AccountInput{
			ExternalID: "acct-100",
			Email:      "ops@example.com",
			Status:     "active",
		},
		Conversation: domain.ConversationInput{
			ExternalID: "conv-100",
			Channel:    "telegram",
			Status:     "completed",
			StartedAt:  startedAt,
		},
		Attempt: domain.RequestAttemptInput{
			ExternalID: "attempt-100",
			Provider:   "anthropic",
			Model:      "claude-opus-4-6",
			TokensIn:   120,
			TokensOut:  240,
			CostUSD:    0.02,
			LatencyMS:  420,
			Success:    true,
			CreatedAt:  attemptAt,
		},
	}
}

func marshalJSON(t *testing.T, payload any) []byte {
	t.Helper()

	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("expected payload to marshal, got %v", err)
	}
	return encoded
}

func seedRetryEvent(t *testing.T, store *memory.Store, payload []byte, key domain.EventKey, retryAt time.Time) {
	t.Helper()

	record := domain.IngestEventRecord{
		EventKey:      key,
		SchemaVersion: domain.SupportedIngestSchemaVersion,
		Status:        domain.IngestEventStatusProcessing,
		Payload:       append([]byte(nil), payload...),
		AttemptCount:  1,
		FirstSeenAt:   retryAt.Add(-time.Minute),
		LastAttemptAt: retryAt.Add(-time.Second),
	}
	if _, inserted, err := store.ClaimEvent(context.Background(), record); err != nil {
		t.Fatalf("expected retry seed event to claim, got %v", err)
	} else if !inserted {
		t.Fatal("expected retry seed event to be inserted")
	}

	if _, err := store.RecordEventFailure(context.Background(), key, "seed retry", retryAt, 3); err != nil {
		t.Fatalf("expected retry seed event to schedule retry, got %v", err)
	}
}
