# Project: My Canine Advisor (나의 반려견 전담 조언자) - SPEC v3

본 프로젝트는 초보 반려인을 위해 '강형욱 훈련사' 페르소나를 가진 AI 에이전트가 반려견의 생애 주기와 과거 이력을 완벽히 기억하고, 일상 속 친구처럼 대화하며 조언을 제공하는 서비스를 목표로 한다.

---

## 1. 기술 스택 및 아키텍처

### 1.1 기술 스택

| 구분 | 선택 | 비고 |
|---|---|---|
| Core Backend | Go (Golang) + Gin | 고성능 동시성 및 비동기 제어 |
| AI Agent SDK | Google ADK for Go | Tool-use 및 세션/메모리 관리 |
| AI Model | Gemini 2.5/3 Flash | MVP용 고속/저비용 (심층 분석 시 Pro 전환 가능) |
| External Search | Google Search Grounding | Gemini 내장 실시간 웹 검색 |
| Database | PostgreSQL (GORM) | 관계형 데이터 + JSONB 계층적 메모리 저장 |
| Webview UI | Next.js (App Router) | 반려견 등록, 리포트 대시보드 (TypeScript) |
| Platform | KakaoTalk (주) / Telegram (부) | 어댑터 패턴으로 멀티 플랫폼 대응 |
| Deployment | GCP (Cloud Run + Cloud SQL) | Webview는 Vercel 또는 Cloud Run |

### 1.2 시스템 아키텍처

```
┌─────────────────────────────────────────────────────┐
│                   사용자 접점                          │
│  ┌──────────┐  ┌──────────┐  ┌───────────────────┐  │
│  │ 카카오톡  │  │ 텔레그램  │  │ Next.js Webview   │  │
│  │ (주 채널) │  │ (부 채널) │  │ (등록/대시보드)    │  │
│  └────┬─────┘  └────┬─────┘  └────────┬──────────┘  │
│       │              │                 │              │
│       ▼              ▼                 │              │
│  ┌──────────────────────┐              │              │
│  │  Platform Adapters   │              │              │
│  │  ├─ KakaoAdapter     │              │              │
│  │  └─ TelegramAdapter  │              │              │
│  └──────────┬───────────┘              │              │
│             │                          │              │
│             ▼                          ▼              │
│  ┌──────────────────────────────────────────────┐    │
│  │            Go Backend (Gin)                   │    │
│  │  ├─ Handler Layer (API Endpoints)             │    │
│  │  ├─ Agent Layer (ADK + Gemini + Tools)        │    │
│  │  ├─ Memory Manager (L1/L2)                    │    │
│  │  └─ Domain Layer (Business Logic)             │    │
│  └──────────────────┬───────────────────────────┘    │
│                     │                                 │
│                     ▼                                 │
│  ┌──────────────────────────────────────────────┐    │
│  │          PostgreSQL (Cloud SQL)               │    │
│  │  ├─ users / dogs (프로필)                     │    │
│  │  ├─ dog_category_contexts (L1)                │    │
│  │  ├─ dog_dynamic_summaries (L2)                │    │
│  │  └─ platform_accounts (플랫폼 계정 연결 예약)  │    │
│  └──────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────┘
```

### 1.3 카카오톡 연동 구조

카카오 i 오픈빌더를 통한 스킬서버 방식:

```
사용자(카카오톡) → 카카오톡 채널 → 오픈빌더(봇 엔진) → HTTP POST(JSON) → Go 스킬서버
                                                                              │
                                                                    ┌─────────┤
                                                                    │  5초 이내?
                                                                    ├─────────┤
                                                                  Yes        No
                                                                    │         │
                                                              직접 응답   useCallback: true
                                                                         + 고루틴 처리
                                                                         → callbackUrl POST
```

- **5초 타임아웃**: 오픈빌더 고정 제한. 변경 불가.
- **콜백 방식**: `useCallback: true`로 즉시 응답 → 고루틴에서 AI 처리 → `callbackUrl`로 최종 응답 POST.
- **콜백 제약**: URL 유효시간 1분, 1회만 사용 가능.
- **콜백 신청**: 챗봇 설정 > AI 챗봇 관리에서 별도 승인 필요.

### 1.4 오픈빌더 블록 구성

**폴백 + 메뉴형 하이브리드 구조**:

| 블록 | 트리거 | 처리 |
|---|---|---|
| AI 자유대화 (Fallback) | 일반 발화 전체 | Go 스킬서버로 전달 → AI 대화 |
| 메뉴 | "메뉴", 고정메뉴 버튼 | ListCard로 메뉴 표시 |
| 반려견 등록 | 메뉴에서 선택 | Webview URL 링크 |
| 프로필 수정 | 메뉴에서 선택 | Webview URL 링크 |
| 리포트 보기 | 메뉴에서 선택 | Webview URL 링크 |
| 반려견 전환 | 메뉴에서 선택 | ListCard로 등록된 반려견 목록 → 선택 |

---

## 2. 데이터 모델 (Data Models)

### 2.1 User (사용자)

```go
type User struct {
    ID        string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
    CreatedAt time.Time
    UpdatedAt time.Time

    // Relations
    Dogs             []Dog             `gorm:"foreignKey:UserID"`
    PlatformAccounts []PlatformAccount `gorm:"foreignKey:UserID"`
}
```

### 2.2 PlatformAccount (플랫폼 계정 - 향후 연결용)

MVP에서는 플랫폼별 독립 계정. DB 스키마만 준비하고 계정 연결 기능은 이후 구현.

```go
type PlatformAccount struct {
    ID         uint      `gorm:"primaryKey"`
    UserID     string    `gorm:"type:uuid;index"`
    Platform   string    `gorm:"index"` // "kakao", "telegram", "web"
    PlatformID string    `gorm:"uniqueIndex:idx_platform_id"` // 플랫폼별 고유 ID
    CreatedAt  time.Time
}
```

### 2.3 Dog (반려견 프로필)

불변 필드와 가변 필드를 구분하여 관리한다.

```go
type Dog struct {
    // === 불변 필드 (Immutable) ===
    ID        string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
    UserID    string    `gorm:"type:uuid;index"`
    Name      string    `gorm:"not null"`             // 이름
    Breed     string    `gorm:"not null"`             // 견종
    Birthday  time.Time `gorm:"not null"`             // 생년월일
    BirthdayEstimated bool `gorm:"default:false"`     // true: 추정치, false: 정확한 생일
    Gender    string    `gorm:"not null"`             // "male" / "female"
    CreatedAt time.Time

    // === 가변 필드 (Mutable) ===
    Weight       float64   `gorm:"type:decimal(5,2)"`  // 무게 (kg)
    Neutered     bool      `gorm:"default:false"`      // 중성화 여부
    ProfilePhoto string    `gorm:"type:text"`          // 프로필 사진 URL
    MedicalNotes string    `gorm:"type:text"`          // 의료 메모
    UpdatedAt    time.Time

    // Relations
    CategoryContexts []DogCategoryContext `gorm:"foreignKey:DogID"`
    DynamicSummary   DogDynamicSummary   `gorm:"foreignKey:DogID"`
}
```

**Birthday 추정 로직**: 사용자가 정확한 생일을 모를 경우 대략적인 개월 수를 입력하면, 서버에서 해당 개월 수 전의 1일을 자동으로 생일로 설정한다. 이때 `BirthdayEstimated = true`로 저장.

예: 사용자가 "8개월"을 입력하면 → `Birthday = 현재 날짜 - 8개월`의 1일, `BirthdayEstimated = true`

---

## 3. 계층적 메모리 시스템 (Hierarchical Memory)

에이전트는 데이터의 신선도와 중요도에 따라 2단계 계층으로 기억을 관리한다.

### 3.1 L1: Recent Context (단기 기억 - Sliding Window)

카테고리별 최근 N개 대화 조각을 유지하는 슬라이딩 윈도우.

```go
// L1 최소 단위: 대화 조각
type ChatSnippet struct {
    UserText string `json:"u"`    // 사용자 발화 요약
    AIText   string `json:"a"`    // AI 답변 요약
    Time     string `json:"t"`    // 발화 시간 (ISO8601)
}

// L1 DB 모델: 카테고리별 최근 문맥
type DogCategoryContext struct {
    ID          uint            `gorm:"primaryKey"`
    DogID       string          `gorm:"uniqueIndex:idx_dog_cat;type:uuid"`
    Category    string          `gorm:"uniqueIndex:idx_dog_cat"` // 7개 카테고리
    RecentItems []ChatSnippet   `gorm:"type:jsonb"`
    MaxItems    int             `gorm:"default:30"`  // 초기값: 전원 30개 (유료 티어)
    UpdatedAt   time.Time
}
```

