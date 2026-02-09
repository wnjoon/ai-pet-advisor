# My Canine Advisor - 개발 계획서

> 기준 문서: [SPEC_v3.md](./SPEC_v3.md) | [RULES.md](./RULES.md)
> 최종 업데이트: 2026-02-09
> 현재 단계: **Phase 2 카카오톡 연동 (코드 완료, 실환경 테스트 대기)**

---

## 진행 상황 요약

| Phase | 설명 | 상태 | 진행률 |
|-------|------|------|--------|
| Phase 1 | 백엔드 API 코어 | ✅ 완료 | 100% |
| Phase 2 | 카카오톡 연동 | 🔶 진행 중 | 90% |
| Phase 3 | Webview (Next.js) | 🔲 미시작 | 0% |
| Phase 4 | 확장 기능 | 🔲 미시작 | 0% |

---

## Phase 1: 백엔드 API 코어

> 목표: Go 백엔드 서버 + PostgreSQL + 메모리 시스템 + ADK 에이전트가 동작하는 상태

### 1.1 프로젝트 초기 세팅

- [x] Go 모듈 초기화 (`go mod init`)
- [x] 디렉토리 구조 생성
  ```
  server/cmd/server/main.go
  server/internal/adapter/platform.go
  server/internal/adapter/kakao/
  server/internal/agent/
  server/internal/domain/
  server/internal/repository/
  server/internal/service/
  server/internal/config/
  server/migrations/
  ```
- [x] 핵심 의존성 설치
  - [x] `github.com/gin-gonic/gin` (웹 프레임워크)
  - [ ] `gorm.io/gorm` + `gorm.io/driver/postgres` → 1.2에서 import 시 추가
  - [ ] `github.com/google/uuid` → 1.2에서 import 시 추가
  - [ ] Google ADK for Go SDK → 1.6에서 import 시 추가
- [x] 설정 관리 구조 (`internal/config/config.go`)
  - [x] 환경변수 로드 (PORT, DATABASE_URL, GOOGLE_API_KEY 등)
  - [x] SESSION_TIMEOUT_MIN, DEFAULT_MAX_ITEMS 설정
- [x] Gin 서버 기본 라우터 세팅 (`cmd/server/main.go`)
  - [x] Health check 엔드포인트 (`GET /health`)
  - [x] 미들웨어 (CORS, Logger, Recovery)
- [x] Docker Compose 로컬 개발 환경
  - [x] PostgreSQL 컨테이너
  - [x] (선택) Go 서버 컨테이너 — 스킵, 로컬 직접 실행

### 1.2 DB 모델 및 마이그레이션

- [x] 도메인 모델 정의 (`internal/domain/`)
  - [x] `user.go` — User, PlatformAccount 구조체
  - [x] `dog.go` — Dog 구조체 (불변/가변 필드 분리)
    - [x] BirthdayEstimated 플래그 포함
  - [x] `memory.go` — ChatSnippet, CategoryStatus, DogCategoryContext, DogDynamicSummary
  - [x] `session.go` — ChatSession 구조체
- [x] GORM AutoMigrate 연결
  - [x] User 테이블
  - [x] PlatformAccount 테이블 (스키마만, MVP 미사용)
  - [x] Dog 테이블
  - [x] DogCategoryContext 테이블 (L1)
  - [x] DogDynamicSummary 테이블 (L2)
- [x] JSONB 커스텀 타입 처리
  - [x] `[]ChatSnippet` ↔ JSONB 직렬화/역직렬화
  - [x] `[]CategoryStatus` ↔ JSONB 직렬화/역직렬화
- [x] 인덱스 확인
  - [x] `idx_dog_cat` (DogID + Category 복합 유니크)
  - [x] PlatformAccount의 `idx_platform_id` 유니크

### 1.3 반려견 CRUD API

- [x] Repository 계층 (`internal/repository/`)
  - [x] `dog_repo.go` — Create, GetByID, GetByUserID, Update, Delete
  - [x] `user_repo.go` — Create, GetByID, GetByPlatformID
