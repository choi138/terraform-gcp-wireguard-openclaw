package dto

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/domain"
)

type ValidationError struct {
	Messages []string
}

func (e ValidationError) Error() string {
	return strings.Join(e.Messages, "; ")
}

type conversationEventRequest struct {
	SchemaVersion int                         `json:"schema_version"`
	Source        string                      `json:"source"`
	EventID       string                      `json:"event_id"`
	OccurredAt    time.Time                   `json:"occurred_at"`
	Account       accountRequest              `json:"account"`
	Conversation  conversationRequest         `json:"conversation"`
	Message       *conversationMessageRequest `json:"message,omitempty"`
}

type infraSnapshotRequest struct {
	SchemaVersion int       `json:"schema_version"`
	Source        string    `json:"source"`
	EventID       string    `json:"event_id"`
	CapturedAt    time.Time `json:"captured_at"`
	VPNPeerCount  *int      `json:"vpn_peer_count"`
	OpenClawUp    *bool     `json:"openclaw_up"`
	CPUPct        *float64  `json:"cpu_pct"`
	MemPct        *float64  `json:"mem_pct"`
}

type requestAttemptEventRequest struct {
	SchemaVersion int                   `json:"schema_version"`
	Source        string                `json:"source"`
	EventID       string                `json:"event_id"`
	OccurredAt    time.Time             `json:"occurred_at"`
	Account       accountRequest        `json:"account"`
	Conversation  conversationRequest   `json:"conversation"`
	Attempt       requestAttemptRequest `json:"attempt"`
}

type accountRequest struct {
	ExternalID string `json:"external_id"`
	Email      string `json:"email"`
	Status     string `json:"status"`
}

type conversationRequest struct {
	ExternalID string     `json:"external_id"`
	Channel    string     `json:"channel"`
	Status     string     `json:"status"`
	StartedAt  time.Time  `json:"started_at"`
	EndedAt    *time.Time `json:"ended_at,omitempty"`
}

type conversationMessageRequest struct {
	ExternalID    string    `json:"external_id"`
	Role          string    `json:"role"`
	ContentMasked string    `json:"content_masked"`
	CreatedAt     time.Time `json:"created_at"`
}

