## Why

We need a second server to run OpenClaw that is reachable only through the existing WireGuard VPN. This enables secure mobile/Telegram control without exposing the service publicly.

## What Changes

- Add a new OpenClaw VM on GCP with no public IP.
- Restrict OpenClaw gateway and SSH access to traffic coming from the VPN server (source tags).
- Ensure both VMs share the same VPC/subnet.
- Add Cloud NAT so the OpenClaw VM can reach external APIs (Telegram/LLM).
- Add startup automation to install and run OpenClaw with required config and secrets.
- Add outputs for OpenClaw internal IP and gateway URL.

## Capabilities

### New Capabilities
- `openclaw-vpn-only`: Provision and operate an OpenClaw gateway that is only reachable over the WireGuard VPN.

### Modified Capabilities
- (none)

## Impact

- Terraform: new VM, firewall rules, Cloud NAT, startup script, variables, outputs, README updates.
- Networking: new internal-only service that depends on the existing VPN path for access.
- Secrets: new OpenClaw gateway password and optional Telegram bot token.
