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
## Requirements

| Name | Version |
|------|---------|
| <a name="requirement_terraform"></a> [terraform](#requirement\_terraform) | >= 1.5.0 |
| <a name="requirement_google"></a> [google](#requirement\_google) | 7.18.0 |

## Providers

| Name | Version |
|------|---------|
| <a name="provider_google"></a> [google](#provider\_google) | 7.18.0 |

## Modules

No modules.

## Resources

| Name | Type |
|------|------|
| [google_compute_address.openclaw_internal_ip](https://registry.terraform.io/providers/hashicorp/google/7.18.0/docs/resources/compute_address) | resource |
| [google_compute_address.vpn_internal_ip](https://registry.terraform.io/providers/hashicorp/google/7.18.0/docs/resources/compute_address) | resource |
| [google_compute_address.vpn_ip](https://registry.terraform.io/providers/hashicorp/google/7.18.0/docs/resources/compute_address) | resource |
| [google_compute_firewall.openclaw_gateway](https://registry.terraform.io/providers/hashicorp/google/7.18.0/docs/resources/compute_firewall) | resource |
| [google_compute_firewall.openclaw_ssh](https://registry.terraform.io/providers/hashicorp/google/7.18.0/docs/resources/compute_firewall) | resource |
| [google_compute_firewall.ssh](https://registry.terraform.io/providers/hashicorp/google/7.18.0/docs/resources/compute_firewall) | resource |
| [google_compute_firewall.wgeasy_ui](https://registry.terraform.io/providers/hashicorp/google/7.18.0/docs/resources/compute_firewall) | resource |
| [google_compute_firewall.wireguard](https://registry.terraform.io/providers/hashicorp/google/7.18.0/docs/resources/compute_firewall) | resource |
| [google_compute_instance.openclaw](https://registry.terraform.io/providers/hashicorp/google/7.18.0/docs/resources/compute_instance) | resource |
| [google_compute_instance.vpn](https://registry.terraform.io/providers/hashicorp/google/7.18.0/docs/resources/compute_instance) | resource |
| [google_compute_project_metadata_item.oslogin](https://registry.terraform.io/providers/hashicorp/google/7.18.0/docs/resources/compute_project_metadata_item) | resource |
| [google_compute_router.openclaw_router](https://registry.terraform.io/providers/hashicorp/google/7.18.0/docs/resources/compute_router) | resource |
| [google_compute_router_nat.openclaw_nat](https://registry.terraform.io/providers/hashicorp/google/7.18.0/docs/resources/compute_router_nat) | resource |
| [google_compute_network.default](https://registry.terraform.io/providers/hashicorp/google/7.18.0/docs/data-sources/compute_network) | data source |
| [google_compute_subnetwork.default](https://registry.terraform.io/providers/hashicorp/google/7.18.0/docs/data-sources/compute_subnetwork) | data source |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_enable_project_oslogin"></a> [enable\_project\_oslogin](#input\_enable\_project\_oslogin) | Optionally enable OS Login at the project level (single metadata item). | `bool` | `false` | no |
| <a name="input_instance_name"></a> [instance\_name](#input\_instance\_name) | Compute Engine instance name. | `string` | n/a | yes |
| <a name="input_machine_type"></a> [machine\_type](#input\_machine\_type) | GCE machine type (e.g., e2-micro). | `string` | n/a | yes |
| <a name="input_openclaw_anthropic_api_key"></a> [openclaw\_anthropic\_api\_key](#input\_openclaw\_anthropic\_api\_key) | Anthropic API key for OpenClaw (optional; set via TF\_VAR\_openclaw\_anthropic\_api\_key). | `string` | `""` | no |
| <a name="input_openclaw_enable_public_ip"></a> [openclaw\_enable\_public\_ip](#input\_openclaw\_enable\_public\_ip) | Attach a public IP to the OpenClaw VM (not recommended). | `bool` | `false` | no |
| <a name="input_openclaw_gateway_password"></a> [openclaw\_gateway\_password](#input\_openclaw\_gateway\_password) | Gateway password for OpenClaw (required). | `string` | n/a | yes |
| <a name="input_openclaw_gateway_port"></a> [openclaw\_gateway\_port](#input\_openclaw\_gateway\_port) | OpenClaw gateway port (TCP). | `number` | `18789` | no |
| <a name="input_openclaw_instance_name"></a> [openclaw\_instance\_name](#input\_openclaw\_instance\_name) | OpenClaw instance name. | `string` | `"openclaw"` | no |
| <a name="input_openclaw_internal_ip_address"></a> [openclaw\_internal\_ip\_address](#input\_openclaw\_internal\_ip\_address) | Optional fixed internal IP for the OpenClaw VM. If null/empty, a reserved internal IP is auto-allocated. | `string` | `null` | no |
| <a name="input_openclaw_machine_type"></a> [openclaw\_machine\_type](#input\_openclaw\_machine\_type) | OpenClaw machine type. | `string` | `"e2-micro"` | no |
| <a name="input_openclaw_model_fallbacks"></a> [openclaw\_model\_fallbacks](#input\_openclaw\_model\_fallbacks) | Fallback OpenClaw models (provider/model), tried in order. | `list(string)` | <pre>[<br/>  "anthropic/claude-opus-4-5"<br/>]</pre> | no |
| <a name="input_openclaw_model_primary"></a> [openclaw\_model\_primary](#input\_openclaw\_model\_primary) | Primary OpenClaw model (provider/model). | `string` | `"anthropic/claude-opus-4-6"` | no |
| <a name="input_openclaw_telegram_bot_token"></a> [openclaw\_telegram\_bot\_token](#input\_openclaw\_telegram\_bot\_token) | Telegram bot token for OpenClaw (optional). | `string` | `null` | no |
| <a name="input_openclaw_version"></a> [openclaw\_version](#input\_openclaw\_version) | OpenClaw CLI version to install (pinned for security). | `string` | `"2026.1.30"` | no |
| <a name="input_openclaw_zone"></a> [openclaw\_zone](#input\_openclaw\_zone) | OpenClaw zone (defaults to var.zone if null/empty). | `string` | `null` | no |
| <a name="input_project_id"></a> [project\_id](#input\_project\_id) | GCP project ID. | `string` | n/a | yes |
| <a name="input_region"></a> [region](#input\_region) | GCP region for the VM and the static IP. | `string` | n/a | yes |
| <a name="input_ssh_source_ranges"></a> [ssh\_source\_ranges](#input\_ssh\_source\_ranges) | Allowed CIDR ranges for SSH (TCP 22). Use your public IP/32. | `list(string)` | <pre>[<br/>  "0.0.0.0/32"<br/>]</pre> | no |
| <a name="input_ui_source_ranges"></a> [ui\_source\_ranges](#input\_ui\_source\_ranges) | Allowed CIDR ranges for wg-easy UI (TCP 51821). Keep this very restricted. | `list(string)` | <pre>[<br/>  "0.0.0.0/32"<br/>]</pre> | no |
| <a name="input_vpn_internal_ip_address"></a> [vpn\_internal\_ip\_address](#input\_vpn\_internal\_ip\_address) | Optional fixed internal IP for the VPN VM. If null/empty, a reserved internal IP is auto-allocated. | `string` | `null` | no |
| <a name="input_wg_default_dns"></a> [wg\_default\_dns](#input\_wg\_default\_dns) | Default DNS for WireGuard clients (e.g., 1.1.1.1 or 1.1.1.1,8.8.8.8). | `string` | n/a | yes |
| <a name="input_wg_host"></a> [wg\_host](#input\_wg\_host) | Optional public hostname or IP for WG\_HOST. If null/empty, the static IP is used. | `string` | `null` | no |
| <a name="input_wg_port"></a> [wg\_port](#input\_wg\_port) | WireGuard UDP port. | `number` | `51820` | no |
| <a name="input_wgeasy_password"></a> [wgeasy\_password](#input\_wgeasy\_password) | Plaintext admin password. On boot, a bcrypt hash is generated (wgpw) and stored as PASSWORD\_HASH. Set exactly one of wgeasy\_password or wgeasy\_password\_hash. | `string` | `null` | no |
| <a name="input_wgeasy_password_hash"></a> [wgeasy\_password\_hash](#input\_wgeasy\_password\_hash) | bcrypt password hash for wg-easy (PASSWORD\_HASH). Recommended if you want to avoid plaintext on the VM. Set exactly one of wgeasy\_password or wgeasy\_password\_hash. | `string` | `null` | no |
| <a name="input_wgeasy_ui_port"></a> [wgeasy\_ui\_port](#input\_wgeasy\_ui\_port) | wg-easy UI TCP port. | `number` | `51821` | no |
| <a name="input_zone"></a> [zone](#input\_zone) | GCP zone for the VM. | `string` | n/a | yes |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_openclaw_gateway_url"></a> [openclaw\_gateway\_url](#output\_openclaw\_gateway\_url) | OpenClaw gateway URL (VPN-only). |
| <a name="output_openclaw_internal_ip"></a> [openclaw\_internal\_ip](#output\_openclaw\_internal\_ip) | Internal IP address of the OpenClaw VM. |
| <a name="output_vpn_internal_ip"></a> [vpn\_internal\_ip](#output\_vpn\_internal\_ip) | Internal IP address of the VPN VM. |
| <a name="output_vpn_public_ip"></a> [vpn\_public\_ip](#output\_vpn\_public\_ip) | Static external IP for the VPN server. |
| <a name="output_wgeasy_ui_url"></a> [wgeasy\_ui\_url](#output\_wgeasy\_ui\_url) | wg-easy UI URL (HTTP). |
| <a name="output_wireguard_port"></a> [wireguard\_port](#output\_wireguard\_port) | WireGuard UDP port. |
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
