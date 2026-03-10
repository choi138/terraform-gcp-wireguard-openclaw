## ADDED Requirements

### Requirement: Ops API network boundary remains private-only
The system MUST expose the Ops API only through private network paths reachable from VPN-connected clients.

#### Scenario: Access from VPN path
- **WHEN** a request is sent from a client inside the approved VPN network
- **THEN** the request can reach the internal Ops API endpoint

#### Scenario: Access from public internet path
- **WHEN** a request is sent from outside the approved VPN network
- **THEN** the request cannot reach the Ops API endpoint

### Requirement: Protected endpoints require bearer authentication
The system MUST require a valid admin bearer token for every endpoint except health and readiness probes.

#### Scenario: Valid token on protected route
- **WHEN** a client sends a request with a valid admin bearer token to a protected endpoint
- **THEN** the API authorizes the request and continues handler execution

#### Scenario: Missing or invalid token on protected route
- **WHEN** a client omits the bearer token or provides an invalid token for a protected endpoint
- **THEN** the API responds with an authentication error and does not run business handlers

### Requirement: Health and readiness probes are available
The system MUST provide health and readiness endpoints for service liveness and dependency readiness checks.

#### Scenario: Liveness check
- **WHEN** `/v1/healthz` is requested
- **THEN** the API returns a success response when the process is running

#### Scenario: Readiness check
- **WHEN** `/v1/readyz` is requested
- **THEN** the API returns ready only when required dependencies (for example database connectivity) are available

### Requirement: Dashboard summary and timeseries APIs are provided
The system MUST provide dashboard summary and timeseries endpoints with validated `from`, `to`, and bucket inputs.

#### Scenario: Summary query with valid range
- **WHEN** a client requests `/v1/dashboard/summary` with a valid time range
- **THEN** the API returns aggregate requests, token usage, cost, error rate, and active account fields

#### Scenario: Timeseries query with invalid bucket
- **WHEN** a client requests `/v1/dashboard/timeseries` with an unsupported bucket value
- **THEN** the API rejects the request with a validation error

### Requirement: Conversation timeline and detail APIs are provided
The system MUST provide conversation list and detail APIs with pagination and associated message/attempt views.

#### Scenario: Paginated conversation list
- **WHEN** a client requests `/v1/conversations` with page and page_size parameters
- **THEN** the API returns a bounded page of conversations and pagination metadata

#### Scenario: Conversation detail with related resources
- **WHEN** a client requests a specific conversation, messages, or attempts resource
- **THEN** the API returns only records associated with the requested conversation identifier

### Requirement: Infra status and snapshot APIs are provided
The system MUST provide current infra status and historical snapshot query endpoints.

#### Scenario: Latest infra status
- **WHEN** a client requests `/v1/infra/status`
- **THEN** the API returns the latest known vpn peer count, service availability, and host utilization fields

#### Scenario: Snapshot range query
- **WHEN** a client requests `/v1/infra/snapshots` with a valid range
- **THEN** the API returns snapshots constrained to the requested interval

### Requirement: Authenticated reads are auditable
The system MUST record an audit event for authenticated read requests to protected Ops API resources.

#### Scenario: Protected read request succeeds
- **WHEN** an authenticated client reads a protected resource
- **THEN** the API writes an audit event containing actor, action, resource type, resource identifier (if present), and timestamp
