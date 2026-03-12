package memory

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/domain"
)

type accountRecord struct {
	ID         int64
	Source     string
	ExternalID string
	Email      string
	Status     string
}

func (s *Store) ClaimEvent(_ context.Context, event domain.IngestEventRecord) (domain.IngestEventRecord, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := ingestEventKey(event.EventType, event.Source, event.EventID)
	if existing, ok := s.ingestEvents[key]; ok {
		return existing, false, nil
	}

	s.ingestEvents[key] = event
	return event, true, nil
}

func (s *Store) PersistConversationEvent(_ context.Context, event domain.ConversationEventInput) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	accountID := s.upsertAccount(event.Source, event.Account)
	conversationID := s.upsertConversation(event.Source, accountID, event.Conversation)
	if event.Message != nil {
		s.upsertMessage(event.Source, conversationID, *event.Message)
	}
	return nil
}

func (s *Store) PersistInfraSnapshot(_ context.Context, snapshot domain.InfraSnapshotInput) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	item := domain.InfraSnapshot{
		ID:           s.nextSnapshotID,
		VPNPeerCount: snapshot.VPNPeerCount,
		OpenClawUp:   snapshot.OpenClawUp,
		CPUPct:       snapshot.CPUPct,
		MemPct:       snapshot.MemPct,
		CapturedAt:   snapshot.CapturedAt,
	}
	s.nextSnapshotID++
	s.snapshots = append(s.snapshots, item)
	return nil
}

func (s *Store) PersistRequestAttempt(_ context.Context, event domain.RequestAttemptEventInput) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	accountID := s.upsertAccount(event.Source, event.Account)
	conversationID := s.upsertConversation(event.Source, accountID, event.Conversation)
	s.upsertAttempt(event.Source, conversationID, event.Attempt)
	return nil
}

func (s *Store) MarkEventCompleted(_ context.Context, key domain.EventKey, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	event, ok := s.ingestEvents[ingestEventKey(key.EventType, key.Source, key.EventID)]
	if !ok {
		return nil
	}
	event.Status = domain.IngestEventStatusCompleted
	event.LastError = ""
	event.NextRetryAt = time.Time{}
	s.ingestEvents[ingestEventKey(key.EventType, key.Source, key.EventID)] = event
	return nil
}

func (s *Store) RecordEventFailure(_ context.Context, key domain.EventKey, lastError string, nextRetryAt time.Time, maxAttempts int) (domain.IngestResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	eventKey := ingestEventKey(key.EventType, key.Source, key.EventID)
	event, ok := s.ingestEvents[eventKey]
	if !ok {
		return domain.IngestResult{}, nil
	}

	result := domain.IngestResult{
		EventKey:     key,
		AttemptCount: event.AttemptCount,
	}

	if nextRetryAt.IsZero() || event.AttemptCount >= maxAttempts {
		event.Status = domain.IngestEventStatusDeadLetter
		event.LastError = lastError
		event.NextRetryAt = time.Time{}
		s.ingestEvents[eventKey] = event
		result.Outcome = domain.IngestOutcomeDeadLetter
		return result, nil
	}

	event.Status = domain.IngestEventStatusRetryScheduled
	event.LastError = lastError
	event.NextRetryAt = nextRetryAt.UTC()
	s.ingestEvents[eventKey] = event
	result.Outcome = domain.IngestOutcomeRetryScheduled
	result.Queued = true
	return result, nil
}

func (s *Store) LeaseRetryBatch(_ context.Context, now, staleBefore time.Time, limit int) ([]domain.IngestEventRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if limit <= 0 {
		limit = 10
	}

	items := make([]domain.IngestEventRecord, 0)
	for _, event := range s.ingestEvents {
		if !isLeaseableInMemory(event, now, staleBefore) {
			continue
		}
		items = append(items, event)
	}

	sort.Slice(items, func(i, j int) bool {
		return leaseSortTime(items[i]).Before(leaseSortTime(items[j]))
	})
	if len(items) > limit {
		items = items[:limit]
	}

	for i := range items {
		item := items[i]
		item.Status = domain.IngestEventStatusProcessing
		item.AttemptCount++
		item.LastAttemptAt = now.UTC()
		s.ingestEvents[ingestEventKey(item.EventType, item.Source, item.EventID)] = item
		items[i] = item
	}

	return items, nil
}

func (s *Store) GetIngestStatus(_ context.Context, now time.Time) (domain.IngestStatus, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var status domain.IngestStatus
	for _, event := range s.ingestEvents {
		switch event.Status {
		case domain.IngestEventStatusProcessing:
			status.Processing++
			status.QueueDepth++
			if age := now.Sub(event.FirstSeenAt).Seconds(); age > status.OldestPendingAgeSeconds {
				status.OldestPendingAgeSeconds = age
			}
		case domain.IngestEventStatusRetryScheduled:
			status.RetryScheduled++
			status.QueueDepth++
			if age := now.Sub(event.FirstSeenAt).Seconds(); age > status.OldestPendingAgeSeconds {
				status.OldestPendingAgeSeconds = age
			}
			if age := now.Sub(event.NextRetryAt).Seconds(); age > 0 && age > status.OldestRetryAgeSeconds {
				status.OldestRetryAgeSeconds = age
			}
		case domain.IngestEventStatusDeadLetter:
			status.DeadLetter++
		}
	}
	return status, nil
}

