# Project: My Canine Advisor (나의 반려견 전담 조언자)

본 프로젝트는 초보 반려인을 위해 '강형욱 훈련사' 페르소나를 가진 AI 에이전트가 반려견의 생애 주기와 과거 이력을 완벽히 기억하고, 일상 속 친구처럼 대화하며 조언을 제공하는 서비스를 목표로 한다.

## 1. 기술 스택 및 아키텍처

### 1.1 기술 스택

|구분|선택|비고|
|---|---|---|
|Core Backend|Go (Golang)|"Gin/Echo 프레임워크, 고성능 동시성 및 비동기 제어"|
|AI Agent SDK|Google ADK for Go|도구 사용(Tool-use) 및 세션/메모리 관리|
|AI Model|Gemini 2.5/3 Flash|MVP용 고속/저비용 모델 (심층 분석 시 Pro 전환 가능)|
|Database|PostgreSQL (GORM)|관계형 데이터 및 JSONB를 활용한 계층적 메모리 저장|
|Webview UI|Next.js (App Router)|"반려견 등록, 결제, 리포트 시각화 (TypeScript)"|
|Platform|KakaoTalk / Telegram|멀티 플랫폼 대응을 위한 어댑터 패턴 적용|

### 1.2 시스템 아키텍처

- Hybrid Architecture: 로직 핵심(Go)과 인터페이스(Next.js Webview)의 분리.
- Messenger Abstraction: 플랫폼별 전용 어댑터를 통한 확장성(카카오톡, 텔레그램, 토스 인앱) 확보.
- Async Callback: 카카오톡 5초 제한 대응을 위한 비동기 고루틴 및 콜백 API 구조.

## 2. 계층적 메모리 시스템 (Hierarchical Memory)

에이전트는 데이터의 신선도와 중요도에 따라 3단계 계층으로 기억을 관리한다.

### 2.1 L1: Recent Context (단기 기억 - Sliding Window)

- 구조: 카테고리별 최근 N개 메시지 리스트.
- 특징: 티어별 사이즈 제한(무료 5개, 유료 30개). 플랫폼이 달라도 DB에서 로드하여 유지.

### 2.2 L2: Dynamic Summary (중기 기억 - Snapshot & Pattern)

- 구조: 카테고리별 '일반적 패턴(Baseline)'과 '최근 특이사항(Latest Observation)'의 병합 요약.
- Reconciliation 로직: L1 데이터가 L2로 압축될 때, 기존 패턴과 충돌하면 'Counter-Example'로 기록하고 반복 시 패턴을 갱신(Confidence Score 관리).

## 3. 세부 데이터 구조 (Data Structures)

### 3.1 ChatSnippet (L1 대화 조각)

L1 리스트(RecentItems)를 구성하는 최소 단위이다.

```go
type ChatSnippet struct {
    UserText string `json:"u"`    // 사용자 발화 요약
    AIText   string `json:"a"`    // AI 답변 요약
    Time     string `json:"t"`    // 발화 시간 (ISO8601)
}
```

### 3.2 CategoryStatus (L2 카테고리 상태)

L2(CategoryStatuses)에서 각 카테고리의 상태와 신뢰도를 관리하는 단위이다.

```go
type CategoryStatus struct {
    Category      string    `json:"c"`      // 식사, 교육, 건강, 기분, 수면, 사회화, 환경
    Baseline      string    `json:"b"`      // 장기적으로 관찰된 지배적 패턴
    LatestObs     string    `json:"l"`      // 최근 대화에서 발견된 일시적/새로운 관찰
    Confidence    int       `json:"conf"`   // 패턴의 확신도 (1~5점)
    StatusTag     string    `json:"tag"`    // Stable(안정), Changing(변화 중), Anomaly(이상 징후)
    UpdatedAt     time.Time `json:"u_at"`   // 해당 카테고리 마지막 업데이트 시간
}
```

### 3.3 GORM 메인 모델 (Database Models)

```go
// L1: 카테고리별 최근 문맥
type DogCategoryContext struct {
    ID          uint          `gorm:"primaryKey"`
    DogID       string        `gorm:"uniqueIndex:idx_dog_cat;type:uuid"`
    Category    string        `gorm:"uniqueIndex:idx_dog_cat"`
    RecentItems []ChatSnippet `gorm:"type:jsonb"` 
    MaxItems    int           `gorm:"default:5"`
    UpdatedAt   time.Time
}

// L2: 반려견 동적 요약 스냅샷
type DogDynamicSummary struct {
    DogID             string            `gorm:"primaryKey;type:uuid"`
    CategoryStatuses  []CategoryStatus  `gorm:"type:jsonb"`
    RapportNotes      string            `gorm:"type:text"` // 보호자와의 친밀도 메모
    LastUpdated       time.Time
}
```

## 4. 에이전트 페르소나 및 긴급도

### 4.1 강형욱 페르소나

- 스타일: 단호하고 따뜻한 어조. 과학적 근거 제시.
- 자기 의심(Self-Correction): L1과 L2 정보가 충돌할 경우 사용자에게 직접 질문하여 상태를 확정하는 액티브 베리피케이션 수행.

### 4.2 긴급도 판단 (4단계)

- L1: 일상적 교육/놀이.
- L2: 주의 관찰 (가벼운 증상).
- L3: 진료 권고 (병원 예약 권장).
- L4: 응급 상황 (즉시 24시 응급실 내원 안내).

## 5. 구현 우선순위 (Claude Code 지침)

1. Go 프로젝트 구조 세팅: internal/adapter, internal/domain, internal/repository.
2. L1-L2 메모리 매니저 개발: ChatSnippet과 CategoryStatus를 활용한 데이터 압축 및 전이 로직 구현.
3. ADK 에이전트 정의: 강형욱 페르소나 및 구글 검색 툴 활성화.
4. 카카오톡 콜백 핸들러: 비동기 처리를 위한 스킬 서버 엔드포인트 구현.