**티어 시스템**: 설정 가능한 구조로 `MaxItems` 필드에 반영. 초기에는 전체 사용자에게 30개(유료 수준) 제공. 향후 무료(5개)/유료(30개) 분리 예정.

**저장 시점**: 매 메시지마다 L1에 즉시 저장.

### 3.2 L2: Dynamic Summary (중기 기억 - Snapshot & Pattern)

카테고리별 장기 패턴과 최근 관찰을 병합한 요약.

```go
// L2 카테고리 상태 단위
type CategoryStatus struct {
    Category   string    `json:"c"`      // 식사, 교육, 건강, 기분, 수면, 사회화, 환경
    Baseline   string    `json:"b"`      // 장기적으로 관찰된 지배적 패턴
    LatestObs  string    `json:"l"`      // 최근 대화에서 발견된 일시적/새로운 관찰
    Confidence int       `json:"conf"`   // 패턴 확신도 (1~5점)
    StatusTag  string    `json:"tag"`    // Stable / Changing / Anomaly
    UpdatedAt  time.Time `json:"u_at"`   // 해당 카테고리 마지막 업데이트 시간
}

// L2 DB 모델: 반려견 동적 요약 스냅샷
type DogDynamicSummary struct {
    DogID            string           `gorm:"primaryKey;type:uuid"`
    CategoryStatuses []CategoryStatus `gorm:"type:jsonb"`
    RapportNotes     string           `gorm:"type:text"` // MVP 미사용. 향후 보호자 성향 추적용 예약 필드
    LastUpdated      time.Time
}
```

### 3.3 Reconciliation (L1 → L2 압축/전이)

L1 데이터를 L2로 압축하는 두 가지 트리거:

| 트리거 | 시점 | 로직 |
|---|---|---|
| **세션 타임아웃** | 마지막 메시지로부터 30분 경과 시 | 세션 중 누적된 L1 데이터를 AI가 분석하여 L2 CategoryStatuses 갱신 |
| **L1 슬라이딩 윈도우 초과** | L1의 RecentItems가 MaxItems(30)을 초과할 때 | 가장 오래된 항목을 L2에 반영 후 제거. 점진적 업데이트 |

**Reconciliation 세부 로직**:
1. L1의 새로운 데이터와 L2의 기존 Baseline을 비교.
2. 일관된 패턴이면 → Baseline 유지, Confidence 상승.
3. 기존 패턴과 충돌하면 → 'Counter-Example'로 LatestObs에 기록, StatusTag를 'Changing'으로 변경.
4. Counter-Example이 반복되면 → Baseline을 갱신, Confidence 리셋.
5. 급격한 변화 감지 시 → StatusTag를 'Anomaly'로 설정.

---

## 4. 에이전트 도구 정의 (ADK Tool-use)

Google ADK에서 정의하는 3개의 하이브리드 도구:

### 4.1 load_context (대화 시작 시)

- **트리거**: 대화 시작 또는 세션 복원 시 자동 호출.
- **입력**: `dog_id`
- **출력**: Dog Profile + L1 RecentItems (현재 대화 중인 카테고리) + L2 CategoryStatuses 전체
- **목적**: 에이전트가 반려견의 현재 상태를 완전히 파악한 채로 대화 시작.

```go
type LoadContextInput struct {
    DogID string `json:"dog_id"`
}

type LoadContextOutput struct {
    Profile          Dog                   `json:"profile"`
    RecentContexts   []DogCategoryContext  `json:"recent_contexts"`   // L1
    DynamicSummary   DogDynamicSummary     `json:"dynamic_summary"`   // L2
}
```

### 4.2 search_history (대화 중 필요 시)

- **트리거**: 사용자가 과거 이력을 언급하거나, AI가 맥락상 과거 참조가 필요하다고 판단할 때.
- **입력**: `dog_id`, `query` (키워드 또는 카테고리), `time_range` (선택)
- **출력**: 관련 ChatSnippet 배열 (L1에서 검색) + L2의 해당 카테고리 Baseline
- **목적**: 과거 대화를 참조하여 "지난번에 말씀하셨던..." 같은 연속성 있는 답변 생성.

```go
type SearchHistoryInput struct {
    DogID     string `json:"dog_id"`
    Query     string `json:"query"`               // 키워드 또는 자연어 질의
    Category  string `json:"category,omitempty"`   // 특정 카테고리 필터
    TimeRange string `json:"time_range,omitempty"` // "7d", "30d", "all"
}

type SearchHistoryOutput struct {
    Snippets []ChatSnippet  `json:"snippets"`
    Baseline string         `json:"baseline,omitempty"` // 해당 카테고리의 L2 Baseline
}
```

