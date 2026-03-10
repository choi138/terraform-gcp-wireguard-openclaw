package domain

import "time"

type DashboardSummary struct {
	RequestsTotal  int64   `json:"requests_total"`
	TokensTotal    int64   `json:"tokens_total"`
	CostUSD        float64 `json:"cost_usd"`
	ErrorRate      float64 `json:"error_rate"`
	ActiveAccounts int64   `json:"active_accounts"`
}

type DashboardPoint struct {
	BucketStart time.Time `json:"bucket_start"`
	Value       float64   `json:"value"`
}

type Conversation struct {
	ID        int64      `json:"id"`
	AccountID int64      `json:"account_id"`
	Channel   string     `json:"channel"`
	Status    string     `json:"status"`
	StartedAt time.Time  `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
}

type Message struct {
	ID             int64     `json:"id"`
	ConversationID int64     `json:"conversation_id"`
	Role           string    `json:"role"`
	ContentMasked  string    `json:"content_masked"`
	CreatedAt      time.Time `json:"created_at"`
}

type RequestAttempt struct {
	ID             int64     `json:"id"`
	ConversationID int64     `json:"conversation_id"`
	Provider       string    `json:"provider"`
	Model          string    `json:"model"`
	TokensIn       int64     `json:"tokens_in"`
	TokensOut      int64     `json:"tokens_out"`
	CostUSD        float64   `json:"cost_usd"`
	LatencyMS      int64     `json:"latency_ms"`
	Success        bool      `json:"success"`
	ErrorCode      *string   `json:"error_code,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

type InfraSnapshot struct {
	ID           int64     `json:"id"`
	VPNPeerCount int       `json:"vpn_peer_count"`
	OpenClawUp   bool      `json:"openclaw_up"`
	CPUPct       float64   `json:"cpu_pct"`
	MemPct       float64   `json:"mem_pct"`
	CapturedAt   time.Time `json:"captured_at"`
}

type AuditEvent struct {
	Actor        string         `json:"actor"`
	Action       string         `json:"action"`
	ResourceType string         `json:"resource_type"`
	ResourceID   string         `json:"resource_id,omitempty"`
	Metadata     map[string]any `json:"metadata_json,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
}

type Pagination struct {
	Page     int
	PageSize int
}

type ConversationFilter struct {
	Channel string
	Status  string
}