- [x] Service 계층 (`internal/service/dog_service.go`)
  - [x] 반려견 등록 (Birthday 추정 로직 포함)
    - [x] 정확한 생일 입력 시: 그대로 저장, `BirthdayEstimated = false`
    - [x] 개월 수 입력 시: N개월 전 1일로 변환, `BirthdayEstimated = true`
  - [x] 반려견 조회 (단건, 사용자별 목록)
  - [x] 반려견 수정 (가변 필드만: weight, neutered, profile_photo, medical_notes)
  - [x] 반려견 삭제
  - [x] 등록 시 L1(7개 카테고리 빈 컨텍스트) + L2(빈 DynamicSummary) 자동 생성
- [x] API 핸들러 (Gin 라우터)
  - [x] `POST /api/dogs` — 반려견 등록
  - [x] `GET /api/dogs/:id` — 반려견 단건 조회
  - [x] `GET /api/users/:user_id/dogs` — 사용자의 반려견 목록
  - [x] `PATCH /api/dogs/:id` — 반려견 정보 수정
  - [x] `DELETE /api/dogs/:id` — 반려견 삭제
- [x] 입력 유효성 검사
  - [x] 필수 필드 (name, breed, gender)
  - [x] 생일: birthday 또는 age_months 중 하나 필수
  - [x] weight > 0

### 1.4 L1 메모리 매니저

- [x] Repository (`internal/repository/memory_repo.go`)
  - [x] L1 조회: GetCategoryContext(dogID, category)
  - [x] L1 전체 조회: GetAllCategoryContexts(dogID)
  - [x] L1 저장: UpsertCategoryContext(context)
- [x] Service (`internal/service/memory_manager.go`)
  - [x] **AppendToL1(dogID, category, snippet)**: L1에 ChatSnippet 추가
    - [x] MaxItems 초과 시 가장 오래된 항목 반환 (L2 전이용)
    - [x] JSONB 배열에 append + 길이 제한 적용
  - [x] **GetRecentContext(dogID, category)**: 특정 카테고리의 최근 문맥 조회
  - [x] **GetAllRecentContexts(dogID)**: 전체 카테고리 문맥 조회
  - [x] MaxItems 설정 기능 (향후 티어 시스템용)

### 1.5 L2 메모리 매니저 + Reconciliation

- [x] Repository
  - [x] L2 조회: GetDynamicSummary(dogID) — 1.4에서 memory_repo.go에 구현
  - [x] L2 저장: UpsertDynamicSummary(summary) — 1.4에서 memory_repo.go에 구현
- [x] Service — Reconciliation 로직
  - [x] **ReconcileOnSessionEnd(dogID, sessionSnippets)**: 세션 종료 시 전체 요약
    - [x] AI에게 세션 대화 + 기존 L2 전달 → 갱신된 CategoryStatuses 수신
    - [x] Baseline 비교 → 일관 시 Confidence 상승
    - [x] 충돌 시 LatestObs에 Counter-Example 기록 + StatusTag = "Changing"
    - [x] Counter-Example 반복 시 Baseline 갱신 + Confidence 리셋
    - [x] 급격한 변화 → StatusTag = "Anomaly"
  - [x] **ReconcileOnOverflow(dogID, category, evictedSnippets)**: L1 초과 시 점진 압축
    - [x] 제거되는 항목을 L2에 반영
    - [x] Confidence 미세 조정
  - [x] Reconciliation용 Gemini 프롬프트 설계
    - [x] 입력: 기존 L2 CategoryStatuses + 새 대화 조각들
    - [x] 출력: 갱신된 CategoryStatuses JSON

### 1.6 ADK 에이전트 정의

- [x] Google ADK 초기화 (`internal/agent/agent.go`)
  - [x] Gemini Flash 모델 연결
  - [x] Google Search Grounding 활성화
- [x] 강형욱 페르소나 시스템 프롬프트 (`internal/agent/prompt.go`)
  - [x] 어조, 스타일 가이드라인
  - [x] 과거 이력 참조 지시
  - [x] Self-Correction 지시 (L1/L2 충돌 시 질문)
  - [x] 긴급도 판단 기준 (L1~L4)
  - [x] 카테고리 자동 분류 + 복합 태깅 지시
- [x] 도구 3개 정의 (`internal/agent/tools.go`)
  - [x] **load_context**: Dog Profile + L1 + L2 로드
    - [x] 입력 스키마: `{ dog_id: string }`
    - [x] 구현: dog_repo + memory_repo 조합
  - [x] **search_history**: 과거 대화 검색
    - [x] 입력 스키마: `{ dog_id, query, category?, time_range? }`
    - [x] 구현: L1 RecentItems에서 키워드 매칭 + L2 Baseline 반환
  - [x] **save_and_reconcile**: 대화 저장 + L2 갱신
    - [x] 입력 스키마: `{ dog_id, category[], snippets[], urgency, behavior_tags? }`
    - [x] 구현: L1 저장 + ReconcileOnOverflow 호출
