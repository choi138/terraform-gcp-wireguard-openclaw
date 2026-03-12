# Backend Product Plan: WireGuard + OpenClaw Ops API (Go)

## 1) 목적
- 이 문서는 `terraform-gcp-wireguard-openclaw` 프로젝트의 백엔드 구현 기획서다.
- 목표는 프론트엔드 Ops Console이 필요한 운영 데이터(대화 로그, 상태 지표, 보안 점검 결과)를 안정적으로 제공하는 것이다.
- 구현 언어는 Go를 기본으로 하며, VPN 전용 접근 원칙을 유지한다.

## 2) 프로젝트 컨텍스트
- 현재 인프라는 Terraform으로 다음을 구성한다.
  - WireGuard VM (wg-easy)
  - OpenClaw VM (기본: 외부 IP 없음, VPN 전용 접근)
  - Firewall/NAT/출력값
- 현재 프론트엔드는 정적 UI 중심이며, 실시간 데이터 소스가 없다.
- 사용자 요구:
  - 운영 대시보드 (요청량, 에러율, 비용, 계정 상태)
  - 대화 이력 확인 (누가/언제/무슨 요청, 실패 사유)
  - 보안 리스크 가시화

## 3) 제품 목표 / 비목표

### 목표 (MVP)
- 대시보드용 집계 API 제공
- Conversation Timeline/Detail API 제공
- 인프라/서비스 상태 API 제공
- 보안 규칙 점검 API 제공
- VPN 내부 네트워크에서만 접근 가능하도록 보안 경계 유지

### 비목표 (MVP 범위 밖)
- 브라우저에서 Terraform apply 직접 실행
- 멀티 테넌트 SaaS 권한 모델
- 완전한 SIEM 수준 감사/탐지 시스템

## 4) 아키텍처 개요

```text
               (VPN Connected User)
Browser UI  ------------------------------->  Go API Server
 (Next.js)       HTTPS (private only)            |
                                                   |
                                                   +--> PostgreSQL (state + logs + aggregates)
                                                   |
                                                   +--> Redis (optional, queues/cache)
                                                   |
OpenClaw Service ---- event webhook / SDK ---------+
                                                   |
WireGuard VM ---- metrics agent push -------------+
```

핵심 원칙:
- API는 public internet에 노출하지 않는다.
- 프론트엔드는 데이터 표시/조작만 담당하고, 정책 판단은 백엔드가 담당한다.
- 민감 데이터는 Secret Manager 참조 또는 마스킹 후 저장한다.

## 5) 배포 토폴로지 (권장)

### 옵션 A: MVP (빠른 구축)
- OpenClaw VM 내부에 `ops-api` 서비스(systemd)로 동작
- PostgreSQL은 같은 VM Docker 또는 Cloud SQL Private IP 사용
- 장점: 빠른 시작, 비용 절감
- 단점: 장애 도메인 분리 약함

### 옵션 B: 권장 운영형
- 전용 백엔드 VM(내부 IP only, `ops-api` 태그) + Cloud SQL(Postgres)
- WireGuard/OpenClaw VM은 수집 에이전트만 실행
- 장점: 확장성/장애격리/운영성 우수
- 단점: 초기 인프라 작업 증가

## 6) Go 기술 스택 제안
- Go: `1.26+`
- HTTP Router: `chi`
- DB Driver: `pgx/v5`
- SQL 관리: `sqlc` + raw SQL migrations
- Migration: `golang-migrate`
- Validation: `go-playground/validator`
- Config: `env` + `internal/config` (12-factor)
- Logging: `slog` (JSON)
- Metrics/Tracing: `OpenTelemetry` + Prometheus endpoint
- Auth:
  - MVP: static admin token or signed session (VPN 내부 전제)
  - 확장: OIDC (Google Workspace 등)

## 7) 백엔드 레포 구조 (제안)

