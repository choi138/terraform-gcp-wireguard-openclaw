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

### Requirement: Secret Manager references for sensitive bootstrap inputs
The system MUST support Secret Manager reference inputs for sensitive values used by wg-easy and OpenClaw bootstrap (admin credentials, gateway password, and optional API/bot tokens).

#### Scenario: Secret references are provided
- **WHEN** a user supplies supported Secret Manager reference variables instead of plaintext variables
- **THEN** Terraform plans/applies using only references and not raw secret payload values

### Requirement: Runtime secret resolution through instance identity
The system MUST resolve Secret Manager-backed values at VM startup using the VM service account identity before starting wg-easy or OpenClaw services.

#### Scenario: Required secret can be resolved
- **WHEN** startup script fetches a required secret successfully from Secret Manager
- **THEN** the target service starts using the retrieved value without logging it or persisting it in plaintext to stdout, stderr, serial console output, or environment files

#### Scenario: Required secret cannot be resolved
- **WHEN** the required secret reference is invalid or access is denied
- **THEN** startup exits with a non-zero status and logs a sanitized actionable error without exposing secret values, names, full resource paths, or version identifiers

### Requirement: Least-privilege secret access
The system MUST grant secret access permissions only to the specific secrets required by each VM.

#### Scenario: VM reads an authorized secret
- **WHEN** a VM service account requests a configured secret that it is explicitly bound to
- **THEN** Secret Manager access is granted

#### Scenario: VM reads an unauthorized secret
- **WHEN** a VM service account requests a secret outside its configured bindings
- **THEN** Secret Manager access is denied

### Requirement: Secret Manager references are the supported source for sensitive bootstrap inputs
The system MUST use Secret Manager reference variables for supported sensitive bootstrap inputs and does not need plaintext fallback variables in this module contract.

#### Scenario: Secret reference configuration is used
- **WHEN** users provide the supported Secret Manager reference variables for sensitive bootstrap inputs
- **THEN** the module provisions successfully without requiring separate plaintext credential variables

### Requirement: Mutually exclusive secret reference alternatives
The system MUST enforce mutually exclusive source rules for credentials that have multiple Secret Manager-backed variants.

#### Scenario: Both secret-backed alternatives are set for one credential
- **WHEN** two mutually exclusive Secret Manager-backed variants are configured for the same credential
- **THEN** Terraform validation fails with a clear error message
