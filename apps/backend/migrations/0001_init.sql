CREATE TABLE IF NOT EXISTS accounts (
  id BIGSERIAL PRIMARY KEY,
  external_id TEXT NOT NULL UNIQUE,
  email TEXT NOT NULL,
  status TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS conversations (
  id BIGSERIAL PRIMARY KEY,
  account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE NO ACTION,
  channel TEXT NOT NULL,
  status TEXT NOT NULL,
  started_at TIMESTAMPTZ NOT NULL,
  ended_at TIMESTAMPTZ NULL
);

CREATE TABLE IF NOT EXISTS messages (
  id BIGSERIAL PRIMARY KEY,
  conversation_id BIGINT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
  role TEXT NOT NULL,
  content_masked TEXT NOT NULL,
  content_raw_encrypted BYTEA NULL,
  created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS request_attempts (
  id BIGSERIAL PRIMARY KEY,
  conversation_id BIGINT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
  provider TEXT NOT NULL,
  model TEXT NOT NULL,
  tokens_in BIGINT NOT NULL DEFAULT 0,
  tokens_out BIGINT NOT NULL DEFAULT 0,
  cost_usd DOUBLE PRECISION NOT NULL DEFAULT 0,
  latency_ms BIGINT NOT NULL DEFAULT 0,
  success BOOLEAN NOT NULL,
  error_code TEXT NULL,
  created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS infra_snapshots (
  id BIGSERIAL PRIMARY KEY,
  vpn_peer_count INT NOT NULL,
  openclaw_up BOOLEAN NOT NULL,
  cpu_pct DOUBLE PRECISION NOT NULL,
  mem_pct DOUBLE PRECISION NOT NULL,
  captured_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS audit_events (
  id BIGSERIAL PRIMARY KEY,
  actor TEXT NOT NULL,
  action TEXT NOT NULL,
  resource_type TEXT NOT NULL,
  resource_id TEXT NULL,
  metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_messages_conversation_created
  ON messages(conversation_id, created_at);

CREATE INDEX IF NOT EXISTS idx_request_attempts_created_success
  ON request_attempts(created_at, success);

CREATE INDEX IF NOT EXISTS idx_request_attempts_conversation_created
  ON request_attempts(conversation_id, created_at);

CREATE INDEX IF NOT EXISTS idx_conversations_account_started
  ON conversations(account_id, started_at);

CREATE INDEX IF NOT EXISTS idx_infra_snapshots_captured
  ON infra_snapshots(captured_at);

CREATE INDEX IF NOT EXISTS idx_audit_events_created
  ON audit_events(created_at);
