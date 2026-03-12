## Context

MVP introduces useful API surfaces, but backend-plan Phase 4 requires operational maturity: standardized telemetry, stronger identity controls, and automated data lifecycle management. These additions reduce security risk and improve incident response speed.

## Goals / Non-Goals

**Goals:**
- Add observable runtime signals for requests, errors, latency, and ingest health.
- Transition from single shared token auth toward OIDC with scoped roles.
- Preserve a controlled emergency access path with strict guardrails.
- Enforce retention windows for raw/sensitive data classes.

**Non-Goals:**
- Full enterprise IAM policy orchestration across all infrastructure components.
- Building a complete custom SIEM or APM platform.
- Replacing all historical logs/metrics pipelines in one release.

## Decisions

- **Decision: Layer auth middleware as OIDC-first with optional break-glass token fallback.**
  - Rationale: enables secure default behavior while retaining recoverability during IdP outages.
  - Alternative considered: immediate token removal with OIDC-only requirement (rejected for operational risk during migration).

- **Decision: Implement endpoint-level role policy mapping in backend configuration.**
  - Rationale: keeps authorization explicit and testable per route group.
  - Alternative considered: implicit role inference based on HTTP method (rejected as too coarse).

- **Decision: Emit Prometheus metrics and OpenTelemetry spans from shared HTTP middleware.**
  - Rationale: provides consistent instrumentation coverage with low per-handler overhead.
  - Alternative considered: handler-by-handler manual instrumentation (rejected due to inconsistency risk).

- **Decision: Implement retention via scheduled backend jobs with dry-run mode.**
  - Rationale: safe rollout and predictable cleanup behavior before destructive execution.
  - Alternative considered: ad hoc SQL/manual cron outside service repo (rejected for traceability gaps).

## Risks / Trade-offs

- **[Risk]** Misconfigured OIDC provider or JWKS can deny legitimate access.  
  **Mitigation:** startup validation, health diagnostics for auth dependencies, and break-glass fallback controls.

- **[Risk]** Excessive telemetry cardinality can raise cost and reduce signal quality.  
  **Mitigation:** enforce label allowlists and sampling defaults.

- **[Trade-off]** Retention cleanup may remove data needed for retrospective analysis.  
  **Mitigation:** make retention windows configurable and log deletion summaries for governance review.

## Migration Plan

1. Add telemetry middleware and verify `/metrics` and trace export in private environments.
2. Integrate OIDC validation and role mapping while keeping break-glass token disabled by default.
3. Run retention jobs in dry-run mode and review deletion candidates.
4. Enable enforced retention and OIDC-first auth in staged rollout.
5. Update runbooks and alerts for operational handoff.

## Open Questions

- Which IdP (Google Workspace OIDC vs alternative) is the first supported provider?
- Should break-glass token be enabled only in non-production or guarded by explicit runtime flag in production?
- What exact retention windows are required per table for compliance versus troubleshooting?
