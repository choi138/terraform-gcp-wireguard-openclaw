## 1. Terraform Core Resources

- [x] 1.1 Add OpenClaw VM resource (internal-only) with startup script
- [x] 1.2 Add firewall rules for OpenClaw gateway and SSH (source tag wg-vpn)
- [x] 1.3 Add Cloud Router + NAT for OpenClaw outbound access

## 2. Configuration & Outputs

- [x] 2.1 Add OpenClaw variables (instance name, ports, secrets, tokens)
- [x] 2.2 Add outputs for OpenClaw internal IP and gateway URL
- [x] 2.3 Add startup-openclaw.sh.tpl with OpenClaw install/config/systemd

## 3. Docs & Examples

- [x] 3.1 Update README with VPN-only access and Telegram pairing steps
- [x] 3.2 Update examples/terraform.tfvars.example with OpenClaw inputs