- [x] 에이전트 실행 파이프라인
  - [x] 메시지 수신 → load_context 자동 호출 → AI 응답 생성 → L1 즉시 저장

### 1.7 세션 매니저

- [x] Service (`internal/service/session_manager.go`)
  - [x] **CreateSession(userID, dogID, platform)**: 새 세션 생성
  - [x] **GetActiveSession(userID, platform)**: 활성 세션 조회
  - [x] **UpdateLastActive(sessionID)**: 마지막 활동 시간 갱신
  - [x] **EndSession(sessionID)**: 세션 종료 + ReconcileOnSessionEnd 트리거
  - [x] **SwitchDog(sessionID, newDogID)**: 대화 중 반려견 전환
- [x] 타임아웃 관리
  - [x] 다음 메시지 수신 시 lazy 체크 (30분 경과 여부)
  - [x] CleanupExpired() 메서드 (백그라운드 고루틴용)
  - [x] 타임아웃 시 자동 ReconcileOnSessionEnd 실행
- [x] 세션 저장소
  - [x] 인메모리 (sync.Map) — MVP
  - [ ] (향후) Redis 또는 DB 영속화

### 1.8 테스트

- [x] 단위 테스트
  - [x] `memory_manager_test.go` (6개)
    - [x] L1 AppendToL1: 정상 추가, MaxItems 초과 시 eviction, 다중 eviction
    - [x] GetRecentContext, GetAllRecentContexts
    - [x] UpdateMaxItems
  - [x] `session_manager_test.go` (11개)
    - [x] 세션 생성/종료, 타임아웃 감지, 다견 전환
    - [x] GetOrCreate, UpdateLastActive, CleanupExpired
  - [x] `dog_service_test.go` (10개)
    - [x] Birthday 추정 변환 (8개월 → 정확한 날짜)
    - [x] 가변 필드만 업데이트, 유효성 검사
- [x] 통합 테스트
  - [x] `api_test.go` (httptest, 7개)
    - [x] 반려견 CRUD 전체 플로우 (생성/조회/수정/삭제)
    - [x] 404/400 에러 케이스
  - [x] `memory_integration_test.go` (6개)
    - [x] L1 저장 → overflow → L2 갱신 E2E 흐름
    - [x] 세션 종료 시 L2 reconciliation
    - [x] Rule-based fallback, 다중 카테고리 플로우
- [x] 테스트 인프라
  - [x] `testutil/testdb.go`: SQLite 인메모리 테스트 DB 헬퍼

---

## Phase 2: 카카오톡 연동

> 목표: 카카오톡에서 실제로 대화하고 AI 응답을 받을 수 있는 상태
> 전제: Phase 1 완료

### 2.1 카카오 어댑터 기본 구조

- [x] Platform Adapter 인터페이스 (`internal/adapter/platform.go`) — Phase 1에서 구현
- [x] 카카오 요청 DTO (`internal/adapter/kakao/request.go`)
- [x] 카카오 응답 빌더 (`internal/adapter/kakao/response.go`)

### 2.2 카카오톡 핸들러 구현

- [x] 요청 DTO — KakaoRequest, KakaoUserRequest, KakaoAction 등
- [x] 응답 빌더 — SimpleText, ListCard, QuickReply, CallbackAck, ErrorResponse
- [x] HTTP 핸들러 (`POST /kakao/skill`)
  - [x] 요청 파싱 → 액션별 분기 (chat, menu, switch_dog)
  - [x] Fallback → AI 대화 파이프라인 호출

### 2.3 콜백 비동기 처리

- [x] 즉시 `useCallback: true` 응답 + 고루틴 AI 처리 + callbackUrl POST
- [x] 55초 타임아웃 (60초 콜백 만료 대비)
- [x] 네트워크 오류 시 재시도 (1회)
- [x] 단순 질의(메뉴, 반려견 전환)는 직접 응답, AI 대화만 콜백

### 2.4 오픈빌더 블록 연동