type requestAttemptRequest struct {
	ExternalID string    `json:"external_id"`
	Provider   string    `json:"provider"`
	Model      string    `json:"model"`
	TokensIn   *int64    `json:"tokens_in"`
	TokensOut  *int64    `json:"tokens_out"`
	CostUSD    *float64  `json:"cost_usd"`
	LatencyMS  *int64    `json:"latency_ms"`
	Success    *bool     `json:"success"`
	ErrorCode  *string   `json:"error_code,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

func DecodeConversationEvent(r *http.Request, maxBytes int64) (domain.ConversationEventInput, error) {
	var payload conversationEventRequest
	if err := decodeJSON(r, maxBytes, &payload); err != nil {
		return domain.ConversationEventInput{}, err
	}

	messages := validateEnvelope(payload.SchemaVersion, payload.Source, payload.EventID, payload.OccurredAt)
	messages = append(messages, validateAccount(payload.Account)...)
	messages = append(messages, validateConversation(payload.Conversation)...)
	if payload.Message != nil {
		messages = append(messages, validateMessage(*payload.Message)...)
	}
	if payload.Conversation.EndedAt != nil && payload.Conversation.EndedAt.Before(payload.Conversation.StartedAt) {
		messages = append(messages, "conversation.ended_at must be greater than or equal to conversation.started_at")
	}
	if payload.Message != nil && payload.Message.CreatedAt.Before(payload.Conversation.StartedAt) {
		messages = append(messages, "message.created_at must be greater than or equal to conversation.started_at")
	}
	if payload.Message != nil && payload.Conversation.EndedAt != nil && payload.Message.CreatedAt.After(*payload.Conversation.EndedAt) {
		messages = append(messages, "message.created_at must be less than or equal to conversation.ended_at")
	}
	if len(messages) > 0 {
		return domain.ConversationEventInput{}, ValidationError{Messages: messages}
	}

	var message *domain.MessageInput
	if payload.Message != nil {
		message = &domain.MessageInput{
			ExternalID:    strings.TrimSpace(payload.Message.ExternalID),
			Role:          strings.TrimSpace(payload.Message.Role),
			ContentMasked: strings.TrimSpace(payload.Message.ContentMasked),
			CreatedAt:     payload.Message.CreatedAt.UTC(),
		}
	}

	return domain.ConversationEventInput{
		SchemaVersion: payload.SchemaVersion,
		Source:        strings.TrimSpace(payload.Source),
		EventID:       strings.TrimSpace(payload.EventID),
		OccurredAt:    payload.OccurredAt.UTC(),
		Account: domain.AccountInput{
			ExternalID: strings.TrimSpace(payload.Account.ExternalID),
			Email:      strings.TrimSpace(payload.Account.Email),
			Status:     strings.TrimSpace(payload.Account.Status),
		},
		Conversation: domain.ConversationInput{
			ExternalID: strings.TrimSpace(payload.Conversation.ExternalID),
			Channel:    strings.TrimSpace(payload.Conversation.Channel),
			Status:     strings.TrimSpace(payload.Conversation.Status),
			StartedAt:  payload.Conversation.StartedAt.UTC(),
			EndedAt:    toUTCPtr(payload.Conversation.EndedAt),
		},
		Message: message,
	}, nil
}

func DecodeInfraSnapshot(r *http.Request, maxBytes int64) (domain.InfraSnapshotInput, error) {
	var payload infraSnapshotRequest
	if err := decodeJSON(r, maxBytes, &payload); err != nil {
		return domain.InfraSnapshotInput{}, err
	}

	messages := validateEnvelope(payload.SchemaVersion, payload.Source, payload.EventID, payload.CapturedAt)
	messages = append(messages, validateInfraSnapshot(payload)...)
	if len(messages) > 0 {
		return domain.InfraSnapshotInput{}, ValidationError{Messages: messages}
	}

	return domain.InfraSnapshotInput{
		SchemaVersion: payload.SchemaVersion,
		Source:        strings.TrimSpace(payload.Source),
		EventID:       strings.TrimSpace(payload.EventID),
		CapturedAt:    payload.CapturedAt.UTC(),
		VPNPeerCount:  *payload.VPNPeerCount,
		OpenClawUp:    *payload.OpenClawUp,
		CPUPct:        *payload.CPUPct,
		MemPct:        *payload.MemPct,
	}, nil
}

func DecodeRequestAttemptEvent(r *http.Request, maxBytes int64) (domain.RequestAttemptEventInput, error) {
	var payload requestAttemptEventRequest
	if err := decodeJSON(r, maxBytes, &payload); err != nil {
		return domain.RequestAttemptEventInput{}, err
	}

	messages := validateEnvelope(payload.SchemaVersion, payload.Source, payload.EventID, payload.OccurredAt)
	messages = append(messages, validateAccount(payload.Account)...)
	messages = append(messages, validateConversation(payload.Conversation)...)
	messages = append(messages, validateAttempt(payload.Attempt)...)
	if payload.Conversation.EndedAt != nil && payload.Conversation.EndedAt.Before(payload.Conversation.StartedAt) {
		messages = append(messages, "conversation.ended_at must be greater than or equal to conversation.started_at")
	}
	if payload.Attempt.CreatedAt.Before(payload.Conversation.StartedAt) {
		messages = append(messages, "attempt.created_at must be greater than or equal to conversation.started_at")
	}
	if payload.Conversation.EndedAt != nil && payload.Attempt.CreatedAt.After(*payload.Conversation.EndedAt) {
		messages = append(messages, "attempt.created_at must be less than or equal to conversation.ended_at")
	}
	if len(messages) > 0 {
		return domain.RequestAttemptEventInput{}, ValidationError{Messages: messages}
	}

	var errorCode *string
	if payload.Attempt.ErrorCode != nil {
		trimmed := strings.TrimSpace(*payload.Attempt.ErrorCode)
		errorCode = &trimmed
	}

	return domain.RequestAttemptEventInput{
		SchemaVersion: payload.SchemaVersion,
		Source:        strings.TrimSpace(payload.Source),
		EventID:       strings.TrimSpace(payload.EventID),
		OccurredAt:    payload.OccurredAt.UTC(),
		Account: domain.AccountInput{
			ExternalID: strings.TrimSpace(payload.Account.ExternalID),
			Email:      strings.TrimSpace(payload.Account.Email),
			Status:     strings.TrimSpace(payload.Account.Status),
		},
		Conversation: domain.ConversationInput{
			ExternalID: strings.TrimSpace(payload.Conversation.ExternalID),
			Channel:    strings.TrimSpace(payload.Conversation.Channel),
			Status:     strings.TrimSpace(payload.Conversation.Status),
			StartedAt:  payload.Conversation.StartedAt.UTC(),
			EndedAt:    toUTCPtr(payload.Conversation.EndedAt),
		},
		Attempt: domain.RequestAttemptInput{
			ExternalID: strings.TrimSpace(payload.Attempt.ExternalID),
			Provider:   strings.TrimSpace(payload.Attempt.Provider),
			Model:      strings.TrimSpace(payload.Attempt.Model),
			TokensIn:   *payload.Attempt.TokensIn,
			TokensOut:  *payload.Attempt.TokensOut,
			CostUSD:    *payload.Attempt.CostUSD,
			LatencyMS:  *payload.Attempt.LatencyMS,
			Success:    *payload.Attempt.Success,
			ErrorCode:  errorCode,
			CreatedAt:  payload.Attempt.CreatedAt.UTC(),
		},
	}, nil
}

func decodeJSON(r *http.Request, maxBytes int64, dest any) error {
	if r.Body == nil {
		return ValidationError{Messages: []string{"request body is required"}}
	}

	var (
		body []byte
		err  error
	)
	if maxBytes > 0 {
		body, err = io.ReadAll(io.LimitReader(r.Body, maxBytes+1))
		if err != nil {
			return ValidationError{Messages: []string{err.Error()}}
		}
		if int64(len(body)) > maxBytes {
			return ValidationError{Messages: []string{fmt.Sprintf("request body must be at most %d bytes", maxBytes)}}
		}
	} else {
		body, err = io.ReadAll(r.Body)
		if err != nil {
			return ValidationError{Messages: []string{err.Error()}}
		}
	}

	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dest); err != nil {
		var syntaxErr *json.SyntaxError
		var typeErr *json.UnmarshalTypeError
		switch {
		case errors.As(err, &syntaxErr):
			return ValidationError{Messages: []string{fmt.Sprintf("request body contains invalid JSON at byte %d", syntaxErr.Offset)}}
		case errors.As(err, &typeErr):
			return ValidationError{Messages: []string{fmt.Sprintf("%s has invalid type", typeErr.Field)}}
		case errors.Is(err, io.EOF):
			return ValidationError{Messages: []string{"request body is required"}}
		default:
			return ValidationError{Messages: []string{err.Error()}}
		}
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return ValidationError{Messages: []string{"request body must contain a single JSON object"}}
	}
	return nil
}

func validateEnvelope(schemaVersion int, source, eventID string, ts time.Time) []string {
	var messages []string
	if schemaVersion != domain.SupportedIngestSchemaVersion {
		messages = append(messages, fmt.Sprintf("schema_version must be %d", domain.SupportedIngestSchemaVersion))
	}
	if strings.TrimSpace(source) == "" {
		messages = append(messages, "source is required")
	}
	if strings.TrimSpace(eventID) == "" {
		messages = append(messages, "event_id is required")
	}
	if ts.IsZero() {
		messages = append(messages, "timestamp field is required and must be RFC3339")
	}
	return messages
}

func validateAccount(account accountRequest) []string {
	var messages []string
	if strings.TrimSpace(account.ExternalID) == "" {
		messages = append(messages, "account.external_id is required")
	}
	if strings.TrimSpace(account.Email) == "" || !strings.Contains(account.Email, "@") {
		messages = append(messages, "account.email must be a valid email address")
	}
	if strings.TrimSpace(account.Status) == "" {
		messages = append(messages, "account.status is required")
	}
	return messages
}

func validateConversation(conversation conversationRequest) []string {
	var messages []string
	if strings.TrimSpace(conversation.ExternalID) == "" {
		messages = append(messages, "conversation.external_id is required")
	}
	if strings.TrimSpace(conversation.Channel) == "" {
		messages = append(messages, "conversation.channel is required")
	}
	if strings.TrimSpace(conversation.Status) == "" {
		messages = append(messages, "conversation.status is required")
	}
	if conversation.StartedAt.IsZero() {
		messages = append(messages, "conversation.started_at is required and must be RFC3339")
	}
	return messages
}

func validateMessage(message conversationMessageRequest) []string {
	var messages []string
	if strings.TrimSpace(message.ExternalID) == "" {
		messages = append(messages, "message.external_id is required")
	}
	switch strings.TrimSpace(message.Role) {
	case "user", "assistant", "system", "tool":
	default:
		messages = append(messages, "message.role must be one of: user,assistant,system,tool")
	}
	if strings.TrimSpace(message.ContentMasked) == "" {
		messages = append(messages, "message.content_masked is required")
	}
	if message.CreatedAt.IsZero() {
		messages = append(messages, "message.created_at is required and must be RFC3339")
	}
	return messages
}

func validateInfraSnapshot(snapshot infraSnapshotRequest) []string {
	var messages []string
	if snapshot.VPNPeerCount == nil {
		messages = append(messages, "vpn_peer_count is required")
	} else if *snapshot.VPNPeerCount < 0 {
		messages = append(messages, "vpn_peer_count must be greater than or equal to 0")
	}
	if snapshot.OpenClawUp == nil {
		messages = append(messages, "openclaw_up is required")
	}
	if snapshot.CPUPct == nil {
		messages = append(messages, "cpu_pct is required")
	} else if *snapshot.CPUPct < 0 || *snapshot.CPUPct > 100 {
		messages = append(messages, "cpu_pct must be between 0 and 100")
	}
	if snapshot.MemPct == nil {
		messages = append(messages, "mem_pct is required")
	} else if *snapshot.MemPct < 0 || *snapshot.MemPct > 100 {
		messages = append(messages, "mem_pct must be between 0 and 100")
	}
	return messages
}

func validateAttempt(attempt requestAttemptRequest) []string {
	var messages []string
	if strings.TrimSpace(attempt.ExternalID) == "" {
		messages = append(messages, "attempt.external_id is required")
	}
	if strings.TrimSpace(attempt.Provider) == "" {
		messages = append(messages, "attempt.provider is required")
	}
	if strings.TrimSpace(attempt.Model) == "" {
		messages = append(messages, "attempt.model is required")
	}
	if attempt.TokensIn == nil {
		messages = append(messages, "attempt.tokens_in is required")
	} else if *attempt.TokensIn < 0 {
		messages = append(messages, "attempt.tokens_in must be greater than or equal to 0")
	}
	if attempt.TokensOut == nil {
		messages = append(messages, "attempt.tokens_out is required")
	} else if *attempt.TokensOut < 0 {
		messages = append(messages, "attempt.tokens_out must be greater than or equal to 0")
	}
	if attempt.CostUSD == nil {
		messages = append(messages, "attempt.cost_usd is required")
	} else if *attempt.CostUSD < 0 {
		messages = append(messages, "attempt.cost_usd must be greater than or equal to 0")
	}
	if attempt.LatencyMS == nil {
		messages = append(messages, "attempt.latency_ms is required")
	} else if *attempt.LatencyMS < 0 {
		messages = append(messages, "attempt.latency_ms must be greater than or equal to 0")
	}
	if attempt.Success == nil {
		messages = append(messages, "attempt.success is required")
	}
	if attempt.CreatedAt.IsZero() {
		messages = append(messages, "attempt.created_at is required and must be RFC3339")
	}
	return messages
}

func toUTCPtr(v *time.Time) *time.Time {
	if v == nil {
		return nil
	}
	utc := v.UTC()
	return &utc
}
