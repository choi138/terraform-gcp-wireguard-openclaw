package domain

import "time"

const SupportedIngestSchemaVersion = 1

type EventKey struct {
	EventType string `json:"event_type"`
	Source    string `json:"source"`
	EventID   string `json:"event_id"`
}

type IngestOutcome string

const (
	IngestOutcomeAccepted       IngestOutcome = "accepted"
	IngestOutcomeDuplicate      IngestOutcome = "duplicate"
	IngestOutcomeRetryScheduled IngestOutcome = "retry_scheduled"
	IngestOutcomeDeadLetter     IngestOutcome = "dead_letter"
)

const (
	IngestEventStatusProcessing     = "processing"
	IngestEventStatusRetryScheduled = "retry_scheduled"
	IngestEventStatusCompleted      = "completed"
	IngestEventStatusDeadLetter     = "dead_letter"
)

type IngestResult struct {
	EventKey
	Outcome      IngestOutcome `json:"outcome"`
	Duplicate    bool          `json:"duplicate"`
	Queued       bool          `json:"queued"`
	AttemptCount int           `json:"attempt_count,omitempty"`
}

type IngestStatus struct {
	QueueDepth              int     `json:"queue_depth"`
	RetryScheduled          int     `json:"retry_scheduled"`
	Processing              int     `json:"processing"`
	DeadLetter              int     `json:"dead_letter"`
	OldestPendingAgeSeconds float64 `json:"oldest_pending_age_seconds"`
	OldestRetryAgeSeconds   float64 `json:"oldest_retry_age_seconds"`
}

type AccountInput struct {
	ExternalID string `json:"external_id"`
	Email      string `json:"email"`
	Status     string `json:"status"`
}

type ConversationInput struct {
	ExternalID string     `json:"external_id"`
	Channel    string     `json:"channel"`
	Status     string     `json:"status"`
	StartedAt  time.Time  `json:"started_at"`
	EndedAt    *time.Time `json:"ended_at,omitempty"`
}

type MessageInput struct {
	ExternalID    string    `json:"external_id"`
	Role          string    `json:"role"`
	ContentMasked string    `json:"content_masked"`
	CreatedAt     time.Time `json:"created_at"`
}

type ConversationEventInput struct {
	SchemaVersion int               `json:"schema_version"`
	Source        string            `json:"source"`
	EventID       string            `json:"event_id"`
	OccurredAt    time.Time         `json:"occurred_at"`
	Account       AccountInput      `json:"account"`
	Conversation  ConversationInput `json:"conversation"`
	Message       *MessageInput     `json:"message,omitempty"`
}

type InfraSnapshotInput struct {
	SchemaVersion int       `json:"schema_version"`
	Source        string    `json:"source"`
	EventID       string    `json:"event_id"`
	CapturedAt    time.Time `json:"captured_at"`
	VPNPeerCount  int       `json:"vpn_peer_count"`
	OpenClawUp    bool      `json:"openclaw_up"`
	CPUPct        float64   `json:"cpu_pct"`
	MemPct        float64   `json:"mem_pct"`
}

type RequestAttemptInput struct {
	ExternalID string    `json:"external_id"`
	Provider   string    `json:"provider"`
	Model      string    `json:"model"`
	TokensIn   int64     `json:"tokens_in"`
	TokensOut  int64     `json:"tokens_out"`
	CostUSD    float64   `json:"cost_usd"`
	LatencyMS  int64     `json:"latency_ms"`
	Success    bool      `json:"success"`
	ErrorCode  *string   `json:"error_code,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type RequestAttemptEventInput struct {
	SchemaVersion int                 `json:"schema_version"`
	Source        string              `json:"source"`
	EventID       string              `json:"event_id"`
	OccurredAt    time.Time           `json:"occurred_at"`
	Account       AccountInput        `json:"account"`
	Conversation  ConversationInput   `json:"conversation"`
	Attempt       RequestAttemptInput `json:"attempt"`
}

type IngestEventRecord struct {
	EventKey
	SchemaVersion int       `json:"schema_version"`
	Status        string    `json:"status"`
	Payload       []byte    `json:"-"`
	LastError     string    `json:"last_error,omitempty"`
	AttemptCount  int       `json:"attempt_count"`
	FirstSeenAt   time.Time `json:"first_seen_at"`
	LastAttemptAt time.Time `json:"last_attempt_at"`
	NextRetryAt   time.Time `json:"next_retry_at"`
}