### 4.3 save_and_reconcile (대화 종료 시)

- **트리거**: 세션 타임아웃(30분) 또는 사용자가 명시적으로 종료 시.
- **입력**: 현재 대화 세션의 요약 데이터
- **출력**: 업데이트된 L1, L2 상태
- **목적**: 대화 내용을 L1에 저장하고, L2 패턴을 갱신.

```go
type SaveAndReconcileInput struct {
    DogID      string         `json:"dog_id"`
    Category   []string       `json:"category"`    // 복수 카테고리 허용
    Snippets   []ChatSnippet  `json:"snippets"`    // 이번 세션의 대화 조각들
    Urgency    int            `json:"urgency"`     // 긴급도 (1-4)
    BehaviorTags []string     `json:"behavior_tags,omitempty"` // 관찰된 행동 태그
}

type SaveAndReconcileOutput struct {
    L1Updated bool   `json:"l1_updated"`
    L2Updated bool   `json:"l2_updated"`
    Summary   string `json:"summary"`  // 이번 세션 요약
}
```

**메시지 단위 L1 즉시 저장**: 위 도구와 별개로, 매 메시지 수신 시 Go 서버에서 L1 RecentItems에 즉시 append한다 (도구 호출이 아닌 서버 로직).

---

## 5. 카테고리 시스템

### 5.1 카테고리 정의

| 카테고리 | 설명 | 예시 |
|---|---|---|
| 식사 | 사료 거부, 간식 조절, 식탐, 구토 등 | "밥을 안 먹어요", "간식만 찾아요" |
| 교육 | 입질, 짖음, 배변 훈련, 산책 매너 등 | "산책할 때 줄을 당겨요" |
| 건강 | 질병 의심 증상, 활력 저하, 예방 접종 등 | "다리를 절어요", "예방접종 시기" |
| 기분 | 꼬리 치기, 카밍 시그널, 무기력, 신남 등 | "꼬리를 안 흔들어요" |
| 수면 | 잠자리 위치, 수면 시간, 수면 중 행동 등 | "밤에 계속 깨요" |
| 사회화 | 낯선 사람/강아지에 대한 반응 등 | "다른 강아지를 무서워해요" |
| 환경 | 이사, 가구 변경, 소음 문제 등 | "이사 후 불안해해요" |

### 5.2 분류 방식

- **AI 자동 분류**: 대화 내용을 AI가 분석하여 카테고리 자동 태깅.
- **복합 카테고리 허용**: 하나의 대화에 여러 카테고리 태깅 가능. 예: "밥을 안 먹고 무기력해요" → `["식사", "기분"]`
- `save_and_reconcile`의 `Category` 필드가 `[]string`인 이유.

---

## 6. 세션 관리

### 6.1 하이브리드 세션 모델

메시지 단위 즉시 저장 + 타임아웃 기반 reconciliation을 결합.

```
메시지 수신 → L1 즉시 저장 → AI 응답 생성 → 응답 반환
                                                  │
                                          타이머 리셋 (30분)
                                                  │
                                           30분 경과 시
                                                  │
                                          save_and_reconcile
                                          (L1→L2 압축)
```

### 6.2 세션 상태

```go
type ChatSession struct {
    SessionID    string    `json:"session_id"`
    UserID       string    `json:"user_id"`
    DogID        string    `json:"dog_id"`       // 현재 대화 중인 반려견
    Platform     string    `json:"platform"`      // "kakao", "telegram"
    StartedAt    time.Time `json:"started_at"`
    LastActiveAt time.Time `json:"last_active_at"`
    IsActive     bool      `json:"is_active"`
}
```

- **타임아웃**: 30분 (환경변수 `SESSION_TIMEOUT_MIN`으로 설정 가능).
- **타임아웃 체크**: 백그라운드 고루틴에서 주기적으로 확인하거나, 다음 메시지 수신 시 마지막 활동 시간 비교.
- **다견 전환**: 카카오톡 메뉴에서 '반려견 전환' 선택 시 ListCard로 등록된 반려견 목록 표시 → 선택하면 세션의 `DogID` 변경.

---

## 7. 에이전트 페르소나 및 긴급도

