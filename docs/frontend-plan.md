# Frontend Product Plan: WireGuard + OpenClaw Ops Console (MVP)

## 1) 목적
- 이 문서는 `terraform-gcp-wireguard-openclaw` 레포를 위한 프론트엔드 웹사이트(운영 콘솔) 기획서다.
- 목표는 Terraform 중심의 운영 절차를 UI로 단순화하고, 보안 실수를 줄이며, 접속/운영 가시성을 높이는 것이다.

## 2) 프로젝트 컨텍스트
- 현재 레포는 GCP에 다음을 프로비저닝하는 Terraform 모듈이다.
  - WireGuard VM (wg-easy)
  - OpenClaw VM (기본값: 외부 IP 없음, VPN 전용 접근)
  - Firewall, NAT, 출력값(`wgeasy_ui_url`, `openclaw_gateway_url` 등)
- 사용자는 대부분 CLI/Terraform 사용자이며, 웹 UI는 아직 없다.

## 3) 제품 목표 / 비목표

### 목표 (MVP)
- Terraform 입력 작성 부담을 낮춘다.
- 위험한 보안 설정을 배포 전에 발견하게 한다.
- 배포 후 필수 출력값과 접속 절차를 한 화면에서 제공한다.
- 운영 시 자주 쓰는 점검 명령을 빠르게 실행 가능한 형태로 제공한다(복사 중심).

### 비목표 (MVP 범위 밖)
- 실제 Terraform apply를 브라우저에서 원격 실행하는 기능
- GCP API 직접 호출 기반 실시간 인프라 상태 수집
- 멀티 사용자 권한 관리/감사 로그

## 4) 타깃 사용자
- 1차: DevOps/플랫폼 엔지니어, 개인 운영자 (Terraform/GCP 사용 가능)
- 2차: 인프라 초보 개발자 (가이드 중심 사용)

## 5) 핵심 가치 제안
- "CLI 문서를 UI 워크플로우로 변환"
- "보안 경고를 사전에 시각화"
- "배포 직후 필요한 정보(접속 URL/IP/체크리스트) 즉시 제공"

## 6) 정보 구조 (IA)

```text
Home
├─ Deploy Wizard
├─ Security Check
├─ Outputs Dashboard
├─ Connect Guide
└─ Runbook
```

권장 네비게이션:
- 좌측 고정 사이드바 + 상단 프로젝트 상태 바
- 모바일에서는 하단 탭(핵심 4개) + 햄버거 메뉴(전체)

## 7) 핵심 사용자 플로우

### 플로우 A: 첫 배포 준비
1. Deploy Wizard에서 변수 입력
2. `terraform.tfvars` 프리뷰/다운로드
3. Security Check에서 위험 항목 확인
4. 로컬에서 `terraform init/plan/apply` 실행

### 플로우 B: 배포 후 접속
1. Outputs Dashboard에 `terraform output -json` 결과 붙여넣기
2. 핵심 출력값 자동 표시 (`vpn_public_ip`, `wgeasy_ui_url`, `openclaw_gateway_url`)
3. Connect Guide 단계대로 WireGuard 연결 후 OpenClaw 접속

### 플로우 C: 장애 점검
1. Runbook에서 상황 선택(접속 불가/컨테이너 비정상/게이트웨이 미응답)
2. 추천 명령 복사 및 실행
3. 결과 기반 다음 조치 확인

## 8) 화면 상세 기획

## 8.1 Home
- 목적: 현재 환경과 다음 액션 요약
- 콘텐츠:
  - "아직 출력값 없음" vs "최근 출력값 로드됨" 상태 카드
  - 바로가기 버튼: `배포 변수 작성`, `보안 점검`, `출력값 입력`
  - 최근 체크 결과(보안 경고 개수)

## 8.2 Deploy Wizard
- 목적: Terraform 변수 작성 실수 감소
- 구성:
  - 섹션 1: GCP 기본값 (`project_id`, `region`, `zone`)
  - 섹션 2: VPN (`wg_port`, `wgeasy_ui_port`, `ui_source_ranges`, `ssh_source_ranges`)
  - 섹션 3: OpenClaw (`openclaw_gateway_port`, `openclaw_version`, 모델/토큰)
  - 섹션 4: 보안 옵션 (`openclaw_enable_public_ip`, OS Login 등)
- 출력:
  - `terraform.tfvars` 텍스트 생성
  - 값 검증 에러/경고 표시

## 8.3 Security Check
- 목적: 보안 리스크 사전 확인
- 규칙 예시:
  - `ui_source_ranges` 또는 `ssh_source_ranges`에 `0.0.0.0/0` 포함 시 High
  - `openclaw_enable_public_ip=true` 시 Critical
  - 평문 비밀번호 사용 시 Medium (Secret Manager 사용 권장)
  - 기본 포트 유지 시 Info (정책에 따라 변경 권장)
- 결과:
  - 심각도별 카드 + 수정 가이드 + 해당 필드로 이동

## 8.4 Outputs Dashboard
- 목적: 배포 결과를 빠르게 활용
- 입력:
  - 사용자가 `terraform output -json` 결과를 붙여넣음
