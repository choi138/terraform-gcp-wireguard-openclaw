package ingest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/domain"
	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/repository"
)

const (
	EventTypeConversation         = "conversation_event"
	EventTypeInfraSnapshot        = "infra_snapshot"
	EventTypeRequestAttempt       = "request_attempt"
	completionRetryPrefix         = "mark event completed: "
	defaultProcessingLeaseTimeout = 2 * time.Minute
)

type Repository interface {
	ClaimEvent(ctx context.Context, event domain.IngestEventRecord) (domain.IngestEventRecord, bool, error)
	PersistConversationEvent(ctx context.Context, event domain.ConversationEventInput) error
	PersistInfraSnapshot(ctx context.Context, snapshot domain.InfraSnapshotInput) error
	PersistRequestAttempt(ctx context.Context, event domain.RequestAttemptEventInput) error
	MarkEventCompleted(ctx context.Context, key domain.EventKey, processedAt time.Time) error
	RecordEventFailure(ctx context.Context, key domain.EventKey, lastError string, nextRetryAt time.Time, maxAttempts int) (domain.IngestResult, error)
	LeaseRetryBatch(ctx context.Context, now, staleBefore time.Time, limit int) ([]domain.IngestEventRecord, error)
	GetIngestStatus(ctx context.Context, now time.Time) (domain.IngestStatus, error)
}

type Config struct {
	RetryBaseDelay         time.Duration
	RetryMaxDelay          time.Duration
	RetryMaxAttempts       int
	ProcessingLeaseTimeout time.Duration
}

type RetryBatchResult struct {
	Processed    int
	Completed    int
	Rescheduled  int
	DeadLettered int
}

type Service struct {
	repo   Repository
	now    func() time.Time
	config Config
}

func NewService(repo Repository, cfg Config) *Service {
	if repo == nil {
		panic("ingest.NewService requires Repository")
	}
	if cfg.RetryBaseDelay <= 0 {
		cfg.RetryBaseDelay = time.Second
	}
	if cfg.RetryMaxDelay <= 0 {
		cfg.RetryMaxDelay = 30 * time.Second
	}
	if cfg.RetryMaxAttempts <= 0 {
		cfg.RetryMaxAttempts = 5
	}
	if cfg.ProcessingLeaseTimeout <= 0 {
		cfg.ProcessingLeaseTimeout = defaultProcessingLeaseTimeout
	}

	return &Service{
		repo:   repo,
		now:    func() time.Time { return time.Now().UTC() },
		config: cfg,
	}
}

func (s *Service) IngestConversationEvent(ctx context.Context, event domain.ConversationEventInput) (domain.IngestResult, error) {
	return s.ingest(ctx, domain.EventKey{
		EventType: EventTypeConversation,
		Source:    event.Source,
		EventID:   event.EventID,
	}, event.SchemaVersion, event, func(ctx context.Context) error {
		return s.repo.PersistConversationEvent(ctx, event)
	})
}

func (s *Service) IngestInfraSnapshot(ctx context.Context, snapshot domain.InfraSnapshotInput) (domain.IngestResult, error) {
	return s.ingest(ctx, domain.EventKey{
		EventType: EventTypeInfraSnapshot,
		Source:    snapshot.Source,
		EventID:   snapshot.EventID,
	}, snapshot.SchemaVersion, snapshot, func(ctx context.Context) error {
		return s.repo.PersistInfraSnapshot(ctx, snapshot)
	})
}

func (s *Service) IngestRequestAttempt(ctx context.Context, event domain.RequestAttemptEventInput) (domain.IngestResult, error) {
	return s.ingest(ctx, domain.EventKey{
		EventType: EventTypeRequestAttempt,
		Source:    event.Source,
		EventID:   event.EventID,
	}, event.SchemaVersion, event, func(ctx context.Context) error {
		return s.repo.PersistRequestAttempt(ctx, event)
	})
}

func (s *Service) GetStatus(ctx context.Context) (domain.IngestStatus, error) {
	return s.repo.GetIngestStatus(ctx, s.now())
}

