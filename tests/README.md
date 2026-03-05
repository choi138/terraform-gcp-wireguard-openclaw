# Tests

Suggested checks for this module:
- cd infra
- terraform fmt -check
- terraform init -backend=false
- terraform validate
- tflint --init && tflint

## Secret Manager path verification
1) Create test secrets and versions in Secret Manager for:
- wg-easy credential source (`wgeasy_password_secret` or `wgeasy_password_hash_secret`)
- `openclaw_gateway_password_secret`
- optional `openclaw_anthropic_api_key_secret`
- optional `openclaw_telegram_bot_token_secret`

2) Use only `*_secret` variables in `infra/examples/basic/terraform.tfvars`.

3) Run:
- terraform plan
- terraform apply

4) Validate runtime behavior:
- `sudo journalctl -u google-startup-scripts.service -n 200`
- `sudo docker ps` (wg-easy is running)
- `sudo systemctl status openclaw` (OpenClaw service is running)

## Secret-only input validation
1) Run `terraform plan` with neither `wgeasy_password_secret` nor `wgeasy_password_hash_secret` and verify it fails.
2) Run `terraform plan` with both `wgeasy_password_secret` and `wgeasy_password_hash_secret` set and verify it fails.
3) Run `terraform plan` without `openclaw_gateway_password_secret` and verify it fails.
4) Run `terraform plan` with valid secret references and verify it succeeds.

For higher confidence, consider adding Terratest or kitchen-terraform in this folder.
