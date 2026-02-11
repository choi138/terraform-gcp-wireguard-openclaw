# terraform-gcp-wireguard-openclaw

Terraform module that provisions a GCP WireGuard (wg-easy) VPN plus an OpenClaw VM. It uses Ubuntu LTS, runs wg-easy via Docker, and includes a static external IP with least-privilege firewall rules.

## Resources created
- 1 WireGuard VM (Ubuntu LTS)
- 1 OpenClaw VM (Ubuntu LTS, **internal IP only**)
- 1 Static External IP (for WireGuard)
- 5 firewall rules (WireGuard UDP, wg-easy UI, WireGuard SSH, OpenClaw Gateway, OpenClaw SSH)
- Cloud Router + Cloud NAT (for OpenClaw outbound API access)
- OS Login enablement (optional, project-level)

## Layout
- `main.tf` - resources, networking, firewall, instances, startup scripts
- `variables.tf` - inputs and validation
- `outputs.tf` - outputs
- `versions.tf` - Terraform/Provider versions
- `templates/startup.sh.tpl` - install/run Docker + wg-easy at boot
- `templates/startup-openclaw.sh.tpl` - install/run OpenClaw at boot
- `examples/basic/` - module usage example
- `docs/` - design/security/operations docs
- `tests/` - testing/validation guide

## Usage
1) Move to the example directory:
```
cd examples/basic
```

2) Copy the example tfvars and edit for your environment:
```
cp terraform.tfvars.example terraform.tfvars
```

3) Initialize and apply:
```
terraform init
terraform plan
terraform apply
```

4) Check the UI URL:
```
terraform output wgeasy_ui_url
```

## Terraform Docs
<!-- BEGIN_TF_DOCS -->
<!-- END_TF_DOCS -->

## Required variables
- `project_id`, `region`, `zone`
- `instance_name`, `machine_type`
- `vpn_internal_ip_address` (optional; pin WireGuard VM internal IP)
- `ssh_source_ranges` (CIDR list)
- `ui_source_ranges` (CIDR list; keep very restricted)
- `wg_default_dns`
- `wg_port` (default: 51820)
- `wgeasy_ui_port` (default: 51821)
- `wg_host` (optional; defaults to static IP)
- `wgeasy_password` **or** `wgeasy_password_hash` (set exactly one; both sensitive)
- `enable_project_oslogin` (optional; default false)
- `openclaw_instance_name`, `openclaw_machine_type`
- `openclaw_internal_ip_address` (optional; pin OpenClaw VM internal IP)
- `openclaw_gateway_port` (default: 18789)
- `openclaw_gateway_password` (required, sensitive)
- `openclaw_version` (default: `2026.1.30`, recommended to pin a patched version)
- `openclaw_anthropic_api_key` (optional, sensitive; use `TF_VAR_openclaw_anthropic_api_key`)
- `openclaw_model_primary` (default: `anthropic/claude-opus-4-6`)
- `openclaw_model_fallbacks` (default: `["anthropic/claude-opus-4-5"]`)
- `openclaw_telegram_bot_token` (optional, sensitive)
- `openclaw_enable_public_ip` (optional; default false, not recommended)

## Outputs
- `vpn_public_ip`
- `wgeasy_ui_url` (http://<ip>:51821)
- `wireguard_port`
- `vpn_internal_ip`
- `openclaw_internal_ip`
- `openclaw_gateway_url` (VPN-only access)

## Security notes (important)
- Do not expose the wg-easy UI to the public internet. Restrict `ui_source_ranges` to your public IP/32.
- Restrict SSH with `ssh_source_ranges` and use OS Login + SSH keys.
- Use strong admin passwords and rotate immediately if exposed.
- Sensitive values (API keys, passwords) are stored in Terraform state. Do not commit or share state files.

## wg-easy configuration
- If `wgeasy_password` is set, the startup script generates a bcrypt hash via `wgpw` and sets `PASSWORD_HASH`. (`PASSWORD` is not allowed in recent versions.)
- If `wgeasy_password_hash` is set, it is used directly as `PASSWORD_HASH`. This avoids leaving plaintext on the VM.
- The following are always set: `WG_HOST`, `WG_PORT`, `WG_DEFAULT_DNS`, `PORT`.
- The UI is exposed over HTTP with `INSECURE=true`, so restrict access.
- The container image is pinned to `ghcr.io/wg-easy/wg-easy:14` to keep env-based configuration stable. Hash example:
  `docker run --rm ghcr.io/wg-easy/wg-easy:14 wgpw 'YOUR_PASSWORD'`

## OpenClaw usage
- The OpenClaw VM has **no external IP** and is reachable only after VPN connection.
- Connect via `openclaw_gateway_url` after WireGuard is up.
- Model/key setup:
  1) (Recommended) Inject locally via env var: `export TF_VAR_openclaw_anthropic_api_key="..."` then `terraform apply`
  2) Default model is **claude opus 4.6**, with **4.5** as fallback
- Telegram integration:
  1) Create a bot token via BotFather
  2) Set `openclaw_telegram_bot_token` and run `terraform apply`
  3) First DM requires approval under the pairing policy
- OpenClaw and WireGuard VMs share the same VPC/subnet.

## Verification checklist
- Confirm the public IP after `terraform apply`
- Access `http://<vpn_public_ip>:51821` and log in
- Connect via WireGuard client (QR or config)
- Confirm traffic is routed through the VPN
- Access `openclaw_gateway_url` while connected to VPN

## Troubleshooting
- Firewall/tags: the VM must have the `wg-vpn` tag. Check UDP 51820 and TCP 51821.
- Docker status:
  - `sudo docker ps`
  - `sudo docker logs wg-easy`
- Startup script logs:
  - `sudo journalctl -u google-startup-scripts.service`
  - `sudo tail -n 200 /var/log/syslog`
- OpenClaw logs:
  - `sudo systemctl status openclaw`
  - `sudo journalctl -u openclaw -n 200`

## Switching to a custom VPC
This module currently uses the default network/subnetwork via `data.google_compute_network.default` and `data.google_compute_subnetwork.default`. Swap to a custom VPC/subnet if needed.
