# terraform-gcp-wireguard-openclaw

Repository for running a private GCP WireGuard + OpenClaw stack and the internal
Ops API that supports it.

This repo now contains two main parts:
- `infra/`: Terraform for the VPN and OpenClaw infrastructure
- `apps/backend/`: Go backend for internal Ops Console APIs

The deployed OpenClaw VM is intended to stay private behind the WireGuard VPN.
The Ops API is also designed for internal-only access.

## Components
- `infra/`: GCP networking, IAM, VM instances, startup scripts, example module usage
- `apps/backend/`: Go API server, migrations, repository adapters, OpenAPI spec
- `openspec/`: change history and synced specs for the Ops API work
- `docs/`: architecture, security, and operational notes
- `tests/`: infrastructure validation guide

## Quick Start

### Infrastructure
1. Move to the example directory:
```bash
cd infra/examples/basic
```

2. Copy the example tfvars and edit for your environment:
```bash
cp terraform.tfvars.example terraform.tfvars
```

3. Initialize and apply:
```bash
terraform init
terraform plan
terraform apply
```

4. Check outputs:
```bash
terraform output wgeasy_ui_url
terraform output openclaw_gateway_url
```

### Backend
1. Create a local env file:
```bash
cd apps/backend
cp .env.example .env
```

2. Load env values:
```bash
set -a
source .env
set +a
```

3. Run migrations and start the API:
```bash
make migrate
make run-api
```

4. Run tests:
```bash
make test
```

Use `OPS_API_ALLOW_MEMORY_FALLBACK=true` only for local smoke tests. Normal
environments should set `OPS_API_DB_DSN` and keep memory fallback disabled.

## Repository Layout
- `infra/main.tf`: resources, networking, IAM, instances, startup scripts
- `infra/variables.tf`: Terraform inputs and validation
- `infra/outputs.tf`: public/internal connection information
- `infra/templates/startup.sh.tpl`: wg-easy bootstrap script
- `infra/templates/startup-openclaw.sh.tpl`: OpenClaw bootstrap script
- `infra/examples/basic/`: example module usage
- `apps/backend/cmd/api`: HTTP server entrypoint
- `apps/backend/cmd/migrate`: SQL migration runner
- `apps/backend/internal/http`: routes, handlers, middleware
- `apps/backend/internal/repository`: memory and Postgres repository implementations
- `apps/backend/api/openapi.yaml`: Ops API contract
- `apps/backend/README.md`: backend-specific runtime and command details
- `openspec/specs/ops-api-core/spec.md`: synced core backend read-model spec
- `openspec/specs/ops-api-ingestion/spec.md`: synced ingest pipeline spec
- `openspec/specs/ops-api-security-analysis/spec.md`: synced tfvars security analysis spec

## Development And CI
- Install repo hooks:
```bash
python3 -m pip install pre-commit
pre-commit install
pre-commit install --hook-type commit-msg
```

- Run repository checks:
```bash
pre-commit run --all-files
```

- Backend CI lives in `.github/workflows/backend-ci.yml`.
  It runs `go test ./...` only when `apps/backend/**` or the workflow file changes.

## Infrastructure Summary

### Resources created
- 1 WireGuard VM (Ubuntu LTS)
- 1 OpenClaw VM (Ubuntu LTS, internal IP only by default)
- 2 dedicated service accounts (VPN/OpenClaw)
- Secret Manager IAM bindings scoped to referenced secrets only
- 1 static external IP for WireGuard
- firewall rules for WireGuard, wg-easy UI, SSH, and the private OpenClaw gateway
- Cloud Router + Cloud NAT for OpenClaw outbound access
- optional project-level OS Login enablement

### Important outputs
- `vpn_public_ip`
- `wgeasy_ui_url`
- `wireguard_port`
- `vpn_internal_ip`
- `openclaw_internal_ip`
- `openclaw_gateway_url`

## Backend Summary
- Runtime: Go `1.26.0`
- Operator routes support legacy admin token mode or OIDC roles (`viewer`, `auditor`, `admin`)
- Internal ingest writes use a distinct `OPS_API_INGEST_TOKEN` and support conversation, infra snapshot, and request-attempt ingestion
- Tfvars security analysis is available through `POST /v1/security/analyze-tfvars` and `GET /v1/security/findings`
- Database-backed mode uses `OPS_API_DB_DSN`
- Read APIs include dashboard, conversations, messages, attempts, and infra snapshots
- Authenticated reads are audited
- Protected `/metrics`, structured request logging, trace emission, break-glass fallback, and retention worker support are available
- OpenAPI contract lives in `apps/backend/api/openapi.yaml`

See `apps/backend/README.md` for backend-only commands and environment details, and `apps/backend/docs/operations.md` for the operational runbook.

## Required Terraform Variables
- `project_id`, `region`, `zone`
- `instance_name`, `machine_type`
- `vpn_internal_ip_address` (optional)
- `ssh_source_ranges`
- `ui_source_ranges`
- `wg_default_dns`
- `wg_port`
- `wgeasy_ui_port`
- exactly one of:
  - `wgeasy_password_secret`
  - `wgeasy_password_hash_secret`
- `openclaw_instance_name`, `openclaw_machine_type`
- `openclaw_internal_ip_address` (optional)
- `openclaw_gateway_port`
- `openclaw_gateway_password_secret`
- `openclaw_version`
- optional model and integration secrets such as:
  - `openclaw_anthropic_api_key_secret`
  - `openclaw_openai_api_key_secret`
  - `openclaw_telegram_bot_token_secret`

## Secret Manager Setup
Secret references support:
- `projects/<project>/secrets/<name>`
- `projects/<project>/secrets/<name>/versions/<version>`

Example:
```bash
gcloud services enable secretmanager.googleapis.com --project <PROJECT_ID>

printf '%s' 'YOUR_VALUE' | gcloud secrets create openclaw-gateway-password \
  --project <PROJECT_ID> \
  --replication-policy=automatic \
  --data-file=-
```

The module grants `roles/secretmanager.secretAccessor` only to secrets that are
explicitly referenced by variables.

## Security Notes
- Do not expose the wg-easy UI publicly. Restrict `ui_source_ranges`.
- Restrict SSH with `ssh_source_ranges` and prefer OS Login.
- Keep OpenClaw private unless there is a deliberate exception.
- Store secret payloads in Secret Manager, not in tfvars or state-adjacent files.
- Treat `OPS_API_ADMIN_TOKEN` as required operational secret material.

## Verification Checklist
- Confirm the public VPN IP after `terraform apply`
- Log in to `wgeasy_ui_url`
- Connect through WireGuard
- Verify `openclaw_gateway_url` is reachable only from inside the VPN
- Verify startup scripts can read configured Secret Manager references
- For the backend, verify:
  - `GET /v1/healthz` returns `200`
  - protected routes return `401` without a token
  - protected routes return `200` with a valid token

## Troubleshooting
- Firewall/tags: verify the VM has the `wg-vpn` tag
- Secret Manager access:
  - verify secret reference format
  - verify Secret Manager API is enabled
  - verify the generated VM service account has accessor permissions
- WireGuard status:
  - `sudo docker ps`
  - `sudo docker logs wg-easy`
- OpenClaw status:
  - `sudo systemctl status openclaw`
  - `sudo journalctl -u openclaw -n 200`
- Startup scripts:
  - `sudo journalctl -u google-startup-scripts.service`
  - `sudo tail -n 200 /var/log/syslog`

## Terraform Docs
<!-- BEGIN_TF_DOCS -->
<!-- END_TF_DOCS -->
