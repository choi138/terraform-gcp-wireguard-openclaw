## Context

`terraform-gcp-wireguard-openclaw` currently establishes VPN-first infrastructure (WireGuard + private OpenClaw) but has no service layer that exposes operational data. The backend plan requires an API that stays private, aggregates operational metrics, and provides traceable read access for an Ops Console.

## Goals / Non-Goals

**Goals:**
- Introduce a Go API service that runs inside the private network boundary.
- Define a stable persistence model for conversation, request-attempt, infra, and audit datasets.
- Deliver MVP read endpoints required by the dashboard and timeline UI.
- Enforce consistent authentication and audit logging on protected reads.

**Non-Goals:**
- Browser-triggered infrastructure mutation (for example Terraform apply from UI).
- Multi-tenant role modeling beyond a single admin scope for MVP.
- Full SIEM or long-term compliance retention in this phase.

## Decisions

- **Decision: Build a dedicated backend module in Go (`chi`, `pgx`, `sqlc`, `golang-migrate`).**
  - Rationale: matches backend plan and keeps API logic isolated from Terraform/frontend concerns.
  - Alternative considered: embedding API process inside existing OpenClaw runtime shell scripts (rejected for maintainability and testability).

- **Decision: Start with PostgreSQL as the primary source of truth for read models.**
  - Rationale: dashboard, timeline, and infra queries require relational filtering and pagination.
  - Alternative considered: file-based snapshots or in-memory-only aggregation (rejected due to weak queryability and recovery).

- **Decision: Keep API private-only and require bearer token for non-health endpoints.**
  - Rationale: aligns with VPN-only access principle and provides immediate request attribution.
  - Alternative considered: unauthenticated VPN trust only (rejected because auditing and blast-radius control are weaker).

- **Decision: Add an explicit audit event write on authenticated read operations.**
  - Rationale: supports operator accountability and downstream security reviews.
  - Alternative considered: rely only on transport logs (rejected because logs are harder to query for resource-level actions).

## Risks / Trade-offs

- **[Risk]** Initial schema may underfit future workload (for example high-cardinality attempt metadata).  
  **Mitigation:** keep append-friendly tables and plan additive migrations.

- **[Risk]** Token-only auth can become operationally brittle if secret rotation is unmanaged.  
  **Mitigation:** integrate token source with Secret Manager and document rotation runbook.

- **[Trade-off]** Delivering multiple read surfaces in one phase increases scope.  
  **Mitigation:** prioritize the exact endpoint set listed in MVP and defer advanced filtering.

## Migration Plan

1. Scaffold `backend/` structure and baseline CI targets for build/test/lint.
2. Create initial Postgres migrations and generated query layer.
3. Implement health endpoints first, then dashboard/conversation/infra read endpoints.
4. Wire token middleware and audit event insertion.
5. Deploy privately behind VPN path and validate frontend reads against the new endpoints.

## Open Questions

- Should the MVP deployment target be OpenClaw co-location first, or a dedicated private backend VM immediately?
- How long should `request_attempts` raw records be retained before aggregation-only mode?
- Which dashboard metrics are strict launch blockers versus post-MVP additions?