func (s *Store) upsertAccount(source string, account domain.AccountInput) int64 {
	key := ingestKey(source, account.ExternalID)
	for i := range s.accounts {
		if ingestKey(s.accounts[i].Source, s.accounts[i].ExternalID) != key {
			continue
		}
		s.accounts[i].Email = account.Email
		s.accounts[i].Status = account.Status
		return s.accounts[i].ID
	}

	id := s.nextAccountID
	s.nextAccountID++
	s.accounts = append(s.accounts, accountRecord{
		ID:         id,
		Source:     source,
		ExternalID: account.ExternalID,
		Email:      account.Email,
		Status:     account.Status,
	})
	return id
}

func (s *Store) upsertConversation(source string, accountID int64, conversation domain.ConversationInput) int64 {
	key := ingestKey(source, conversation.ExternalID)
	if id, ok := s.conversationByExternal[key]; ok {
		for i := range s.conversations {
			if s.conversations[i].ID != id {
				continue
			}
			s.conversations[i].AccountID = accountID
			s.conversations[i].Channel = conversation.Channel
			s.conversations[i].Status = conversation.Status
			s.conversations[i].StartedAt = conversation.StartedAt
			s.conversations[i].EndedAt = conversation.EndedAt
			return id
		}
	}

	id := s.nextConversationID
	s.nextConversationID++
	s.conversations = append(s.conversations, domain.Conversation{
		ID:        id,
		AccountID: accountID,
		Channel:   conversation.Channel,
		Status:    conversation.Status,
		StartedAt: conversation.StartedAt,
		EndedAt:   conversation.EndedAt,
	})
	s.conversationByExternal[key] = id
	return id
}

func (s *Store) upsertMessage(source string, conversationID int64, message domain.MessageInput) {
	key := ingestKey(source, message.ExternalID)
	if id, ok := s.messageByExternal[key]; ok {
		for i := range s.messages {
			if s.messages[i].ID != id {
				continue
			}
			s.messages[i].ConversationID = conversationID
			s.messages[i].Role = message.Role
			s.messages[i].ContentMasked = message.ContentMasked
			s.messages[i].CreatedAt = message.CreatedAt
			return
		}
	}

	id := s.nextMessageID
	s.nextMessageID++
	s.messages = append(s.messages, domain.Message{
		ID:             id,
		ConversationID: conversationID,
		Role:           message.Role,
		ContentMasked:  message.ContentMasked,
		CreatedAt:      message.CreatedAt,
	})
	s.messageByExternal[key] = id
}

func (s *Store) upsertAttempt(source string, conversationID int64, attempt domain.RequestAttemptInput) {
	key := ingestKey(source, attempt.ExternalID)
	if id, ok := s.attemptByExternal[key]; ok {
		for i := range s.attempts {
			if s.attempts[i].ID != id {
				continue
			}
			s.attempts[i].ConversationID = conversationID
			s.attempts[i].Provider = attempt.Provider
			s.attempts[i].Model = attempt.Model
			s.attempts[i].TokensIn = attempt.TokensIn
			s.attempts[i].TokensOut = attempt.TokensOut
			s.attempts[i].CostUSD = attempt.CostUSD
			s.attempts[i].LatencyMS = attempt.LatencyMS
			s.attempts[i].Success = attempt.Success
			s.attempts[i].ErrorCode = attempt.ErrorCode
			s.attempts[i].CreatedAt = attempt.CreatedAt
			return
		}
	}

	id := s.nextAttemptID
	s.nextAttemptID++
	s.attempts = append(s.attempts, domain.RequestAttempt{
		ID:             id,
		ConversationID: conversationID,
		Provider:       attempt.Provider,
		Model:          attempt.Model,
		TokensIn:       attempt.TokensIn,
		TokensOut:      attempt.TokensOut,
		CostUSD:        attempt.CostUSD,
		LatencyMS:      attempt.LatencyMS,
		Success:        attempt.Success,
		ErrorCode:      attempt.ErrorCode,
		CreatedAt:      attempt.CreatedAt,
	})
	s.attemptByExternal[key] = id
}

func ingestKey(source, externalID string) string {
	return encodeKeyPart(source, externalID)
}

func ingestEventKey(eventType, source, eventID string) string {
	return encodeKeyPart(eventType, source, eventID)
}

func isLeaseableInMemory(event domain.IngestEventRecord, now, staleBefore time.Time) bool {
	switch event.Status {
	case domain.IngestEventStatusRetryScheduled:
		return !event.NextRetryAt.After(now)
	case domain.IngestEventStatusProcessing:
		return !event.LastAttemptAt.After(staleBefore)
	default:
		return false
	}
}

func leaseSortTime(event domain.IngestEventRecord) time.Time {
	if event.Status == domain.IngestEventStatusRetryScheduled && !event.NextRetryAt.IsZero() {
		return event.NextRetryAt
	}
	return event.LastAttemptAt
}

func encodeKeyPart(parts ...string) string {
	encoded := ""
	for _, part := range parts {
		encoded += fmt.Sprintf("%d:%s|", len(part), part)
	}
	return encoded
}