### 7.1 강형욱 훈련사 페르소나

| 특성 | 설명 | 예시 |
|---|---|---|
| 단호하면서 따뜻한 어조 | 직접적이지만 배려 있는 표현 | "보호자님, 이건 꼭 고쳐주셔야 해요" |
| 과학적 근거 제시 | 행동학/수의학적 이유 설명 | "앞발을 핥는 건 스트레스 신호일 수 있어요" |
| 보호자 교육 강조 | 강아지가 아닌 보호자 행동 변화 유도 | "강아지가 아니라 보호자님이 먼저 바뀌어야 해요" |
| 비유적 표현 | 이해하기 쉬운 비유 활용 | "줄 당기는 건 아이가 편의점 앞에서 떼쓰는 거랑 같아요" |
| 과거 이력 참조 | L1/L2 데이터를 활용한 연속 대화 | "지난번 식사 거부 때와는 다른 양상이네요" |
| Self-Correction | L1/L2 정보 충돌 시 사용자에게 직접 질문 | "지난번엔 잘 먹는다고 하셨는데, 최근에 바뀐 건가요?" |

### 7.2 긴급도 판단 (4단계)

| 레벨 | 분류 | 설명 | 에이전트 행동 |
|---|---|---|---|
| L1 | 일상 | 교육, 놀이, 일반 상담 | 일반적인 조언 제공 |
| L2 | 주의 | 가벼운 증상, 관찰 필요 | 조언 + 경과 관찰 권고 |
| L3 | 진료 권고 | 병원 예약 권장 수준 | **텍스트 강조** + 병원 검색 웹링크 제공 |
| L4 | 응급 | 즉시 병원 필요 | **긴급 경고 텍스트** + 24시 응급 동물병원 검색 링크 + **Webview 대시보드 알림 표시** |

**L3/L4 카카오톡 응답 형태**:
```json
{
  "version": "2.0",
  "template": {
    "outputs": [
      { "simpleText": { "text": "⚠️ [긴급] AI 응답 텍스트..." } }
    ],
    "quickReplies": [
      {
        "action": "webLink",
        "label": "🏥 근처 동물병원 찾기",
        "webLinkUrl": "https://search.naver.com/search.naver?query=24시+동물병원+근처"
      }
    ]
  }
}
```

---

## 8. Webview (Next.js)

### 8.1 MVP 범위

| 페이지 | 설명 |
|---|---|
| 반려견 등록 | 이름, 견종(검색 드롭다운), 생일(정확/추정), 무게, 성별, 중성화 입력 |
| 프로필 수정 | 가변 필드(무게, 중성화, 사진, 의료 메모) 수정 |
| 리포트 대시보드 | L2 카테고리 상태 + 최근 대화 타임라인 |

### 8.2 리포트 대시보드 구성

```
┌─────────────────────────────────────────────┐
│  🐕 바둑이 (말티즈, 8개월)                    │
├─────────────────────────────────────────────┤
│                                             │
│  [카테고리별 상태 카드 - 7개]                 │
│  ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐       │
│  │ 식사 │ │ 교육 │ │ 건강 │ │ 기분 │       │
│  │ 🟢   │ │ 🟡   │ │ 🟢   │ │ 🔴   │       │
│  │Stable│ │Chang.│ │Stable│ │Anomal│       │
│  └──────┘ └──────┘ └──────┘ └──────┘       │
│  ┌──────┐ ┌──────┐ ┌──────┐                │
│  │ 수면 │ │사회화│ │ 환경 │                │
│  │ 🟢   │ │ 🟡   │ │ 🟢   │                │
│  └──────┘ └──────┘ └──────┘                │
│                                             │
│  [최근 대화 타임라인]                        │
│  ─────────────────────────────              │
│  📅 2/9  식사 | "사료를 거부하고 간식만..."   │
│  📅 2/7  교육 | "산책 중 줄 당기는 행동..."   │
│  📅 2/5  기분 | "무기력하고 활동량 감소..."   │
│  📅 2/3  건강 | "왼쪽 앞발을 자꾸 핥아..."   │
│                                             │
│  [⚠️ 긴급 알림 영역 - L3/L4 발생 시 표시]    │
│                                             │
└─────────────────────────────────────────────┘
```

### 8.3 Webview 접근 방식

- 카카오톡 메뉴 버튼에서 Webview URL로 연결.
- URL에 인증 토큰 포함하여 사용자 식별 (카카오톡 user_id ↔ 서버 user_id 매핑).
- 반응형 디자인으로 모바일 최적화.

