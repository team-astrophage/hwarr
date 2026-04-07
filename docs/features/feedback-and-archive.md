# 화르르(hwarr) 기능 명세서: 피드백 & 아카이브

---

# Part 1: 피드백 기능

## 1. 기능 개요

피드백 기능은 사용자가 버그 제보, 기능 제안, 기타 의견을 **익명으로** 개발팀에 전달할 수 있는 시스템이다. 사용자가 작성한 피드백은 서버를 경유하여 **Discord webhook**으로 개발 채널에 embed 형태로 전달된다. 별도의 로그인이나 인증 절차 없이 누구나 사용할 수 있으며, IP 기반 rate limiting으로 남용을 방지한다. 선택적으로 답변받을 이메일을 첨부할 수 있다.

**관련 소스 파일:**

| 파일 | 경로 |
|---|---|
| FeedbackButton | `client/src/features/feedback/components/FeedbackButton.tsx` |
| FeedbackModal | `client/src/features/feedback/components/FeedbackModal.tsx` |
| useFeedbackSubmit | `client/src/features/feedback/api/useFeedbackSubmit.ts` |
| types | `client/src/features/feedback/types.ts` |
| 서버 핸들러 | `server/internal/handler/feedback.go` |
| 서버 설정 | `server/internal/config/config.go` |

---

## 2. 피드백 버튼 (FeedbackButton.tsx)

### 2.1 컴포넌트 구조

`FeedbackButton`은 render prop 패턴을 지원하는 래퍼 컴포넌트이다. `children` prop이 없으면 기본 원형 버튼을 렌더링하고, `children`이 있으면 `open` 핸들러를 인자로 넘겨 호출자가 트리거 UI를 자유롭게 구성할 수 있다.

```ts
interface FeedbackButtonProps {
  children?: (open: () => void) => React.ReactNode
}
```

### 2.2 사용 위치

#### 맵 페이지 (`client/src/features/fire-map/components/MapPage.tsx`)

- **위치**: `absolute bottom-[244px] right-4 z-[1000]` -- 맵 우하단 플로팅 버튼
- **렌더링 조건**: `chatOpen === false`일 때만 표시 (채팅 패널이 열려 있으면 숨김)
- **트리거**: `children` 없이 사용 -> 기본 원형 버튼 렌더링

#### 랜딩 페이지 (`client/src/features/landing/components/LandingPage.tsx`)

- **위치**: footer 영역, "완전 익명 - 로그인 불필요" 텍스트 바로 아래 중앙 정렬
- **트리거**: `children` render prop 사용 -> 텍스트 링크 스타일 커스텀 버튼
  - 텍스트: `"의견 보내기"`
  - 스타일: `text-[0.6875rem] text-[var(--color-text-secondary)] underline-offset-2 hover:underline`

### 2.3 기본 버튼 (children 미제공 시)

| 속성 | 값 |
|---|---|
| 크기 | `w-12 h-12` (48x48px) |
| 배경색 | `bg-[var(--color-bg-surface)]` |
| 모양 | `rounded-full` (완전 원형) |
| 그림자 | `shadow-[var(--shadow-heavy)]` |
| aria-label | `"의견 보내기"` |

#### 아이콘

- SVG 봉투(envelope) 아이콘
- 크기: `width="20" height="20"`, viewBox `0 0 24 24`
- `fill="none"`, `stroke="var(--color-text-secondary)"`, `strokeWidth="2"`
- `strokeLinecap="round"`, `strokeLinejoin="round"`
- 구성: 사각형 봉투 body (`<path>`) + 봉투 뚜껑 접힘선 (`<polyline points="22,6 12,13 2,6">`)

#### hover 효과

- `hover:bg-[var(--color-bg-elevated)]` -- 배경색이 elevated 레벨로 밝아짐

#### active 효과

- `active:scale-90` -- 클릭 시 90%로 축소 (눌림 효과)
- `transition-transform` -- 크기 변화에 transition 적용

### 2.4 상태 관리

- `useState(false)` -- 모달 open/close 상태
- `open === true`일 때 `<FeedbackModal onClose={() => setOpen(false)} />` 렌더링

---

## 3. 피드백 모달 (FeedbackModal.tsx)

### 3.1 Portal 렌더링

- `createPortal(...)` 을 사용하여 **`document.body`** 에 직접 렌더링
- 목적: 부모 요소에 `transform`, `filter`, `backdrop-filter` 등이 설정되어 있으면 `position: fixed`가 해당 요소 기준으로 동작하는 CSS 스펙 이슈를 우회하기 위함 (특히 랜딩 페이지 footer의 `backdrop-filter`)
- SSR 안전성: `typeof document === 'undefined'`이면 `null` 반환