```text
apps/backend/
├─ cmd/
│  ├─ api/main.go
│  └─ migrate/main.go
├─ internal/
│  ├─ config/
│  ├─ domain/
│  │  ├─ conversation/
│  │  ├─ dashboard/
│  │  ├─ security/
│  │  └─ infra/
│  ├─ repository/
│  │  ├─ postgres/
│  │  └─ memory/        # 테스트용
│  ├─ service/
│  ├─ http/
│  │  ├─ handler/
│  │  ├─ middleware/
│  │  └─ dto/
│  ├─ ingest/
│  └─ worker/
├─ migrations/
├─ api/openapi.yaml
└─ Makefile
```

레이어 원칙:
- `handler -> service -> repository` 단방향 의존
- 도메인 규칙은 `service/domain`에 두고, DB/HTTP 상세는 바깥 레이어로 격리

## 8) 핵심 도메인 모델

### 8.1 Conversation/Message
- Conversation: 세션 단위 메타 정보
- Message: 사용자/어시스턴트/시스템 메시지 단위 기록
- Request Attempt: 모델 호출 단위 (토큰/비용/지연/에러)

### 8.2 Dashboard Metric
- 시간 버킷(1m/5m/1h/day) 기반 집계
- 요청수, 성공률, 에러율, 토큰, 비용, 활성 계정 수

### 8.3 Infra Health
- WireGuard peer 수
- OpenClaw 서비스 상태(systemd)
- 최근 에러 이벤트 수

### 8.4 Security Finding
- 규칙 기반 진단 결과
- severity: `critical | high | medium | info`
- 상태: `open | acknowledged | resolved`

## 9) 데이터베이스 스키마 (초안)

주요 테이블:
- `accounts`
  - `id`, `external_id`, `email`, `status`, `created_at`
- `conversations`
  - `id`, `account_id`, `channel`, `status`, `started_at`, `ended_at`
- `messages`
  - `id`, `conversation_id`, `role`, `content_masked`, `content_raw_encrypted`, `created_at`
- `request_attempts`
  - `id`, `conversation_id`, `provider`, `model`, `tokens_in`, `tokens_out`, `cost_usd`, `latency_ms`, `success`, `error_code`, `created_at`
- `infra_snapshots`
  - `id`, `vpn_peer_count`, `openclaw_up`, `cpu_pct`, `mem_pct`, `captured_at`
- `security_findings`
  - `id`, `rule_id`, `severity`, `title`, `description`, `field`, `status`, `detected_at`, `resolved_at`
- `audit_events`
  - `id`, `actor`, `action`, `resource_type`, `resource_id`, `metadata_json`, `created_at`

인덱스 핵심:
- `messages(conversation_id, created_at)`
- `request_attempts(created_at, success)`
- `conversations(account_id, started_at)`
- `security_findings(status, severity, detected_at)`

## 10) API 설계 (MVP)

### 10.1 Health
- `GET /v1/healthz`
- `GET /v1/readyz`

### 10.2 Dashboard
- `GET /v1/dashboard/summary?from=...&to=...`
  - 요청수, 토큰, 비용, 에러율, 활성 계정 요약
- `GET /v1/dashboard/timeseries?metric=requests&bucket=1h&from=...&to=...`

### 10.3 Conversations
- `GET /v1/conversations?channel=telegram&status=failed&page=1&page_size=50`
- `GET /v1/conversations/{id}`
- `GET /v1/conversations/{id}/messages`
- `GET /v1/conversations/{id}/attempts`

### 10.4 Infra
- `GET /v1/infra/status`
- `GET /v1/infra/snapshots?from=...&to=...`

### 10.5 Security
- `POST /v1/security/analyze-tfvars`
  - 입력: tfvars JSON
  - 출력: finding 목록
- `GET /v1/security/findings?status=open`

### 10.6 Ingest (내부용)
- `POST /v1/ingest/conversation-events`
- `POST /v1/ingest/infra-snapshot`
- `POST /v1/ingest/request-attempt`

## 11) 대화 로그 저장 정책 (중요)
- 기본 정책:
  - `content_masked` 저장 (비밀값/토큰/키 패턴 마스킹)
  - `content_raw_encrypted`는 옵션 (기본 비활성)
- 보존 정책:
  - 메시지 원문: 7~30일 (옵션)
  - 메타데이터/집계: 90일 이상