- [x] Fallback → AI 자유대화 (handleChat)
- [x] 메뉴 블록 — ListCard (등록/전환/리포트)
- [x] 반려견 전환 — 목록 조회 → ListCard → SwitchDog
- [x] x-api-key 인증 미들웨어
- [ ] 오픈빌더 UI 설정 (사용자 작업: 스킬 서버 URL, Fallback 블록, 메뉴 블록)

### 2.5 긴급도 응답 처리

- [x] L3 — 텍스트 + "동물병원 찾기" QuickReply (네이버 검색)
- [x] L4 — 텍스트 + "24시 동물병원 찾기" QuickReply (네이버 검색)
- [ ] Webview 대시보드 알림 플래그 (Phase 3에서 구현)

### 2.6 카카오톡 연동 테스트

- [x] 단위 테스트 — 응답 빌더 JSON 포맷 검증 (5개)
- [x] 핸들러 테스트 — 긴급도 감지, 라우팅, 인증 (9개)
- [ ] 실환경 테스트 — ngrok + 오픈빌더 연결 (사용자 작업)

---

## Phase 3: Webview (Next.js)

> 목표: 반려견 등록/수정 + 리포트 대시보드가 동작하는 상태
> 전제: Phase 1 완료, Phase 2는 병행 가능

### 3.1 Next.js 프로젝트 세팅

- [ ] Next.js (App Router) 프로젝트 생성 (`web/` 디렉토리)
- [ ] TypeScript 설정
- [ ] Tailwind CSS 설정
- [ ] shadcn/ui 초기화 (뉴모피즘 커스터마이징 기반)
- [ ] Go API 서버 연동 설정 (API Base URL 환경변수)
- [ ] 모바일 퍼스트 반응형 레이아웃

### 3.2 반려견 등록 페이지

- [ ] 등록 폼 UI
  - [ ] 이름 입력 (텍스트)
  - [ ] 견종 검색 드롭다운 (한국 인기 견종 포함)
  - [ ] 생일 입력 모드 전환
    - [ ] "정확한 생일 알아요" → 날짜 선택
    - [ ] "대략적인 나이만 알아요" → 개월 수 입력
  - [ ] 무게 입력 (숫자, kg)
  - [ ] 성별 선택 (라디오/토글)
  - [ ] 중성화 여부 (토글)
- [ ] 폼 유효성 검사
- [ ] API 연동 (`POST /api/dogs`)
- [ ] 등록 완료 후 카카오톡으로 돌아가기 안내

### 3.3 프로필 수정 페이지

- [ ] 기존 프로필 로드 (`GET /api/dogs/:id`)
- [ ] 가변 필드만 수정 가능 (무게, 중성화, 사진, 의료 메모)
- [ ] 불변 필드는 읽기 전용 표시
- [ ] API 연동 (`PATCH /api/dogs/:id`)

### 3.4 리포트 대시보드

- [ ] 반려견 정보 헤더 (이름, 견종, 나이 자동 계산)
- [ ] 카테고리별 상태 카드 (7개)
  - [ ] StatusTag에 따른 색상 표시 (Stable=초록, Changing=노랑, Anomaly=빨강)
  - [ ] Confidence 레벨 표시
  - [ ] Baseline 요약 텍스트
  - [ ] LatestObs (최근 관찰) 표시
- [ ] 최근 대화 타임라인
  - [ ] 시간순 정렬
  - [ ] 카테고리 태그 표시
  - [ ] 대화 요약 (UserText + AIText)
- [ ] 긴급 알림 영역
  - [ ] L3/L4 발생 시 상단 배너 표시
  - [ ] 알림 확인/해제 기능
- [ ] API 연동
  - [ ] `GET /api/dogs/:id` — 프로필
  - [ ] `GET /api/dogs/:id/summary` — L2 CategoryStatuses
  - [ ] `GET /api/dogs/:id/timeline` — L1 최근 대화 타임라인
  - [ ] `GET /api/dogs/:id/alerts` — 긴급 알림 조회

### 3.5 인증/접근 제어

- [ ] 카카오톡 Webview URL에 토큰 파라미터 포함 방식 설계
- [ ] 토큰 검증 미들웨어 (Go API 서버)
- [ ] 사용자 ↔ 반려견 소유권 검증

### 3.6 Webview 테스트

- [ ] 반려견 등록 플로우 E2E
- [ ] 프로필 수정 플로우
- [ ] 대시보드 데이터 표시 정확성
- [ ] 모바일 반응형 확인

---