### 3.2 Backdrop

| 속성 | 값 |
|---|---|
| 포지션 | `fixed inset-0` |
| z-index | `z-[2000]` |
| 배경색 | `bg-black/60` (검정 60% 불투명도) |
| 블러 | `backdrop-blur-sm` (Tailwind 기본 `4px` blur) |
| 정렬 | `flex items-start justify-center` |
| 상단 여백 | `pt-[20vh]` (뷰포트 높이의 20%) |
| 좌우 여백 | `px-4` |

- **Backdrop 클릭 시**: `requestClose()` 호출 (미저장 경고 포함)

### 3.3 모달 컨테이너

| 속성 | 값 |
|---|---|
| 배경색 | `bg-[var(--color-bg-surface)]` |
| 모서리 반경 | `rounded-[16px]` |
| 그림자 | `shadow-[var(--shadow-heavy)]` |
| 너비 | `w-full max-w-[360px]` |
| padding | `px-5 py-5` (좌우 20px, 상하 20px) |
| 레이아웃 | `flex flex-col` |

- **모달 내부 클릭**: `e.stopPropagation()` -- backdrop 클릭 이벤트 전파 차단

### 3.4 Body Scroll Lock

모달이 마운트되면 `document.body.style.overflow`를 `'hidden'`으로 설정하여 배경 스크롤을 잠근다. 언마운트 시 이전 값(`prev`)으로 복원한다.

```ts
useEffect(() => {
  const prev = document.body.style.overflow
  document.body.style.overflow = 'hidden'
  return () => { document.body.style.overflow = prev }
}, [])
```

### 3.5 키보드 제어

- **ESC 키**: `window.keydown` 이벤트 리스너에서 `e.key === 'Escape'`일 때 `requestClose()` 호출
- 의존성 배열에 `[message]`를 포함하여 `requestClose` 클로저가 최신 `message` 값을 참조하도록 보장

### 3.6 모달 헤더

- 제목: `"의견 보내기"` -- `text-[1.0625rem] font-bold text-[var(--color-text-base)]`
- 닫기 버튼: 우측 정렬
  - 크기: `w-8 h-8`
  - 모양: `rounded-full`
  - aria-label: `"닫기"`
  - hover: `hover:bg-[var(--color-bg-elevated)]`
  - transition: `transition-colors`
  - 아이콘: X 표시 SVG (`width="16" height="16"`, `strokeWidth="2.5"`)

---

## 4. 카테고리 선택

### 4.1 카테고리 목록

| value | label | icon |
|---|---|---|
| `'bug'` | `"버그"` | `"🐛"` |
| `'idea'` | `"제안"` | `"💡"` |
| `'etc'` | `"기타"` | `"💬"` |

TypeScript 타입: `FeedbackCategory = 'bug' | 'idea' | 'etc'` (`client/src/features/feedback/types.ts`)

### 4.2 레이아웃

- 컨테이너: `flex gap-2 mb-3`
- 각 칩: `flex-1` (3등분 균등 배분)

### 4.3 칩 스타일

| 속성 | 값 |
|---|---|
| 높이 | `h-9` (36px) |
| 모서리 반경 | `rounded-[10px]` |
| 폰트 크기 | `text-[0.8125rem]` (13px) |
| 폰트 굵기 | `font-bold` |
| transition | `transition-colors` |

### 4.4 활성/비활성 스타일

| 상태 | 배경색 | 텍스트 색상 |
|---|---|---|
| **활성** (`active`) | `bg-[var(--color-accent)]` | `text-black` |
| **비활성** | `bg-[var(--color-bg-elevated)]` | `text-[var(--color-text-secondary)]` |

### 4.5 아이콘 배치

- 이모지 아이콘은 `<span className='mr-1' aria-hidden='true'>` 안에 렌더링
- `aria-hidden='true'` -- 스크린리더에서 이모지를 읽지 않도록 처리

### 4.6 기본값

- 초기 선택 카테고리: `'bug'`

---

## 5. 메시지 입력

### 5.1 textarea 속성

| 속성 | 값 |
|---|---|
| rows | `5` |
| placeholder | `"무엇이든 자유롭게 적어주세요"` |
| 최대 글자 수 | `500` (`FEEDBACK_MAX_LEN` 상수, `types.ts`에 정의) |
| resize | `resize-none` (크기 조절 불가) |