func (s *Service) ProcessDueRetries(ctx context.Context, limit int) (RetryBatchResult, error) {
	if limit <= 0 {
		limit = 10
	}

	now := s.now()
	staleBefore := now.Add(-s.config.ProcessingLeaseTimeout)
	batch, err := s.repo.LeaseRetryBatch(ctx, now, staleBefore, limit)
	if err != nil {
		return RetryBatchResult{}, err
	}

	result := RetryBatchResult{Processed: len(batch)}
	var firstErr error
	for _, item := range batch {
		err := s.processLeasedEvent(ctx, item)
		switch {
		case err == nil:
			result.Completed++
		case errors.Is(err, errRetryScheduled):
			result.Rescheduled++
		case errors.Is(err, errDeadLettered):
			result.DeadLettered++
		default:
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	return result, firstErr
}

var (
	errRetryScheduled = errors.New("retry scheduled")
	errDeadLettered   = errors.New("dead lettered")
)

func (s *Service) ingest(ctx context.Context, key domain.EventKey, schemaVersion int, payload any, persist func(context.Context) error) (domain.IngestResult, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return domain.IngestResult{}, err
	}

	now := s.now()
	event := domain.IngestEventRecord{
		EventKey:      key,
		SchemaVersion: schemaVersion,
		Status:        domain.IngestEventStatusProcessing,
		Payload:       payloadBytes,
		AttemptCount:  1,
		FirstSeenAt:   now,
		LastAttemptAt: now,
	}

	stored, inserted, err := s.repo.ClaimEvent(ctx, event)
	if err != nil {
		return domain.IngestResult{}, err
	}
	if !inserted {
		return existingResult(stored), nil
	}

	if err := persist(ctx); err != nil {
		failureResult, failureErr := s.handleFailure(ctx, key, stored.AttemptCount, err)
		if failureErr != nil {
			return domain.IngestResult{}, failureErr
		}
		if failureResult.Outcome == domain.IngestOutcomeRetryScheduled {
			return failureResult, nil
		}
		return failureResult, err
	}

	if err := s.repo.MarkEventCompleted(ctx, key, now); err != nil {
		result, completionErr := s.handleCompletionFailure(ctx, key, stored.AttemptCount, err)
		if completionErr != nil {
			return domain.IngestResult{}, completionErr
		}
		if result.Outcome == domain.IngestOutcomeRetryScheduled {
			return result, nil
		}
		return result, err
	}

	return domain.IngestResult{
		EventKey:     key,
		Outcome:      domain.IngestOutcomeAccepted,
		AttemptCount: stored.AttemptCount,
	}, nil
}

func (s *Service) processLeasedEvent(ctx context.Context, event domain.IngestEventRecord) error {
	if isCompletionRetry(event) {
		return s.retryCompletionOnly(ctx, event)
	}

	var persistErr error

	switch event.EventType {
	case EventTypeConversation:
		var payload domain.ConversationEventInput
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			persistErr = fmt.Errorf("decode conversation event payload: %w", err)
		} else {
			persistErr = s.repo.PersistConversationEvent(ctx, payload)
		}
	case EventTypeInfraSnapshot:
		var payload domain.InfraSnapshotInput
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			persistErr = fmt.Errorf("decode infra snapshot payload: %w", err)
		} else {
			persistErr = s.repo.PersistInfraSnapshot(ctx, payload)
		}
	case EventTypeRequestAttempt:
		var payload domain.RequestAttemptEventInput
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			persistErr = fmt.Errorf("decode request attempt payload: %w", err)
		} else {
			persistErr = s.repo.PersistRequestAttempt(ctx, payload)
		}
	default:
		persistErr = fmt.Errorf("unsupported retry event type %q", event.EventType)
	}

	if persistErr != nil {
		result, err := s.handleFailure(ctx, event.EventKey, event.AttemptCount, persistErr)
		if err != nil {
			return err
		}
		switch result.Outcome {
		case domain.IngestOutcomeRetryScheduled:
			return errRetryScheduled
		case domain.IngestOutcomeDeadLetter:
			return errDeadLettered
		default:
			return persistErr
		}
	}

	return s.completeLeasedEvent(ctx, event)
}

func (s *Service) handleFailure(ctx context.Context, key domain.EventKey, attemptCount int, err error) (domain.IngestResult, error) {
	nextRetryAt := time.Time{}
	if errors.Is(err, repository.ErrTransient) {
		nextRetryAt = s.now().Add(s.backoffForAttempt(attemptCount))
	}
	result, recordErr := s.repo.RecordEventFailure(ctx, key, err.Error(), nextRetryAt, s.config.RetryMaxAttempts)
	if recordErr != nil {
		return domain.IngestResult{}, recordErr
	}
	return result, nil
}

func (s *Service) handleCompletionFailure(ctx context.Context, key domain.EventKey, attemptCount int, err error) (domain.IngestResult, error) {
	return s.handleFailure(ctx, key, attemptCount, fmt.Errorf("%s%w", completionRetryPrefix, err))
}

func (s *Service) retryCompletionOnly(ctx context.Context, event domain.IngestEventRecord) error {
	return s.completeLeasedEvent(ctx, event)
}

func isCompletionRetry(event domain.IngestEventRecord) bool {
	return strings.HasPrefix(event.LastError, completionRetryPrefix)
}

func (s *Service) completeLeasedEvent(ctx context.Context, event domain.IngestEventRecord) error {
	if err := s.repo.MarkEventCompleted(ctx, event.EventKey, s.now()); err != nil {
		result, completionErr := s.handleCompletionFailure(ctx, event.EventKey, event.AttemptCount, err)
		if completionErr != nil {
			return completionErr
		}
		switch result.Outcome {
		case domain.IngestOutcomeRetryScheduled:
			return errRetryScheduled
		case domain.IngestOutcomeDeadLetter:
			return errDeadLettered
		default:
			return err
		}
	}

	return nil
}

func (s *Service) backoffForAttempt(attemptCount int) time.Duration {
	if attemptCount < 1 {
		attemptCount = 1
	}

	delay := s.config.RetryBaseDelay
	for i := 1; i < attemptCount; i++ {
		delay *= 2
		if delay >= s.config.RetryMaxDelay {
			return s.config.RetryMaxDelay
		}
	}
	return delay
}

func existingResult(event domain.IngestEventRecord) domain.IngestResult {
	result := domain.IngestResult{
		EventKey:     event.EventKey,
		AttemptCount: event.AttemptCount,
	}

	switch event.Status {
	case domain.IngestEventStatusCompleted:
		result.Outcome = domain.IngestOutcomeDuplicate
		result.Duplicate = true
	case domain.IngestEventStatusRetryScheduled, domain.IngestEventStatusProcessing:
		result.Outcome = domain.IngestOutcomeRetryScheduled
		result.Queued = true
	case domain.IngestEventStatusDeadLetter:
		result.Outcome = domain.IngestOutcomeDeadLetter
	default:
		result.Outcome = domain.IngestOutcomeDuplicate
		result.Duplicate = true
	}

	return result
}
