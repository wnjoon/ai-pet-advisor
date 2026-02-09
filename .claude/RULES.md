# My Canine Advisor - 개발 규칙

> 이 문서는 프로젝트의 모든 개발 과정에서 반드시 준수해야 하는 규칙을 정의한다.
> 관련 문서: [SPEC_v3.md](./SPEC_v3.md) | [PLAN.md](./PLAN.md)

---

## 1. 프로젝트 디렉토리 구조

```
canine_advisor/
├── server/          # Go 백엔드 (Gin + GORM + ADK)
│   ├── cmd/
│   ├── internal/
│   ├── migrations/
│   ├── go.mod
│   ├── go.sum
│   └── Dockerfile
├── web/             # Next.js Webview (App Router + TypeScript)
│   ├── src/
│   ├── package.json
│   └── ...
├── .claude/         # 프로젝트 문서
│   ├── SPEC_v3.md
│   ├── PLAN.md
│   └── RULES.md
└── README.md
```

- Go 서버와 Webview는 **완전히 분리된 디렉토리**에서 독립적으로 개발한다.
- 각 디렉토리는 자체 의존성 관리를 가진다 (`go.mod` / `package.json`).
- 환경변수는 각 디렉토리의 `.env` 파일에서 로드한다.
  - `server/.env` — Go 서버 환경변수 (godotenv 등으로 로드)
  - `web/.env` — Next.js 환경변수 (내장 .env 지원)
  - `.env` 파일은 **절대 커밋하지 않는다** (`.gitignore`에 등록됨).
  - `.env.example` 파일을 커밋하여 필요한 환경변수를 문서화한다.

---

## 2. Git 브랜치 전략

### 2.1 브랜치 계층

```
main                              # 프로덕션 (직접 커밋 금지)
└── dev                           # 개발 메인 브랜치
    ├── phase1/base               # Phase 1: 백엔드 API 코어
    │   ├── phase1/1.1-project-setup
    │   ├── phase1/1.2-db-models
    │   ├── phase1/1.3-dog-crud
    │   ├── phase1/1.4-l1-memory
    │   ├── phase1/1.5-l2-reconciliation
    │   ├── phase1/1.6-adk-agent
    │   ├── phase1/1.7-session-manager
    │   └── phase1/1.8-tests
    ├── phase2/base               # Phase 2: 카카오톡 연동
    │   ├── phase2/2.1-adapter-interface
    │   ├── phase2/2.2-kakao-handler
    │   ├── ...
    ├── phase3/base               # Phase 3: Webview
    │   └── ...
    └── phase4/base               # Phase 4: 확장
        └── ...
```

> Git은 `phase1`과 `phase1/*`이 동시에 존재할 수 없으므로, Phase 브랜치는 `phase{N}/base`로 명명한다.

### 2.2 브랜치 네이밍 규칙

| 레벨 | 패턴 | 예시 |
|------|------|------|
| Phase 브랜치 | `phase{N}/base` | `phase1/base`, `phase2/base` |
| 세부 단계 브랜치 | `phase{N}/{N.M}-{kebab-case-name}` | `phase1/1.1-project-setup` |

### 2.3 브랜치 생성/머지 플로우

```
[세부 단계 시작]
dev → phase{N}/base → phase{N}/{N.M}-xxx  (브랜치 생성)

[세부 단계 완료]
phase{N}/{N.M}-xxx → phase{N}/base     (머지 후 세부 브랜치 삭제)
phase{N}/base → phase{N}/{N.M+1}-yyy   (다음 단계 브랜치 생성)

[Phase 완료]
phase{N}/base → dev                     (머지 후 Phase 브랜치 삭제)
dev → remote push                       (원격에 push)
dev → phase{N+1}/base                   (다음 Phase 브랜치 생성)
```

---

## 3. 개발 워크플로우

### 3.1 작업 시작 시

1. PLAN.md에서 현재 진행할 단계 확인
2. 올바른 브랜치에 있는지 확인 (해당 세부 단계 브랜치)
3. 없으면 Phase 브랜치에서 새 세부 단계 브랜치 생성

### 3.2 작업 진행 중

- 코드 작성 중 자유롭게 커밋 (WIP 커밋 허용)
- **빌드 확인은 가능하나 서버/앱 실행은 금지** (아래 규칙 5 참조)

### 3.3 세부 단계 완료 시

1. **사용자 검증 필수**: 테스트 코드 실행 결과 또는 화면 동작을 사용자가 직접 확인
2. 사용자가 완료를 확정하면:
   - PLAN.md에서 해당 단계 체크박스 업데이트 (`- [ ]` → `- [x]`)
   - PLAN.md 상단 진행률 업데이트
   - PLAN.md 작업 로그에 기록 추가
   - 변경사항 커밋
   - Phase 브랜치에 머지
