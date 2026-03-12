package memory

import (
	"context"
	"sort"
	"time"

	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/domain"
)

func (s *Store) CompactRawMessagePayloads(_ context.Context, cutoff time.Time, limit int, dryRun bool) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	candidates := make([]int64, 0, limit)
	for _, message := range s.messages {
		if len(candidates) >= limit {
			break
		}
		if message.CreatedAt.Before(cutoff) && len(s.messageRawPayload[message.ID]) > 0 {
			candidates = append(candidates, message.ID)
		}
	}
	if dryRun {
		return len(candidates), nil
	}
	for _, id := range candidates {
		delete(s.messageRawPayload, id)
	}
	return len(candidates), nil
}

func (s *Store) DeleteExpiredAuditEvents(_ context.Context, cutoff time.Time, limit int, dryRun bool) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	indexes := make([]int, 0, limit)
	for i, event := range s.audits {
		if len(indexes) >= limit {
			break
		}
		if event.CreatedAt.Before(cutoff) {
			indexes = append(indexes, i)
		}
	}
	if dryRun {
		return len(indexes), nil
	}
	s.audits = removeIndexes(s.audits, indexes)
	return len(indexes), nil
}

func (s *Store) DeleteExpiredInfraSnapshots(_ context.Context, cutoff time.Time, limit int, dryRun bool) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	protectedID := int64(0)
	if len(s.snapshots) > 0 {
		latest := s.snapshots[0]
		for _, item := range s.snapshots[1:] {
			if item.CapturedAt.After(latest.CapturedAt) {
				latest = item
			}
		}
		protectedID = latest.ID
	}

	indexes := make([]int, 0, limit)
	for i, snapshot := range s.snapshots {
		if len(indexes) >= limit {
			break
		}
		if snapshot.ID == protectedID {
			continue
		}
		if snapshot.CapturedAt.Before(cutoff) {
			indexes = append(indexes, i)
		}
	}
	if dryRun {
		return len(indexes), nil
	}
	s.snapshots = removeIndexes(s.snapshots, indexes)
	return len(indexes), nil
}

func (s *Store) DeleteExpiredIngestEvents(_ context.Context, cutoff time.Time, limit int, dryRun bool) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	keys := make([]string, 0, limit)
	for key, event := range s.ingestEvents {
		if len(keys) >= limit {
			break
		}
		if event.Status != domain.IngestEventStatusCompleted && event.Status != domain.IngestEventStatusDeadLetter {
			continue
		}
		if event.FirstSeenAt.Before(cutoff) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	if dryRun {
		return len(keys), nil
	}
	for _, key := range keys {
		delete(s.ingestEvents, key)
	}
	return len(keys), nil
}

func removeIndexes[T any](items []T, indexes []int) []T {
	if len(indexes) == 0 {
		return items
	}
	out := make([]T, 0, len(items)-len(indexes))
	indexSet := make(map[int]struct{}, len(indexes))
	for _, idx := range indexes {
		indexSet[idx] = struct{}{}
	}
	for i, item := range items {
		if _, ok := indexSet[i]; ok {
			continue
		}
		out = append(out, item)
	}
	return out
}