### 5.2 스타일

| 속성 | 값 |
|---|---|
| 너비 | `w-full` |
| 배경색 | `bg-[var(--color-bg-elevated)]` |
| 텍스트 색상 | `text-[var(--color-text-base)]` |
| 모서리 반경 | `rounded-[12px]` |
| padding | `px-3 py-3` |
| 폰트 크기 | `text-[0.9375rem]` (15px) |
| 줄 높이 | `leading-relaxed` |
| outline | `outline-none` |
| placeholder 색상 | `placeholder:text-[var(--color-text-secondary)]` |
| focus 링 | `focus:ring-2 focus:ring-[var(--color-accent)]/50` (accent 색상 50% 불투명도) |

### 5.3 Autofocus

- 모달 마운트 시 `textareaRef.current?.focus()`로 자동 포커스

### 5.4 글자 수 카운터

| 조건 | 동작 |
|---|---|
| `message.length >= 400` | 카운터 표시 (`{message.length}/{FEEDBACK_MAX_LEN}`) |
| `message.length < 400` | 카운터 숨김 |
| `message.length > 500` (`over === true`) | 경고 색상: `text-[var(--color-negative)]` (빨간색) |
| `message.length <= 500` | 일반 색상: `text-[var(--color-text-secondary)]` |

- 카운터 위치: textarea 바로 아래, 우측 정렬
- 폰트: `text-[0.75rem] font-medium` (12px)
- 카운터 영역 최소 높이: `min-h-[18px]` (카운터 유무와 관계없이 레이아웃 안정)

### 5.5 에러 메시지 표시

- 카운터와 동일 줄 좌측에 에러 메시지 표시
- `submit.isError === true`일 때 `(submit.error as Error).message` 렌더링
- 스타일: `text-[0.75rem] text-[var(--color-negative)]`

---

## 6. 이메일 입력 (선택)

### 6.1 접힘 구조

HTML `<details>` / `<summary>` 요소를 사용한 네이티브 접힘(accordion) 구현.

- 컨테이너: `<details className='mb-4 group'>` -- Tailwind `group` 유틸리티로 자식 요소 상태 연동
- `<summary>`: `cursor-pointer list-none` (기본 삼각형 마커 제거)

### 6.2 summary 텍스트

- 텍스트: `"답변받을 이메일 (선택)"`
- 스타일: `text-[0.8125rem] text-[var(--color-text-secondary)]`
- 레이아웃: `flex items-center gap-1 select-none`

### 6.3 Chevron 아이콘 및 회전 애니메이션

- SVG chevron-right 아이콘 (`<path d='M9 18l6-6-6-6' />`)
- 크기: `width="12" height="12"`, viewBox `0 0 24 24`
- `stroke="currentColor"`, `strokeWidth="2.5"`, `strokeLinecap="round"`, `strokeLinejoin="round"`
- **접힘 시**: 기본 방향 (우측 pointing, 0deg)
- **펼침 시**: `group-open:rotate-90` -- `<details>` 요소에 `open` 속성이 추가되면 Tailwind `group-open` variant가 활성화되어 90도 회전
- transition: `transition-transform` (부드러운 회전 애니메이션)

### 6.4 이메일 input 필드

| 속성 | 값 |
|---|---|
| type | `email` |
| placeholder | `"you@example.com"` |
| 상단 여백 | `mt-2` |
| 너비 | `w-full` |
| 배경색 | `bg-[var(--color-bg-elevated)]` |
| 텍스트 색상 | `text-[var(--color-text-base)]` |
| 모서리 반경 | `rounded-[10px]` |
| padding | `px-3 py-2.5` |
| 폰트 크기 | `text-[0.875rem]` (14px) |
| outline | `outline-none` |
| placeholder 색상 | `placeholder:text-[var(--color-text-secondary)]` |
| focus 링 | `focus:ring-2 focus:ring-[var(--color-accent)]/50` |

---

## 7. 전송 버튼

### 7.1 텍스트

| 상태 | 텍스트 |
|---|---|
| 기본 | `"보내기"` |
| 전송 중 (`submit.isPending === true`) | `"보내는 중..."` |

### 7.2 스타일

| 속성 | 값 |
|---|---|
| 너비 | `w-full` |
| 배경색 | `bg-[var(--color-accent)]` |
| 텍스트 색상 | `text-black` |
| 모서리 반경 | `rounded-[12px]` |
| padding | `py-3` |
| 폰트 크기 | `text-[0.9375rem]` (15px) |
| 폰트 굵기 | `font-bold` |
| active 효과 | `active:scale-[0.98]` (클릭 시 98%로 미세 축소) |
| transition | `transition-transform` |

