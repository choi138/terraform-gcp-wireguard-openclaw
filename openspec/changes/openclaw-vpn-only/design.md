## Context

We already run a WireGuard VM in the default VPC. We need a second VM to host OpenClaw and ensure it is reachable only over the VPN. The OpenClaw host still needs outbound internet access for Telegram/LLM APIs, but must not have a public IP.

## Goals / Non-Goals

**Goals:**
- Provision an OpenClaw VM with no external IP.
- Allow access only from the WireGuard VM via VPC tags.
- Provide outbound internet access via Cloud NAT.
- Automate OpenClaw installation and service startup on boot.
- Surface internal IP and gateway URL as Terraform outputs.

**Non-Goals:**
- Publicly exposing OpenClaw or SSH.
- Building a custom VPC (remain on current default VPC).
- Managing OpenClaw data backups or multi-instance scaling.

## Decisions

- **Same VPC/Subnet**: Keep OpenClaw on the same default VPC/subnet as the WireGuard VM to simplify routing and tag-based firewall rules.
- **No external IP**: Remove `access_config` from the OpenClaw network interface to enforce VPN-only reachability.
- **Tag-based firewall**: Allow gateway/SSH only from `source_tags = ["wg-vpn"]`, target the OpenClaw VM tag (e.g., `openclaw`). This avoids fragile CIDR allowlists.
- **Cloud NAT**: Provide outbound connectivity without assigning a public IP. Chosen over a public IP to preserve the VPN-only access requirement.
- **Boot-time install**: Use a startup script to install Node.js and OpenClaw and register a systemd service, keeping infra reproducible.
- **Secrets via env**: Use `OPENCLAW_GATEWAY_PASSWORD` and optional `TELEGRAM_BOT_TOKEN` from instance metadata to avoid embedding credentials in the config file.

## Risks / Trade-offs

- **[Risk]** Cloud NAT misconfiguration blocks outbound API access → **Mitigation**: Use router + NAT with `ALL_SUBNETWORKS_ALL_IP_RANGES`, verify with `curl` in VM.
- **[Risk]** OpenClaw install via npm may fail or change upstream → **Mitigation**: Pin major version if needed and log startup-script output for diagnosis.
- **[Risk]** VPN dependency for access means OpenClaw unavailable if WireGuard is down → **Mitigation**: Treat VPN as a critical dependency; monitor or keep it minimal.
- **[Trade-off]** No public IP prevents direct SSH from the internet → **Mitigation**: Use VPN-only SSH or temporary IAP if needed.

## Migration Plan

1) Apply Terraform changes to create the OpenClaw VM, firewall rules, and Cloud NAT.
2) Verify OpenClaw service and gateway reachability from a VPN-connected client.
3) Configure Telegram bot token and test DM pairing.
4) Rollback by `terraform destroy` of OpenClaw resources (keeps WireGuard intact).

## Open Questions

- Confirm the exact OpenClaw gateway port and CLI command (`openclaw gateway`) for the intended version.
- Decide whether to pin a specific OpenClaw npm version for stability.
