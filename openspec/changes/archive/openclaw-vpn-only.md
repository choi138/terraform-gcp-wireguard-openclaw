---
schema: openspec.changes/v1
title: OpenClaw 서버 추가 (VPN 전용 접근)
date: 2026-02-10
owner: user
---

## 목표
- GCP에 OpenClaw 전용 VM 1대를 추가 생성한다.
- OpenClaw는 부팅 시 자동 설치/실행되며, **기존 WireGuard VPN을 통해서만 접근** 가능해야 한다.
- Telegram 봇 연결과 휴대폰(웹/컨트롤 UI) 연결이 가능하도록 기본 설정을 제공한다.

## 핵심 요구사항
1) 리소스
- Compute Engine VM 1대 (Ubuntu LTS)
- 방화벽 규칙 2개
  - OpenClaw 게이트웨이 포트(기본 18789/TCP) 인바운드 허용
  - SSH 22/TCP 인바운드 허용
- 위 방화벽은 **source_tags = ["wg-vpn"]** 만 허용하고, openclaw VM은 전용 tag(예: `openclaw`)로 타겟팅
- **VPN을 통해서만 접근**: openclaw VM에는 외부 IP를 붙이지 않고, 내부 IP만 사용
- 외부 API(Telegram/LLM) 통신을 위한 **Cloud NAT** 구성
- openclaw VM과 기존 WireGuard(VPS) VM은 **같은 VPC(동일 네트워크/서브넷)** 를 사용해야 함

2) OpenClaw 자동 설치/실행
- VM 부팅 시 OpenClaw 설치 및 gateway 서비스 실행
- config 파일에 `gateway.mode=local`, `gateway.bind=lan`, `gateway.port` 설정
- 인증 방식: `gateway.auth.mode=password` + `OPENCLAW_GATEWAY_PASSWORD` 환경변수 사용
- Telegram 봇 토큰: env `TELEGRAM_BOT_TOKEN` 또는 config `channels.telegram.botToken`
- 기본 DM 정책은 `pairing`

3) 출력(Outputs)
- `openclaw_internal_ip`
- `openclaw_gateway_url` (http://<internal_ip>:18789)

4) 보안
- OpenClaw 게이트웨이 포트는 외부 공개 금지 (VPN 내부에서만 접근)
- SSH도 VPN 내부에서만 접근
- README에 “VPN 접속 후에만 내부 IP로 접근 가능”을 명시

## 구현 변경 사항 (Terraform)
### 파일 변경
- **추가**: `startup-openclaw.sh.tpl`
- **수정**: `main.tf`, `variables.tf`, `outputs.tf`, `README.md`, `examples/terraform.tfvars.example`

### main.tf 변경
- locals에 `openclaw_tag = "openclaw"` 추가
- `google_compute_instance` 추가 (openclaw)
  - zone: `var.openclaw_zone` (기본 `var.zone`)
  - machine_type: `var.openclaw_machine_type`
  - tags: `[local.openclaw_tag]`
  - **외부 IP 제거** (network_interface에서 access_config 제거)
  - metadata_startup_script: `templatefile("${path.module}/startup-openclaw.sh.tpl", {...})`
- 방화벽 규칙 2개 추가
  - `openclaw-gateway`: tcp `var.openclaw_gateway_port`, source_tags `[local.vpn_tag]`, target_tags `[local.openclaw_tag]`
  - `openclaw-ssh`: tcp 22, source_tags `[local.vpn_tag]`, target_tags `[local.openclaw_tag]`
- Cloud NAT 추가
  - `google_compute_router` + `google_compute_router_nat`
  - `source_subnetwork_ip_ranges_to_nat = "ALL_SUBNETWORKS_ALL_IP_RANGES"`

### variables.tf 추가
- `openclaw_instance_name` (string, default: "openclaw")
- `openclaw_machine_type` (string, default: "e2-micro")
- `openclaw_zone` (string, default: var.zone)
- `openclaw_gateway_port` (number, default: 18789)
- `openclaw_gateway_password` (string, sensitive, **required**)  
- `openclaw_telegram_bot_token` (string, sensitive, optional)
- (선택) `openclaw_enable_public_ip` (bool, default: false)

### outputs.tf 추가
- `openclaw_internal_ip`
- `openclaw_gateway_url`

## startup-openclaw.sh.tpl (요약 설계)
- Node.js 22 설치 (NodeSource)
- openclaw CLI 설치: `npm install -g openclaw@latest`
- 전용 사용자 `openclaw` 생성
- config 파일 생성: `~openclaw/.openclaw/openclaw.json`
  - gateway.mode = "local"
  - gateway.bind = "lan"
  - gateway.port = ${openclaw_gateway_port}
  - gateway.auth.mode = "password"
  - channels.telegram.enabled = true (token 제공 시)
  - channels.telegram.botToken = ${openclaw_telegram_bot_token} (또는 env `TELEGRAM_BOT_TOKEN` 사용)
  - channels.telegram.dmPolicy = "pairing"
- systemd 서비스 생성: `openclaw.service`
  - Environment: `OPENCLAW_CONFIG_PATH`, `OPENCLAW_STATE_DIR`, `OPENCLAW_GATEWAY_PASSWORD`, `TELEGRAM_BOT_TOKEN`
  - ExecStart: `/usr/local/bin/openclaw gateway`

## README 업데이트
- OpenClaw 접속 방법: VPN 연결 후 내부 IP로 접속
- Telegram 연결 방법: BotFather 토큰 생성 → `openclaw_telegram_bot_token` 설정 → apply
- DM pairing 안내 (최초 DM에서 코드 승인)
- split tunnel 사용 시 VPC 내부 대역을 AllowedIPs에 추가해야 함

## 검증 체크리스트
- `terraform apply` 후 openclaw VM 생성됨
- VPN 연결된 상태에서만 `http://<openclaw_internal_ip>:18789` 접근 가능
- Telegram 봇 토큰 설정 시 DM pairing 동작 확인