---

## 9. 프로젝트 구조 (Go Backend)

```
canine_advisor/
├── server/                       # Go 백엔드
│   ├── cmd/
│   │   └── server/
│   │       └── main.go          # 서버 엔트리포인트
│   ├── internal/
│   │   ├── adapter/             # 플랫폼 어댑터
│   │   │   ├── platform.go      # 어댑터 인터페이스
│   │   │   ├── kakao/           # 카카오톡 어댑터
│   │   │   │   ├── handler.go   # HTTP 핸들러 (스킬서버 엔드포인트)
│   │   │   │   ├── request.go   # 요청 DTO
│   │   │   │   └── response.go  # 응답 DTO
│   │   │   └── telegram/        # 텔레그램 어댑터 (이후)
│   │   ├── agent/               # ADK 에이전트
│   │   │   ├── agent.go         # 에이전트 정의 + 페르소나
│   │   │   ├── tools.go         # load_context, search_history, save_and_reconcile
│   │   │   └── prompt.go        # 시스템 프롬프트 관리
│   │   ├── domain/              # 도메인 모델
│   │   │   ├── dog.go           # Dog, DogProfile
│   │   │   ├── memory.go        # ChatSnippet, CategoryStatus, L1/L2 모델
│   │   │   ├── session.go       # ChatSession
│   │   │   └── user.go          # User, PlatformAccount
│   │   ├── repository/          # 데이터 접근 계층
│   │   │   ├── dog_repo.go
│   │   │   ├── memory_repo.go   # L1/L2 CRUD
│   │   │   └── user_repo.go
│   │   ├── service/             # 비즈니스 로직
│   │   │   ├── memory_manager.go    # L1/L2 관리, Reconciliation 로직
│   │   │   ├── session_manager.go   # 세션 생명주기 관리
│   │   │   └── dog_service.go       # 반려견 CRUD
│   │   └── config/              # 설정
│   │       └── config.go        # 환경변수, 상수
│   ├── migrations/              # DB 마이그레이션
│   ├── go.mod
│   ├── go.sum
│   └── Dockerfile
├── web/                          # Next.js Webview
│   └── ...
└── .claude/                      # 프로젝트 문서
    ├── SPEC_v3.md
    ├── PLAN.md
    └── RULES.md
```

---

## 10. 구현 우선순위 및 마일스톤

### Phase 1: 백엔드 API 코어 (MVP 핵심)

| 순서 | 작업 | 설명 |
|---|---|---|
| 1 | Go 프로젝트 구조 세팅 | Gin 라우터, GORM 연결, 폴더 구조 |
| 2 | DB 모델 및 마이그레이션 | User, Dog, L1, L2, PlatformAccount 테이블 |
| 3 | 반려견 CRUD API | 등록, 조회, 수정, 삭제 엔드포인트 |
| 4 | L1 메모리 매니저 | ChatSnippet 저장, 슬라이딩 윈도우 관리 |
| 5 | L2 메모리 매니저 + Reconciliation | CategoryStatus 갱신, L1→L2 압축 로직 |
| 6 | ADK 에이전트 정의 | 3개 도구 + 강형욱 페르소나 + Google Search Grounding |
| 7 | 세션 매니저 | 세션 생성/종료, 타임아웃 관리 (30분) |
| 8 | 단위 테스트 | L1/L2 메모리 매니저, Reconciliation 로직 |
| 9 | 통합 테스트 | API 엔드포인트 httptest |

### Phase 2: 카카오톡 연동

| 순서 | 작업 | 설명 |
|---|---|---|
| 10 | 카카오 어댑터 구현 | 스킬서버 핸들러, 요청/응답 DTO |
| 11 | 콜백 비동기 처리 | 5초 제한 대응 고루틴 + callbackUrl 처리 |
| 12 | 오픈빌더 블록 구성 | Fallback + 메뉴 블록 설정 |
| 13 | 다견 전환 | ListCard로 반려견 목록 표시/선택 |
| 14 | 긴급도 응답 | L3/L4 시 병원 링크 + 강조 응답 |

### Phase 3: Webview