3. 다음 세부 단계 브랜치 생성 후 진행

### 3.4 Phase 완료 시

1. 모든 세부 단계가 완료되었는지 PLAN.md에서 확인
2. Phase 브랜치를 dev에 머지
3. dev를 remote에 push
4. PLAN.md 상단 Phase 상태 업데이트 (✅ 완료)
5. dev에서 다음 Phase 브랜치 생성

---

## 4. PLAN.md 동기화 규칙

> PLAN.md는 프로젝트의 **Single Source of Truth**이다.

- **모든 개발 단계는 PLAN.md에 기록되어야 한다.**
- 체크박스 상태는 실제 개발 상태와 항상 일치해야 한다.
- 새로운 작업 항목이 추가되면 PLAN.md에 먼저 반영한 후 개발을 시작한다.
- 기존 항목이 변경/삭제되면 PLAN.md를 즉시 업데이트한다.
- PLAN.md 업데이트는 별도 커밋 또는 해당 단계 완료 커밋에 포함한다.

---

## 5. 빌드 및 실행 규칙

### 절대 금지

- Claude 내부에서 Go 서버 실행 (`go run`, 장시간 프로세스)
- Claude 내부에서 Next.js 개발 서버 실행 (`npm run dev`, `next dev`)
- Claude 내부에서 Docker Compose 실행
- Claude 내부에서 PostgreSQL 등 외부 서비스 실행

### 허용

- 오류 확인을 위한 빌드 명령 (`go build ./...`, `go vet ./...`)
- 정적 분석 (`golangci-lint`, `tsc --noEmit`)
- 테스트 실행 (`go test ./...`, `npm test`) — **단, 외부 서비스 의존 없는 테스트만**
- 의존성 설치 (`go mod tidy`, `npm install`)

### 사용자 직접 실행

다음은 사용자가 직접 터미널에서 수행한다:

- Go 서버 빌드 및 실행
- Next.js 빌드 및 실행
- Docker Compose up/down
- DB 마이그레이션 실행
- 실제 카카오톡 연동 테스트
- E2E 테스트 (외부 서비스 의존)

---

## 6. 커밋 규칙

### 커밋 메시지 형식

```
<type>(<scope>): <description>

[optional body]

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
```

### Type

| Type | 설명 |
|------|------|
| `feat` | 새로운 기능 |
| `fix` | 버그 수정 |
| `refactor` | 리팩토링 (기능 변경 없음) |
| `test` | 테스트 추가/수정 |
| `docs` | 문서 변경 |
| `chore` | 빌드, 설정 등 기타 |
| `plan` | PLAN.md 업데이트 |

### Scope

| Scope | 설명 |
|-------|------|
| `server` | Go 백엔드 |
| `web` | Next.js Webview |
| `domain` | 도메인 모델 |
| `memory` | L1/L2 메모리 시스템 |
| `agent` | ADK 에이전트 |
| `kakao` | 카카오톡 어댑터 |
| `telegram` | 텔레그램 어댑터 |
| `db` | DB 마이그레이션/스키마 |

### 예시

```
feat(server/domain): define Dog model with immutable/mutable fields
feat(server/memory): implement L1 sliding window append logic
test(server/memory): add unit tests for L1 overflow eviction
plan: mark phase1/1.4 L1 memory manager as complete
docs: update SPEC_v3 with session timeout clarification
```

---

## 7. 코드 품질 규칙

### Go (server/)

- `go vet ./...` 경고 없어야 함
- `go build ./...` 성공해야 함
- public 함수/타입에 GoDoc 주석 작성
- 에러는 반드시 처리 (`_`로 무시 금지, 의도적 무시 시 주석)
- GORM 모델 변경 시 마이그레이션 파일 갱신

### TypeScript (web/)

- `tsc --noEmit` 에러 없어야 함
- strict mode 유지
- 컴포넌트는 함수형 + TypeScript props 타입 정의

---

## 8. 검증 체크리스트

### 세부 단계 완료 조건

아래 중 해당하는 항목이 모두 충족되어야 사용자에게 검증을 요청한다:

- [ ] 코드가 빌드 가능한 상태인가? (`go build` / `tsc`)
- [ ] 해당 단계의 PLAN.md 체크박스가 모두 구현되었는가?
- [ ] 단위 테스트가 작성되었는가? (테스트 대상인 경우)
- [ ] 기존 테스트가 깨지지 않았는가?

### Phase 완료 조건

- [ ] 모든 세부 단계가 완료 확정되었는가?
- [ ] Phase 내 모든 테스트가 통과하는가?
- [ ] PLAN.md가 최신 상태인가?
- [ ] Phase 브랜치가 dev에 정상 머지 가능한가? (충돌 없음)
