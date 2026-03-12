## ADDED Requirements

### Requirement: Metrics endpoint and tracing signals are provided
The system MUST expose internal metrics and tracing instrumentation for API request handling and ingest pipeline health.

#### Scenario: Metrics scrape from internal network
- **WHEN** an authorized internal observer scrapes `/metrics`
- **THEN** the endpoint returns current request, latency, error, and ingest lag metrics

#### Scenario: Request trace emission
- **WHEN** a protected API request is processed
- **THEN** the system emits a trace span with request outcome attributes

### Requirement: Structured operational logging is standardized
The system MUST emit structured logs containing `request_id`, `user_id` (or equivalent principal), `path`, `latency_ms`, `status`, and `error_code` fields.

#### Scenario: Successful request log event
- **WHEN** the API handles a successful request
- **THEN** the corresponding log record includes the standardized operational fields

#### Scenario: Failed request log event
- **WHEN** the API handles a failed request
- **THEN** the corresponding log record includes standardized fields and a non-empty error code indicator

### Requirement: OIDC authentication is supported for operator access
The system MUST support OIDC token authentication for Ops API access.

#### Scenario: Valid OIDC token
- **WHEN** a client presents a valid OIDC access token signed by a configured provider
- **THEN** the API authenticates the principal and attaches identity claims to request context

#### Scenario: Invalid OIDC token
- **WHEN** a client presents an expired or invalid OIDC token
- **THEN** the API rejects the request with an authentication error

### Requirement: Role-based authorization is enforced
The system MUST enforce role-based access controls for `admin`, `viewer`, and `auditor` roles on protected endpoints.

#### Scenario: Viewer accesses read-only endpoint
- **WHEN** an authenticated `viewer` calls an endpoint designated as read-only
- **THEN** the API authorizes access

#### Scenario: Viewer accesses admin-only endpoint
- **WHEN** an authenticated `viewer` calls an endpoint designated as admin-only
- **THEN** the API rejects the request with an authorization error

### Requirement: Break-glass token access is controlled and auditable
The system MUST provide an optional break-glass token path that is explicitly configurable and fully auditable.

#### Scenario: Break-glass disabled
- **WHEN** break-glass token mode is not enabled
- **THEN** bearer-token authentication is rejected on routes requiring OIDC authentication

#### Scenario: Break-glass enabled and used
- **WHEN** break-glass token mode is enabled and a valid emergency token is used
- **THEN** the API authorizes according to configured emergency policy and records a dedicated audit event

### Requirement: Data retention policies are automatically enforced
The system MUST run scheduled retention jobs that remove or compact records outside configured retention windows.

#### Scenario: Retention window exceeded
- **WHEN** records age beyond configured retention thresholds
- **THEN** retention jobs mark or delete eligible records according to configured policy

#### Scenario: Dry-run retention execution
- **WHEN** retention jobs run in dry-run mode
- **THEN** the system reports candidate record counts without deleting data