### 7.3 disabled 조건

`canSubmit`이 `false`이면 버튼이 비활성화된다. `canSubmit`은 다음 세 조건을 **모두** 만족해야 `true`:

1. `message.trim().length > 0` -- 메시지가 공백만이 아닌 실제 내용을 포함
2. `!over` -- 메시지 길이가 `FEEDBACK_MAX_LEN`(500자) 이하
3. `!submit.isPending` -- 현재 전송 진행 중이 아님

### 7.4 disabled 스타일

- `disabled:opacity-30` -- 비활성 시 30% 불투명도
- `disabled:active:scale-100` -- 비활성 시 클릭해도 축소 효과 없음

---

## 8. 성공 상태

### 8.1 전환 조건

`submit.mutate()` 호출 후 `onSuccess` 콜백에서 `setSent(true)` 실행 시 모달 내용이 성공 화면으로 교체된다.

### 8.2 성공 화면 구성

| 요소 | 값 | 스타일 |
|---|---|---|
| 이모지 | `"🔥"` | `text-5xl mb-3`, `aria-hidden='true'` |
| 주 텍스트 | `"의견 감사합니다"` | `text-[1.0625rem] font-bold text-[var(--color-text-base)]` |
| 보조 텍스트 | `"개발자에게 전달됐어요"` | `text-[0.8125rem] text-[var(--color-text-secondary)] mt-1` |

- 레이아웃: `py-10 flex flex-col items-center text-center`

### 8.3 자동 닫힘

- `sent === true`가 되면 `setTimeout(onClose, 1500)` 실행
- **1500ms (1.5초)** 후 모달 자동 닫힘
- cleanup: 컴포넌트 언마운트 시 `clearTimeout(t)`으로 타이머 해제

---

## 9. 미저장 경고

### 9.1 동작 조건

`requestClose()` 함수가 호출될 때 다음 두 조건을 **모두** 만족하면 `window.confirm` 대화상자 표시:

1. `message.trim().length > 0` -- 사용자가 내용을 작성한 상태
2. `!sent` -- 아직 전송이 완료되지 않은 상태

### 9.2 확인 메시지

```
작성 중인 내용이 사라집니다. 닫을까요?
```

### 9.3 동작

| 사용자 선택 | 결과 |
|---|---|
| **확인** | `onClose()` 호출 -> 모달 닫힘 |
| **취소** | `return` -> 모달 유지 |

### 9.4 트리거 경로

`requestClose()`는 다음 세 가지 경로에서 호출된다:
1. **ESC 키** 입력
2. **Backdrop** (모달 외부 영역) 클릭
3. **닫기 버튼** (X) 클릭

---

## 10. API (useFeedbackSubmit.ts)

### 10.1 엔드포인트

```
POST {API_URL}/api/feedback
```

`API_URL`은 `@/lib/config`에서 import되는 환경 변수 기반 base URL이다.

### 10.2 요청 구조 (FeedbackRequest)

```ts
interface FeedbackRequest {
  category: FeedbackCategory    // 'bug' | 'idea' | 'etc'
  message: string               // 1~500자
  email?: string                // 선택, 최대 200자
  page?: string                 // 선택, 현재 페이지 경로 (window.location.pathname)
}
```

- `Content-Type: application/json`
- `email`은 `email.trim() || undefined` -- 빈 문자열이면 `undefined`로 전송하지 않음
- `page`는 `typeof window !== 'undefined' ? window.location.pathname : undefined`

### 10.3 응답 구조

```ts
// 성공 시
{ ok: true }
```

서버 response model: `FeedbackResponse(ok: bool)`, status code `201`.

### 10.4 에러 코드별 메시지

| HTTP 상태 코드 | 클라이언트 에러 메시지 | 서버 detail |
|---|---|---|
| `429` | `"잠시 후 다시 시도해주세요"` | `"rate_limited"` |
| `422` | `"입력값을 확인해주세요"` | (validation error) |
| 기타 (`!res.ok`) | `"전송에 실패했어요"` | 다양함 |
| `502` | `"전송에 실패했어요"` | `"webhook_failed"` |
| `503` | `"전송에 실패했어요"` | `"feedback_disabled"` |

### 10.5 TanStack Query 통합