- 표시:
  - `vpn_public_ip`
  - `wgeasy_ui_url`
  - `openclaw_gateway_url`
  - `vpn_internal_ip`
  - `openclaw_internal_ip`
- 기능:
  - URL 열기 버튼(새 탭)
  - 값 복사
  - 마스킹 토글(민감 값 대응 확장 대비)

## 8.5 Connect Guide
- 목적: 실제 접속 성공률 향상
- 단계:
  1. wg-easy UI 접속
  2. 클라이언트 프로파일 생성/다운로드
  3. WireGuard 연결 확인
  4. VPN 연결 상태에서 OpenClaw Gateway 접속
- 체크박스 기반 진행 상태 저장(LocalStorage)

## 8.6 Runbook
- 목적: 장애 대응 시간 단축
- 시나리오별 명령 템플릿:
  - 컨테이너 상태 확인: `sudo docker ps`, `sudo docker logs wg-easy`
  - 스타트업 로그: `sudo journalctl -u google-startup-scripts.service`
  - OpenClaw 서비스: `sudo systemctl status openclaw`, `sudo journalctl -u openclaw -n 200`
- UX:
  - "명령 복사" + "해석 가이드" + "다음 조치"

## 9) 상태/데이터 모델 (UI 기준)

```ts
type TfvarsInput = {
  project_id: string;
  region: string;
  zone: string;
  wg_port: number;
  wgeasy_ui_port: number;
  ui_source_ranges: string[];
  ssh_source_ranges: string[];
  openclaw_gateway_port: number;
  openclaw_enable_public_ip: boolean;
  openclaw_version: string;
  // 확장: secret refs
  openclaw_gateway_password?: string;
  openclaw_gateway_password_secret_ref?: string;
};

type TerraformOutputs = {
  vpn_public_ip?: string;
  wgeasy_ui_url?: string;
  wireguard_port?: number;
  vpn_internal_ip?: string;
  openclaw_internal_ip?: string;
  openclaw_gateway_url?: string;
};

type SecurityIssue = {
  id: string;
  severity: "critical" | "high" | "medium" | "info";
  title: string;
  description: string;
  field?: keyof TfvarsInput;
  fixHint: string;
};
```

## 10) 디자인 방향 (Gemini 구현 가이드)
- 톤: "Infrastructure Control Room"
- 키워드: 청록/슬레이트 기반, 명확한 상태 색(정상/경고/위험), 모노스페이스 보조 타이포
- 피해야 할 것:
  - 보라색 중심 기본 템플릿 룩
  - 카드만 반복되는 대시보드 클론
- 권장:
  - 강한 타이포 대비(헤드라인 + 코드형 텍스트)
  - 배경은 그리드/노이즈/그라디언트로 공간감
  - 상태 변화 애니메이션(초기 로드/검사 완료)만 제한적으로 사용

## 11) 기술 가정
- 프론트엔드는 정적 배포 가능 구조 권장 (예: Next.js static export 또는 Vite SPA)
- 초기 버전은 백엔드 없이 클라이언트 상태(LocalStorage) 중심
- 향후 확장:
  - Secret Manager 설정 가이드 화면
  - 실제 Terraform state/plan 업로드 파서

## 12) 접근성/반응형 요구사항
- 모바일 최소 폭 360px 대응
- 색상만으로 상태 전달 금지(아이콘/텍스트 병행)
- 키보드 포커스 이동 가능
- 명도 대비 WCAG AA 수준 목표

## 13) 성능/품질 요구사항
- Lighthouse(모바일) Performance 80+ / Accessibility 90+ 목표
- 초기 로드 JS 번들 과대화 금지 (MVP 기준 라우트 단위 분리)
- 에러 상태(잘못된 JSON, 빈 출력값) 명확한 메시지 제공

## 14) MVP 수용 기준 (Acceptance Criteria)
- 사용자가 UI에서 입력한 값으로 `terraform.tfvars`를 생성하고 복사/다운로드할 수 있다.
- 최소 4개 보안 규칙을 검사하고 심각도별로 보여준다.
- `terraform output -json` 붙여넣기 시 핵심 출력값 6개를 파싱해 표시한다.
- Connect Guide 단계 완료 상태가 새로고침 후에도 유지된다.
- Runbook에서 최소 6개 점검 명령을 시나리오별로 제공한다.
- 데스크톱(>=1280px)과 모바일(<=390px) 레이아웃이 깨지지 않는다.

## 15) 구현 우선순위 (Gemini 전달용)
1. 라우팅/레이아웃 셸(사이드바 + 상단 상태바)
2. Deploy Wizard + tfvars 생성기
3. Security Check 규칙 엔진
4. Outputs Dashboard JSON 파서
5. Connect Guide 상태 저장
6. Runbook 시나리오 UI
7. 최종 반응형/접근성 정리

## 16) 제외 결정 기록
- 이번 단계에서는 실제 인프라 변경 실행 기능을 넣지 않는다.
- 인증/권한 체계는 단일 사용자 로컬 도구로 가정하고 추후 도입한다.
- 실시간 GCP 상태 동기화는 추후 백엔드 도입 시 검토한다.
