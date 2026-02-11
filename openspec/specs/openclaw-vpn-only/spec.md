## ADDED Requirements

### Requirement: OpenClaw VM is private and in the same VPC
The system MUST provision the OpenClaw VM without a public IP and attach it to the same VPC/subnet as the WireGuard VM.

#### Scenario: Provision private OpenClaw VM
- **WHEN** Terraform applies the OpenClaw resources
- **THEN** the OpenClaw VM has no external IP and uses the same VPC/subnet as the WireGuard VM

### Requirement: VPN-only access to gateway and SSH
The system MUST allow OpenClaw gateway and SSH access only from the WireGuard VM tag and deny all other sources.

#### Scenario: Access from VPN tag
- **WHEN** traffic originates from a VM with tag `wg-vpn`
- **THEN** TCP 18789 and TCP 22 are permitted to the OpenClaw VM

#### Scenario: Access from non-VPN source
- **WHEN** traffic originates from any source without tag `wg-vpn`
- **THEN** TCP 18789 and TCP 22 are not permitted to the OpenClaw VM

### Requirement: Outbound access via Cloud NAT
The system MUST provide outbound internet access for the OpenClaw VM without assigning a public IP.

#### Scenario: External API access
- **WHEN** the OpenClaw VM calls external APIs (e.g., Telegram)
- **THEN** the traffic egresses through Cloud NAT successfully

### Requirement: OpenClaw service bootstraps on startup
The system MUST install OpenClaw and start the gateway service on boot with password authentication enabled.

#### Scenario: Gateway service starts
- **WHEN** the OpenClaw VM boots
- **THEN** the OpenClaw gateway service is running and listening on the configured port

### Requirement: Expose internal connection details
The system MUST output the OpenClaw VM internal IP and gateway URL for VPN users.

#### Scenario: Terraform outputs
- **WHEN** Terraform apply completes
- **THEN** outputs include `openclaw_internal_ip` and `openclaw_gateway_url`