- `useMutation({ mutationFn: submitFeedback })` -- TanStack Query의 `useMutation` hook 사용
- 반환값에서 `isPending`, `isError`, `error`, `mutate` 등을 활용

---

## 11. 서버 처리 (feedback.go)

### 11.1 라우터 설정

- 엔드포인트: `POST /api/feedback`
- 성공 응답: `201 Created`

### 11.2 요청 검증

요청 body의 `category`는 `"bug"`, `"idea"`, `"etc"` 중 하나여야 하며, `message`는 1~500자 범위, `email`과 `page`는 선택적(최대 200자)이다. 검증 실패 시 `400 Bad Request`를 반환한다.

### 11.3 CloudFront IP 추출

실 서비스 환경에서는 CloudFront/ALB를 거치기 때문에 `request.client.host`가 프록시 IP가 된다. 실제 클라이언트 IP를 다음 로직으로 추출한다:

`X-Forwarded-For` 헤더에서 **첫 번째**(leftmost) 항목을 추출한다.

- `X-Forwarded-For` 헤더에서 **첫 번째**(leftmost) 항목을 추출 -- CloudFront가 viewer IP를 맨 앞에 추가하고, ALB가 자신의 hop을 뒤에 추가하는 구조

### 11.4 User-Agent 추출

CloudFront는 기본적으로 `User-Agent`를 `"Amazon CloudFront"`로 덮어쓴다. 실제 클라이언트 UA를 얻기 위해:

`cloudfront-viewer-user-agent` 헤더를 우선 사용하고, 없으면 `user-agent` 헤더로 fallback한다.

- `cloudfront-viewer-user-agent` 헤더는 CloudFront managed origin request policy에서 전달됨

### 11.5 UA 축약 함수

UA 최대 길이: 80자

- UA가 비어있거나 `"-"`이면 `"unknown client"` 반환
- 80자 이하면 그대로 사용
- 80자 초과 시 앞 79자 + `"..."` (말줄임표)

### 11.6 Rate Limiting

| 설정 | 값 | 비고 |
|---|---|---|
| 윈도우 | `600초` (10분) | `_RL_WINDOW_SEC = 600` |
| 최대 횟수 | `3회` (기본값) | `FEEDBACK_RATE_LIMIT_PER_10MIN`, 환경변수로 변경 가능 |
| 저장소 | Redis | 공유 Redis 클라이언트 사용 |
| 키 형식 | `feedback:rl:{ip}` | IP 기반 |

**동작 흐름:**

1. `redis.incr(key)` -- 카운터 증가 (키가 없으면 1로 생성)
2. `count == 1`이면 `redis.expire(key, 600)` -- TTL 설정 (최초 요청 시에만)
3. `count > FEEDBACK_RATE_LIMIT_PER_10MIN`이면 `429 Too Many Requests` 반환

**Soft fail:** Redis가 사용 불가능하면 rate limit을 건너뛰고 요청을 허용한다 (로그만 남김).

### 11.7 Discord Webhook 전달

#### embed 구성

| 카테고리 | 라벨 |
|---|---|
| `bug` | 🐛 버그 제보 |
| `idea` | 💡 기능 제안 |
| `etc` | 💬 기타 의견 |

#### embed 색상 (카테고리별)

| 카테고리 | 색상 hex | 의미 |
|---|---|---|
| `bug` | `0xF3727F` | negative red (빨간색 계열) |
| `idea` | `0x1ED760` | accent green (초록색 계열) |
| `etc` | `0xFFA42B` | warning orange (주황색 계열) |
| fallback | `0x888888` | 회색 |

#### embed 필드 구조

```json
{
  "author": {"name": "카테고리 라벨"},
  "description": "메시지 본문 (blockquote 형식)",
  "color": "카테고리별 색상",
  "fields": ["조건부 필드"],
  "footer": {"text": "IP · UA"},
  "timestamp": "UTC ISO 타임스탬프"
}
```

#### 조건부 fields

- `body.page`가 존재하면: `{"name": "페이지", "value": "`{body.page}`", "inline": True}`
- `body.email`이 존재하면: `{"name": "답변 이메일", "value": body.email, "inline": True}`
- 값이 없는 필드는 포함하지 않음 -- Discord embed에 빈 `-` 플레이스홀더가 나타나는 것을 방지

#### blockquote 변환

`_quote_lines()` 함수가 메시지의 각 줄 앞에 `> `를 추가하여 Discord blockquote 형식으로 변환:

각 줄 앞에 `> `를 추가하여 Discord blockquote 형식으로 변환한다.

#### payload 래핑