| 순서 | 작업 | 설명 |
|---|---|---|
| 15 | Next.js 프로젝트 세팅 | App Router, TypeScript, Tailwind |
| 16 | 반려견 등록/수정 페이지 | 프로필 입력 폼, 견종 검색 드롭다운 |
| 17 | 리포트 대시보드 | L2 카테고리 상태 카드 + 대화 타임라인 |
| 18 | 긴급 알림 UI | L3/L4 발생 시 대시보드에 알림 표시 |

### Phase 4: 확장 (이후)

- 텔레그램 어댑터
- 계정 연결 (플랫폼 간 동일 사용자 매핑)
- RapportNotes 활성화 (보호자 성향 추적)
- 티어 시스템 (무료 5개 / 유료 30개 분리)
- 결제 연동 (토스페이먼츠 등)

---

## 11. 테스트 전략

### 11.1 단위 테스트 (필수)

| 대상 | 테스트 항목 |
|---|---|
| Memory Manager (L1) | ChatSnippet 추가, 슬라이딩 윈도우 초과 시 삭제, 카테고리별 격리 |
| Memory Manager (L2) | CategoryStatus 갱신, Confidence 증감, StatusTag 전이 |
| Reconciliation | L1→L2 압축 정확성, Counter-Example 처리, 패턴 갱신 |
| Session Manager | 세션 생성/종료, 타임아웃 감지, 다견 전환 |
| Birthday Estimation | 개월 수 → Birthday 변환 정확성 |

### 11.2 통합 테스트 (필수)

| 대상 | 테스트 항목 |
|---|---|
| API Endpoints | 반려견 CRUD, 메모리 조회/저장 |
| 카카오 어댑터 | 스킬서버 요청 파싱, 응답 포맷 검증, 콜백 처리 |
| E2E 메모리 흐름 | 메시지 수신 → L1 저장 → 세션 종료 → L2 갱신 전체 흐름 |

---

## 12. 환경 설정

```env
# Server
PORT=8080
GIN_MODE=release

# Database
DATABASE_URL=postgres://user:pass@localhost:5432/canine_advisor

# Google ADK / Gemini
GOOGLE_API_KEY=xxx
GEMINI_MODEL=gemini-2.5-flash

# Session
SESSION_TIMEOUT_MIN=30

# KakaoTalk
KAKAO_SKILL_API_KEY=xxx

# Tier (향후)
DEFAULT_MAX_ITEMS=30
```

---

## 부록 A: 카카오톡 스킬서버 요청/응답 포맷

### 요청 (Incoming)

```json
{
  "intent": { "id": "block-id", "name": "block-name" },
  "userRequest": {
    "timezone": "Asia/Seoul",
    "utterance": "바둑이가 밥을 안 먹어요",
    "user": { "id": "kakao-user-id", "properties": {} },
    "callbackUrl": "https://bot-api.kakao.com/callback/..."
  },
  "bot": { "id": "bot-id", "name": "My Canine Advisor" },
  "action": { "name": "fallback", "params": {} }
}
```

### 응답 - 즉시 (5초 이내)

```json
{
  "version": "2.0",
  "template": {
    "outputs": [
      { "simpleText": { "text": "AI 응답 메시지" } }
    ]
  }
}
```

### 응답 - 콜백 사용 시

**1단계: 즉시 응답**
```json
{
  "version": "2.0",
  "useCallback": true,
  "data": { "text": "잠시만 기다려주세요, 바둑이의 상태를 확인하고 있어요..." }
}
```

**2단계: callbackUrl로 POST**
```json
{
  "version": "2.0",
  "template": {
    "outputs": [
      { "simpleText": { "text": "실제 AI 분석 결과 응답" } }
    ]
  }
}
```

---

## 부록 B: 데모 시나리오

1. **반려견 등록**: Webview에서 "바둑이" (말티즈, 약 8개월 추정, 3.5kg, 수컷, 미중성화) 등록
2. **첫 상담** (카카오톡): "바둑이가 밥을 안 먹어요" → AI가 프로필 조회(load_context) 후 맞춤 상담
3. **두 번째 상담**: "산책할 때 줄을 당겨요" → AI가 L1에서 과거 식사 거부 이력 참조(search_history)하며 연속성 있는 답변
4. **기억 메커니즘 데모**: "지난번에 뭐라고 했었죠?" → search_history로 과거 로그 검색 후 정확히 답변
5. **긴급 상황 데모**: "바둑이가 피를 토했어요" → L4 긴급 판단 → 병원 링크 + Webview 알림
6. **리포트 확인**: Webview 대시보드에서 카테고리별 상태 + 타임라인 확인
