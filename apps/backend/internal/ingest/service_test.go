package ingest

import (
	"context"
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

	items, err := store.ListMessages(context.Background(), 2, domain.Pagination{Page: 1, PageSize: 100})
	if err != nil {
		t.Fatalf("expected list messages to succeed, got %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one message for ingested conversation, got %d", len(items))
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