payload에 `allowed_mentions.parse`를 빈 배열로 설정하여 mention 차단한다.

- `allowed_mentions.parse`를 빈 배열로 설정하여 사용자 입력에 포함된 `@everyone`, `@here`, `<@userid>` 등의 mention이 실제로 작동하지 않도록 차단

#### 전송

- Discord webhook으로 비동기 전송
- 실패 시 `502 Bad Gateway` 반환

### 11.8 Webhook URL 미설정 시

- `FEEDBACK_DISCORD_WEBHOOK_URL`이 빈 문자열이면 `503 Service Unavailable` 반환
- 환경변수: `FEEDBACK_DISCORD_WEBHOOK_URL` (기본값: `""`)

### 11.9 로깅

성공 시 category, IP, message 길이, email 유무를 로그에 기록한다.

---
---

# Part 2: 아카이브 기능

## 1. 기능 개요 (현재 Mock UI)

아카이브 페이지는 화재 발생 이력을 **히트맵 캘린더**와 **지역 랭킹** 형태로 시각화하는 페이지이다. 현재는 **하드코딩된 Mock 데이터**로 구성되어 있으며, 백엔드 API 연동은 후순위로 예정되어 있다. 별도의 데이터 fetch 로직이나 API 호출은 존재하지 않는다.

**관련 소스 파일:**

| 파일 | 경로 |
|---|---|
| ArchivePage | `client/src/features/archive/components/ArchivePage.tsx` |

**컴포넌트 의존성:**

- `Header` 컴포넌트 (`client/src/components/Header.tsx`)를 상단에 렌더링

**페이지 레이아웃:**

- 컨테이너: `flex flex-col min-h-svh bg-[var(--color-bg-base)]`

---

## 2. 탭 선택 (일별/주별/월별)

### 2.1 탭 목록

```ts
const TABS = ['일별', '주별', '월별'] as const
```

| 인덱스 | 탭 이름 |
|---|---|
| 0 | `"일별"` |
| 1 | `"주별"` |
| 2 | `"월별"` |

### 2.2 기본 선택값

```ts
const [activeTab, setActiveTab] = useState<(typeof TABS)[number]>('일별')
```

- 초기 활성 탭: `"일별"`

### 2.3 탭 컨테이너 스타일

| 속성 | 값 |
|---|---|
| 레이아웃 | `flex gap-1` |
| 좌우 마진 | `mx-5` |
| 상단 마진 | `mt-4` |
| 배경색 | `bg-[var(--color-bg-surface)]` |
| 모서리 반경 | `rounded-[12px]` |
| 내부 padding | `p-1` |

### 2.4 개별 탭 버튼 스타일

| 속성 | 값 |
|---|---|
| 너비 | `flex-1` (균등 분할) |
| padding | `py-2.5` |
| 텍스트 정렬 | `text-center` |
| 폰트 크기 | `text-[0.8125rem]` (13px) |
| 폰트 굵기 | `font-bold` |
| 모서리 반경 | `rounded-[10px]` |
| transition | `transition-all` |
| 텍스트 변환 | `uppercase` |
| 자간 | `tracking-[1px]` |

### 2.5 활성/비활성 스타일

| 상태 | 배경색 | 텍스트 색상 |
|---|---|---|
| **활성** | `bg-[var(--color-bg-elevated)]` | `text-[var(--color-text-base)]` |
| **비활성** | (없음, 투명) | `text-[var(--color-text-secondary)]` |

### 2.6 현재 동작

탭 클릭 시 `activeTab` state가 변경되지만, 현재 Mock UI이므로 **모든 탭이 동일한 데이터를 표시**한다. 탭별 데이터 분기는 미구현 상태.

---

## 3. 오늘의 화재 현황 (3개 지표)

### 3.1 섹션 컨테이너

| 속성 | 값 |
|---|---|
| 좌우 마진 | `mx-5` |
| 상단 마진 | `mt-5` |
| 배경색 | `bg-[var(--color-bg-surface)]` |
| 모서리 반경 | `rounded-[20px]` |
| 그림자 | `shadow-[var(--shadow-medium)]` |
| padding | `p-5` |

### 3.2 섹션 제목

- 텍스트: `"오늘의 화재 현황"`
- 스타일: `text-[0.6875rem] font-bold text-[var(--color-text-secondary)] uppercase tracking-[1.5px] mb-4`

### 3.3 지표 그리드

- 레이아웃: `grid grid-cols-3 gap-3`
- 각 지표: `text-center`

