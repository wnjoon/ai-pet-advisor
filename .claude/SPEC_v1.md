# Project: My Canine Advisor (나의 반려견 전담 조언자)

본 프로젝트는 초보 반려인들을 위해 '강형욱 훈련사가 옆에 있는 것 같은' 개인화된 실시간 상담 서비스를 구축하는 것을 목표로 한다. 단순한 챗봇을 넘어, 특정 반려견의 생애 주기와 과거 이력을 완벽히 기억하는 에이전트를 구현한다.

---

## 1. 기술 스택 및 아키텍처

### 1.1 기술 스택

| 구분 | 선택 | 비고 |
| --- | --- | --- |
| 프론트엔드 | **Next.js (App Router)** | React 기반, Vercel 배포 |
| 백엔드 | **Next.js API Routes** | 별도 서버 없이 통합 |
| AI 모델 | **Claude Haiku 4.5** | tool-use 지원, 비용 효율적, 빠른 응답 |
| AI SDK | **Anthropic TypeScript SDK** | tool-use + 스트리밍 지원 |
| UI 라이브러리 | **shadcn/ui + Tailwind CSS** | 뉴모피즘 커스터마이징에 적합 |
| 패키지 매니저 | **pnpm** | 빠르고 디스크 효율적 |
| 배포 | **Vercel** | git push 자동 배포 |
| 데이터 저장 | **In-memory** (우선), DB 추가 가능 | 시간 남으면 영속성 추가 |

### 1.2 아키텍처 개요

```
[Browser] ←→ [Next.js App]
                ├── Pages (React + shadcn/ui)
                │   ├── 반려견 등록 페이지
                │   ├── 채팅 페이지 (SSE 스트리밍)
                │   └── 반려견 선택/전환 UI
                └── API Routes
                    ├── /api/chat (Claude API + Tool-use + SSE)
                    ├── /api/dogs (반려견 CRUD)
                    └── /api/logs (대화 로그 관리)
```

- **단일 프로젝트 구조**: 프론트엔드와 백엔드가 하나의 Next.js 프로젝트에 통합
- **SSE 스트리밍**: Claude 응답을 토큰 단위로 실시간 스트리밍하여 UX 향상
- **In-memory 저장소**: 서버 메모리에 Map/Object로 데이터 관리 (MVP)

---

## 2. 핵심 아키텍처: 계층적 메모리 시스템

Claude는 다음 세 가지 층위의 데이터를 관리하여 '장기 기억'을 시뮬레이션한다.

1. **Static Profile**: 반려견의 기본 정보 (이름, 종, 나이, 무게, 성별, 중성화 여부).
2. **Dynamic Summary**: 대화가 거듭될 때마다 업데이트되는 반려견의 현재 상태 요약.
3. **Categorized History**: 카테고리별로 분류된 대화 기록.

> **차별화 포인트**: 과거 대화를 기억하고 참조하는 '기억 메커니즘'과 Claude tool-use를 활용한 구조화된 데이터 관리가 핵심 데모 포인트이다.

---

## 3. 데이터 스키마 (Data Schema)

### 3.1 반려견 프로필 (Dog Profile)

```typescript
interface DogProfile {
  dog_id: string;          // UUID
  user_id: string;         // 세션 ID
  name: string;            // 이름
  breed: string;           // 견종 (검색+선택 UI로 입력)
  age: number;             // 나이 (개월 수)
  weight: number;          // 무게 (kg)
  gender: "male" | "female";  // 성별
  neutered: boolean;       // 중성화 여부
  created_at: string;      // ISO8601
}
```

### 3.2 동적 요약 (Dynamic Summary)

```typescript
interface DynamicSummary {
  dog_id: string;
  last_updated: string;    // ISO8601
  summary: string;         // AI가 생성한 현재 상태 요약
  recent_concerns: string[];  // 최근 주요 관심사
  behavior_patterns: string[];  // 관찰된 행동 패턴
}
```

### 3.3 대화 로그 (Chat Log)

```typescript
interface ChatLog {
  chat_id: string;         // UUID
  user_id: string;
  dog_id: string;
  chatTime: string;        // ISO8601
  category: Category;
  question: string;        // 사용자 질문 요약
  answer: string;          // AI 답변 요약
  messages: Message[];     // 전체 대화 내역
  metadata: {
    sentiment_score: number;   // 1-10, 사용자 불안도
    behavior_tags: string[];
    urgency_level: number;     // 1-5
    action_item: string;       // 보호자가 실천해야 할 핵심 지침
  };
}

type Category = "식사" | "교육" | "건강" | "기분" | "수면" | "사회화" | "환경";
```