- 열람 정책:
  - 상세 열람 권한 분리 (관리자만)
  - 조회/내보내기 이벤트는 `audit_events`에 기록

## 12) 인증/인가 전략

### MVP
- 네트워크: VPN 내부 접근 필수
- 인증: `Authorization: Bearer <admin-token>` (Secret Manager 관리)
- 인가: 단일 관리자 롤

### 확장
- OIDC 로그인 + 역할(`admin`, `viewer`, `auditor`)
- 세션 만료/강제 로그아웃/감사 추적 강화

## 13) 보안 설계 체크리스트
- API 서버 외부 IP 금지
- 백엔드 방화벽 `source_tags`/`source_ranges` 최소화
- DB 연결 TLS 강제
- at-rest 암호화 (Cloud SQL or disk encryption)
- 민감 정보 마스킹/암호화 저장
- rate limit + request size 제한 + audit logging
- CORS 엄격 설정 (허용 Origin 명시)

## 14) 운영/관측 설계
- `/metrics` 노출 (VPN 내부)
- 표준 로그 필드:
  - `request_id`, `user_id`, `path`, `latency_ms`, `status`, `error_code`
- 알림 조건 예시:
  - `error_rate > 5% (5m)`
  - `ingest 지연 > 2m`
  - `openclaw_up=false` 2회 연속

## 15) 성능/품질 요구사항
- API p95 지연:
  - summary API: <= 300ms
  - conversation detail: <= 500ms (page_size=100 기준)
- ingestion 처리량:
  - 초당 100 events 이상 (MVP 목표)
- 신뢰성:
  - 실패 이벤트 재시도/멱등키 지원

## 16) 개발 단계 계획

### Phase 0: 초기 세팅
- Go 모노리포 구조 생성
- 공통 config/logger/error 포맷 정리
- CI: `go test`, `golangci-lint`, migration check

### Phase 1: Core API + DB
- Postgres 스키마/migration 작성
- Conversation/Dashboard read API 구현
- 기본 인증 미들웨어 구현

### Phase 2: Ingest 파이프라인
- OpenClaw 이벤트 ingest endpoint
- infra snapshot ingest endpoint
- 멱등 처리/재시도 큐(필요 시 Redis) 도입

### Phase 3: Security Engine
- tfvars 분석 규칙 엔진 API
- severity별 finding 저장/조회

### Phase 4: 운영 고도화
- OTel/Prometheus 연동
- OIDC 인증 전환
- 보존 정책 자동 정리(job)

## 17) 프론트엔드 연동 포인트
- Home: `GET /v1/dashboard/summary`
- Outputs: 기존 수동 입력 유지 + backend 저장 옵션 추가
- Security Check: `POST /v1/security/analyze-tfvars`
- Conversation Timeline 신규:
  - 목록: `GET /v1/conversations`
  - 상세: `GET /v1/conversations/{id}/messages`
- Runbook: infra 상태와 연결하여 추천 우선순위 표시

## 18) 인프라(Terraform) 확장 항목
- 백엔드 VM 또는 컨테이너 런타임 리소스
- 백엔드 내부 LB/서비스 디스커버리 (선택)
- Cloud SQL (private IP) + service account 권한
- 백엔드 전용 firewall rule
- Secret Manager 항목:
  - `ops_api_admin_token`
  - `ops_api_db_dsn` (또는 분리된 DB credential)

## 19) 수용 기준 (Acceptance Criteria)
- 프론트 대시보드가 백엔드 API로 실데이터를 표시한다.
- 대화 목록/상세 조회가 페이지네이션으로 동작한다.
- 보안 점검 API가 최소 6개 규칙을 반환한다.
- API는 VPN 외부에서 접근 불가해야 한다.
- 기본 감사 로그(`audit_events`)가 인증된 조회 요청을 기록한다.

## 20) 오픈 이슈
- 대화 원문 저장 범위(완전 저장 vs 마스킹 저장) 정책 확정 필요
- Redis 도입 시점(MVP 즉시 vs 트래픽 이후)
- 백엔드를 OpenClaw VM에 동거할지, 전용 VM으로 분리할지 최종 결정 필요
- OIDC 도입 시점 및 IdP 선택 필요