### 3.4 개별 지표

| 지표 | Mock 값 | 숫자 색상 | 라벨 |
|---|---|---|---|
| 총 화재 | `1,842` | `text-[var(--color-warning)]` | `"총 화재"` |
| 방화범 | `247` | `text-[var(--color-accent)]` | `"방화범"` |
| 전소 달성 | `3` | `text-[var(--color-warning)]` | `"전소 달성"` |

### 3.5 숫자 스타일

| 속성 | 값 |
|---|---|
| 폰트 크기 | `text-[1.5rem]` (24px) |
| 폰트 굵기 | `font-bold` |
| 숫자 정렬 | `fontVariantNumeric: 'tabular-nums'` (inline style) |

### 3.6 라벨 스타일

| 속성 | 값 |
|---|---|
| 폰트 크기 | `text-[0.6875rem]` (11px) |
| 색상 | `text-[var(--color-text-secondary)]` |
| 상단 마진 | `mt-1` |

---

## 4. 히트맵 캘린더

### 4.1 섹션 위치

- 좌우 마진: `mx-5`
- 상단 마진: `mt-5`

### 4.2 제목

- 텍스트: `"2026년 4월"` (하드코딩)
- 스타일: `text-[1rem] font-bold text-[var(--color-text-base)] mb-3`

### 4.3 그리드 구조

- 레이아웃: `grid grid-cols-7 gap-[3px]` -- **7열** (일~토)

### 4.4 요일 라벨

```ts
const DAY_LABELS = ['일', '월', '화', '수', '목', '금', '토']
```

- 스타일: `text-[0.625rem] text-[var(--color-text-secondary)] text-center pb-1`
- 폰트 크기: `0.625rem` (10px)

### 4.5 히트맵 데이터 (Mock)

```ts
const HEATMAP_DATA = [
  // Week 1 (수~토, 앞 3칸은 빈칸)
  -1, -1, -1, 2, 3, 5, 4,
  // Week 2
  1, 2, 3, 4, 2, 5, 3,
  // Week 3
  1, 3, 2, 4, 5, 3, 2,
  // Week 4
  2, 4, 3, 1, 0, 0, 0,
]
```

- `level === -1`: 해당 월에 속하지 않는 날짜 (빈 셀) -> `bg-transparent`
- `level >= 0`: 유효한 날짜 셀 -> `cursor-pointer`

### 4.6 셀 스타일

| 속성 | 값 |
|---|---|
| 비율 | `aspect-square` (정사각형) |
| 모서리 반경 | `rounded-[4px]` |
| transition | `transition-transform` |

### 4.7 hover 효과

- `hover:scale-[1.15]` -- 마우스 오버 시 115%로 확대
- `transition-transform` 으로 부드러운 확대 효과

### 4.8 색상 레벨 (0-5)

```ts
const LEVEL_COLORS: Record<number, string> = {
  0: 'bg-[var(--color-bg-elevated)]',
  1: 'bg-[#3b1a1a]',
  2: 'bg-[#6b2020]',
  3: 'bg-[#a33030]',
  4: 'bg-[#e04040]',
  5: 'bg-[#ff6b35] shadow-[0_0_8px_rgba(255,107,53,0.4)]',
}
```

| 레벨 | 색상 | hex 값 | 비고 |
|---|---|---|---|
| 0 | 기본 배경 | `var(--color-bg-elevated)` | 화재 없음 |
| 1 | 어두운 적갈색 | `#3b1a1a` | 최소 화재 |
| 2 | 진한 적색 | `#6b2020` | 낮은 화재 |
| 3 | 중간 적색 | `#a33030` | 중간 화재 |
| 4 | 밝은 적색 | `#e04040` | 높은 화재 |
| 5 | 주황색 + glow | `#ff6b35` | 최대 화재, `shadow-[0_0_8px_rgba(255,107,53,0.4)]` glow 효과 |

- 레벨 5는 유일하게 **box-shadow glow** 효과가 있음: `0 0 8px rgba(255,107,53,0.4)` -- 주황색 빛 번짐

### 4.9 범례 (Legend)

- 위치: 히트맵 그리드 하단 우측 정렬
- 레이아웃: `flex items-center gap-1 justify-end mt-2.5`
- 텍스트 스타일: `text-[0.625rem] text-[var(--color-text-secondary)]` (10px)