---

## 4. 에이전트 도구 정의 (Claude Tool-use)

Claude Haiku 4.5가 기억을 관리하기 위해 호출하는 Tool 인터페이스는 다음과 같다. 채팅 UI에서 tool 실행 과정을 사용자에게 시각적으로 표시한다 (예: "프로필 조회 중...", "과거 기록 검색 중...").

### 4.1 get_dog_context(dog_id)

- **목적**: 특정 반려견의 프로필과 최근 요약 정보를 가져옴.
- **출력**: DogProfile + DynamicSummary
- **UI 표시**: "🐕 [이름]의 프로필을 확인하고 있어요..."

### 4.2 save_chat_log(log_data)

- **목적**: 대화가 끝난 후 구조화된 데이터를 저장.
- **기능**: 사용자의 감정(Sentiment)과 긴급도(Urgency)를 분석하여 함께 저장.
- **트리거**: 5분 미응답 시 자동 종료하며 요약 생성 및 저장.
- **UI 표시**: "📝 상담 내용을 정리하고 있어요..."

### 4.3 search_past_logs(dog_id, query)

- **목적**: 현재 고민과 유사한 과거 이력이 있는지 키워드 또는 카테고리 기반 검색.
- **출력**: 관련 ChatLog 배열
- **UI 표시**: "🔍 과거 상담 기록을 찾아보고 있어요..."

### 4.4 update_dynamic_summary(dog_id, summary_data)

- **목적**: 대화 종료 시 DynamicSummary를 갱신.
- **기능**: 새로운 대화 내용을 반영하여 요약 업데이트.
- **UI 표시**: "✨ [이름]의 상태를 업데이트하고 있어요..."

---

## 5. 카테고리 및 분석 가이드라인

에이전트는 모든 대화를 다음 카테고리 중 하나로 분류해야 한다.

| 카테고리 | 설명 |
| --- | --- |
| 식사 | 사료 거부, 간식 조절, 식탐, 구토 등 |
| 교육 | 입질, 짖음, 배변 훈련, 산책 매너 등 |
| 건강 | 질병 의심 증상, 활력 저하, 예방 접종 등 |
| 기분 | 꼬리 치기, 카밍 시그널, 무기력, 신남 등 |
| 수면 | 잠자리 위치, 수면 시간, 수면 중 행동 등 |
| 사회화 | 낯선 사람/강아지에 대한 반응, 사회성 부족 등 |
| 환경 | 이사, 가구 변경, 소음 문제, 새로운 가족 등 |

---

## 6. 페르소나: 강형욱 훈련사 스타일

시스템 프롬프트에 반영할 페르소나 특징:

- **단호하면서도 따뜻한 어조**: "보호자님, 이건 꼭 고쳐주셔야 해요" 같은 직접적이지만 배려 있는 표현.
- **과학적 근거 제시**: 행동학/수의학적 이유를 함께 설명. "개가 앞발을 핥는 건 스트레스 신호일 수 있어요."
- **보호자 교육 강조**: 강아지 문제가 아닌 보호자의 행동 변화를 유도. "강아지가 아니라 보호자님이 먼저 바뀌어야 해요."
- **비유적 표현 활용**: 이해하기 쉽게 비유로 설명. "산책할 때 줄을 당기는 건, 마치 아이가 편의점 앞에서 막 떼쓰는 거랑 같아요."
- **과거 이력 참조**: 반드시 과거 상담 내용을 언급하여 신뢰 형성. "지난번 식사 거부 때와는 다른 양상이네요."
- **긴급 상황 안내**: urgency_level 4 이상 시 텍스트 내에서 동물병원 방문을 강하게 권고.

---

## 7. 가동 로직 (Implementation Logic)