## Phase 4: 확장 기능 (이후)

> Phase 1~3 완료 후 순차적으로 진행

### 4.1 텔레그램 어댑터

- [ ] Telegram Bot API 연동
- [ ] TelegramAdapter 구현 (PlatformAdapter 인터페이스)
- [ ] 봇 명령어 구성 (/start, /register, /switch, /report)
- [ ] 인라인 키보드로 반려견 전환

### 4.2 계정 연결

- [ ] PlatformAccount 활성화
- [ ] Webview에서 로그인 후 여러 플랫폼 ID를 하나의 User에 연결
- [ ] 연결된 계정 간 데이터(L1/L2) 공유

### 4.3 RapportNotes 활성화

- [ ] 보호자 성향 추적 로직 구현
- [ ] 대화 스타일 메모 (상세/간략 선호, 이모지 사용 등)
- [ ] 에이전트 어조 동적 조절

### 4.4 티어 시스템

- [ ] 무료 (MaxItems=5) / 유료 (MaxItems=30) 분리
- [ ] User 모델에 tier 필드 추가
- [ ] 등록 시 기본 무료 → 업그레이드 플로우

### 4.5 결제 연동

- [ ] 토스페이먼츠 PG 연동
- [ ] Webview 결제 페이지
- [ ] 구독 관리 (월간/연간)

---

## 의존성 관계

```
Phase 1.1 프로젝트 세팅
    └─→ Phase 1.2 DB 모델
         ├─→ Phase 1.3 반려견 CRUD
         ├─→ Phase 1.4 L1 메모리
         │    └─→ Phase 1.5 L2 + Reconciliation
         │         └─→ Phase 1.6 ADK 에이전트
         │              └─→ Phase 1.7 세션 매니저
         │                   └─→ Phase 1.8 테스트
         │                        └─→ Phase 2 카카오톡
         │                             └─→ Phase 4 확장
         └─→ Phase 3.1 Next.js 세팅 (Phase 1.3 이후 병행 가능)
              └─→ Phase 3.2~3.6 Webview 기능
```

**병행 가능한 작업**:
- Phase 1.3 (CRUD) 완료 후 → Phase 3 (Webview) 시작 가능
- Phase 2 (카카오톡)와 Phase 3 (Webview)는 독립적으로 병행 가능

---

## 작업 로그

> 각 작업 세션 종료 시 여기에 기록

| 날짜 | 작업 내용 | 완료 항목 | 메모 |
|------|----------|----------|------|
| 2026-02-09 | SPEC_v3, 개발 계획서 작성 | 문서 작성 | 인터뷰 기반 스펙 확정 |
| 2026-02-09 | Phase 1.1 프로젝트 초기 세팅 | Go 모듈, Gin 서버, config, 디렉토리 구조 | GORM/uuid/ADK는 사용 시 추가 예정 |
| 2026-02-09 | Phase 1.2 DB 모델 및 마이그레이션 | domain 4파일, AutoMigrate, JSONB 타입 | DB명 ai_pet_advisor로 변경 |
| 2026-02-09 | Phase 1.3 반려견 CRUD API | repo/service/handler, User+Dog CRUD | Birthday 추정, L1/L2 초기화, 유효성검사 |
| 2026-02-09 | Phase 1.4 L1 메모리 매니저 | memory_repo, memory_manager | AppendToL1 eviction, MaxItems 관리 |
| 2026-02-09 | Phase 1.5 L2 + Reconciliation | reconciler, reconciler_prompt | AI 인터페이스 + 룰기반 폴백 + 프롬프트 |
| 2026-02-09 | Phase 1.6 ADK 에이전트 | agent, tools, prompt, chat_handler | ADK v0.4.0, 강형욱 페르소나, 3 tools |
| 2026-02-09 | Phase 1.7 세션 매니저 | session_manager | sync.Map, lazy timeout, L2 reconcile on end |
| 2026-02-09 | 프롬프트 튜닝 | prompt.go, agent.go | 되묻기 금지, 가용 정보 기반 답변 우선, 빈 응답 수정 |
| 2026-02-09 | Phase 1.8 테스트 | 38개 테스트 전체 통과 | SQLite 인메모리, 단위+통합 테스트 |
| 2026-02-09 | Phase 2 카카오톡 연동 | 어댑터, 핸들러, 콜백, 긴급도 | 52개 테스트 전체 통과 |
| | | | |