| 요소 순서 | 내용 |
|---|---|
| 1 | `"적음"` (텍스트) |
| 2 | 레벨 0 색상 사각형 |
| 3 | 레벨 1 색상 사각형 |
| 4 | 레벨 2 색상 사각형 |
| 5 | 레벨 3 색상 사각형 |
| 6 | 레벨 4 색상 사각형 |
| 7 | 레벨 5 색상 사각형 |
| 8 | `"많음"` (텍스트) |

- 각 사각형 크기: `w-3 h-3` (12x12px)
- 모서리 반경: `rounded-[2px]`

---

## 5. 역대 TOP 화재 지역

### 5.1 섹션 위치

- 좌우 마진: `mx-5`
- 상단 마진: `mt-5`
- 하단 마진: `mb-8`

### 5.2 헤더

- 제목: `"역대 TOP 화재 지역"`
  - 스타일: `text-[0.875rem] font-bold text-[var(--color-text-base)]`
- 기간 배지: `"이번 주"`
  - 스타일: `text-[0.6875rem] text-[var(--color-text-secondary)] bg-[var(--color-bg-elevated)] px-3 py-1 rounded-[var(--radius-pill)]`
  - 위치: 헤더 우측 (`flex items-center justify-between`)

### 5.3 랭킹 리스트

- 레이아웃: `flex flex-col gap-1.5`

### 5.4 Mock 데이터

```ts
const TOP_LOCATIONS = [
  { rank: 1, name: '강남 테헤란로',   count: 2847, pct: 100 },
  { rank: 2, name: '구로 디지털단지', count: 2221, pct: 78  },
  { rank: 3, name: '판교 테크노밸리', count: 1850, pct: 65  },
  { rank: 4, name: '여의도 IFC',      count: 1480, pct: 52  },
  { rank: 5, name: '성수동 뚝섬',     count: 1167, pct: 41  },
]
```

- `pct`는 1위 대비 상대 퍼센트 (1위 = 100%)

### 5.5 개별 항목 스타일

| 속성 | 값 |
|---|---|
| 레이아웃 | `flex items-center gap-3` |
| padding | `p-3 px-4` |
| 배경색 | `bg-[var(--color-bg-surface)]` |
| 모서리 반경 | `rounded-[14px]` |
| hover | `hover:bg-[var(--color-bg-elevated)]` |
| transition | `transition-colors` |

### 5.6 순위별 색상

```ts
const RANK_COLORS: Record<number, string> = {
  1: 'text-[#ffd700]',  // 금색
  2: 'text-[#c0c0c0]',  // 은색
  3: 'text-[#cd7f32]',  // 동색
}
```

| 순위 | 색상 | hex 값 |
|---|---|---|
| 1위 | 금색 | `#ffd700` |
| 2위 | 은색 | `#c0c0c0` |
| 3위 | 동색 | `#cd7f32` |
| 4위 이하 | 기본 보조 텍스트 | `var(--color-text-secondary)` |

- 순위 숫자 스타일: `w-6 text-[1rem] font-bold text-center`
- `fontVariantNumeric: 'tabular-nums'` (inline style)

### 5.7 지역명

- 스타일: `text-[0.8125rem] font-semibold text-[var(--color-text-base)] mb-1`

### 5.8 막대 그래프

#### 트랙 (배경)

| 속성 | 값 |
|---|---|
| 높이 | `h-1.5` (6px) |
| 배경색 | `bg-[var(--color-bg-elevated)]` |
| 모서리 반경 | `rounded-[3px]` |
| overflow | `overflow-hidden` |

#### 바 (진행 표시)

| 속성 | 값 |
|---|---|
| 높이 | `h-full` |
| 모서리 반경 | `rounded-[3px]` |
| 너비 | `{loc.pct}%` (inline style) |
| 배경 gradient | `linear-gradient(90deg, #ff4444, #ff8c00)` |

- gradient: **좌측 빨간색** (`#ff4444`) -> **우측 주황색** (`#ff8c00`)

### 5.9 건수 표시

| 속성 | 값 |
|---|---|
| 폰트 크기 | `text-[0.875rem]` (14px) |
| 폰트 굵기 | `font-bold` |
| 색상 | `text-[var(--color-warning)]` |
| 최소 너비 | `min-w-[50px]` |
| 정렬 | `text-right` |
| 숫자 정렬 | `fontVariantNumeric: 'tabular-nums'` (inline style) |
| 포맷 | `{loc.count.toLocaleString()}건` (천 단위 쉼표 + "건" 접미사) |

---

*이 문서는 소스 코드 기반으로 작성되었으며, 모든 값은 해당 소스 파일에서 직접 추출한 것이다.*