1. **Context Loading**: 대화 시작 시 `get_dog_context`를 호출하여 반려견의 기본 정보와 최근 요약을 로드한다.
2. **Persona Injection**: 강형욱 훈련사 페르소나를 시스템 프롬프트로 주입한다.
3. **Real-time Analysis**: 대화 중 발견된 특이점(예: "왼쪽 앞발을 자꾸 핥아요")은 즉시 `behavior_tags`에 추가한다.
4. **Past Reference**: `search_past_logs`로 관련 과거 대화를 찾아 응답에 반영한다.
5. **Tool-use 시각화**: tool 실행 시 사용자에게 상태를 표시한다 (예: "과거 기록을 찾아보고 있어요...").
6. **Auto Closing**: 5분 미응답 시 자동으로 대화를 종료하고 `save_chat_log`를 호출하며 `DynamicSummary`를 갱신한다.

---

## 8. UI/UX 설계

### 8.1 디자인 컨셉

- **따뜻한 뉴모피즘**: 부드러운 색상(오렌지/베이지), 둥근 카드, 그림자로 입체감 표현.
- shadcn/ui 컴포넌트를 뉴모피즘 스타일로 커스터마이징.

### 8.2 주요 화면

#### 반려견 등록 페이지
- 이름, 견종(검색+선택 드롭다운), 나이, 무게, 성별, 중성화 여부 입력
- 견종 목록: 한국에서 인기 있는 견종을 포함한 검색 가능한 드롭다운
- 다견 지원: 여러 마리 등록 가능, 목록에서 선택/전환

#### 채팅 페이지
- SSE 스트리밍으로 토큰 단위 실시간 응답 표시
- Tool-use 실행 시 상태 로그 표시 (예: "🔍 과거 상담 기록을 찾아보고 있어요...")
- 자동 종료: 5분 미응답 시 자동 요약 생성

#### 반려견 선택/전환
- 헤더 또는 사이드에서 현재 상담 중인 반려견 표시
- 다른 반려견으로 전환 가능

### 8.3 인증

- **세션 기반**: 브라우저 세션으로 사용자 구분. 로그인 없이 바로 사용 가능.
- 향후 소셜 로그인(NextAuth.js) 추가 가능한 구조로 설계.

---

## 9. 우선순위 및 구현 순서 (24시간 해커톤)

### 필수 (Must-have)

| 순서 | 기능 | 설명 |
| --- | --- | --- |
| 1 | 반려견 등록 | 프로필 입력 폼 + In-memory 저장 |
| 2 | 채팅 UI + 스트리밍 | Claude Haiku 연동, SSE 스트리밍 |
| 3 | Tool-use 메모리 시스템 | get_dog_context, save_chat_log, search_past_logs 구현 |
| 4 | 강형욱 페르소나 | 시스템 프롬프트 설계 및 적용 |

### 우선 (Should-have)

| 순서 | 기능 | 설명 |
| --- | --- | --- |
| 5 | Tool-use 시각화 UI | tool 실행 과정을 채팅 UI에 표시 |
| 6 | 다견 지원 | 여러 반려견 등록/선택/전환 |
| 7 | 견종 검색 UI | 검색 가능한 드롭다운으로 견종 선택 |

### 선택 (Nice-to-have)

| 기능 | 설명 |
| --- | --- |
| DB 영속성 | SQLite/Supabase로 데이터 영속성 추가 |
| 소셜 로그인 | NextAuth.js로 Google/Kakao 로그인 |

---

## 10. 단계별 목표 (Milestones)

- **Step 1**: Next.js 프로젝트 세팅 + In-memory 기반 반려견 등록 및 기본 대화 기능 구현.
- **Step 2**: Claude Haiku 4.5 Tool-use를 이용한 대화 내용 카테고리 자동 분류 + 메모리 시스템.
- **Step 3**: 과거 대화를 참조하여 답변에 반영하는 '기억 메커니즘' 증명 + tool-use 시각화.
- **Step 4**: (Optional) DB 영속성 추가 또는 Supabase 연동.

---

## 부록: 데모 시나리오

해커톤 심사를 위한 데모 흐름:

1. **반려견 등록**: "바둑이" (말티즈, 8개월, 3.5kg, 수컷, 미중성화) 등록
2. **첫 상담**: "바둑이가 밥을 안 먹어요" → AI가 프로필 조회 후 맞춤 상담
3. **두 번째 상담**: "바둑이가 산책할 때 줄을 당겨요" → AI가 과거 식사 거부 상담을 기억하고 언급
4. **기억 메커니즘 데모**: "지난번에 뭐라고 했었죠?" → AI가 과거 로그 검색 후 정확히 답변
5. **Tool-use 시각화**: 각 단계에서 tool 실행 상태가 UI에 표시됨
