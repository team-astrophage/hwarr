# 화르르(Hwarr) Landing Page - Feature Specification

> 소스 기준 커밋: `houston` branch
> 최종 갱신: 2026-04-06

---

## 1. 기능 개요

### 1.1 페이지 목적

화르르 서비스의 첫 진입점으로, 사용자에게 서비스의 정체성("익명 실시간 방화 지도")을 한눈에 전달하고, 핵심 실시간 통계(상황판), 지역별 방화 랭킹(오늘의 방화 지역 TOP 10)을 노출하여 호기심을 유발한 뒤, 하단 CTA 버튼을 통해 지도 페이지(`/map`)로 전환시키는 것이 목적이다.

### 1.2 URL 및 라우팅

| 항목 | 값 |
|---|---|
| URL path | `/` (root) |
| 라우터 | TanStack Router (`@tanstack/react-router`) |
| Route 정의 파일 | `client/src/routes/index.tsx` |
| Route 생성 | `createFileRoute('/')` |
| component | `LandingPage` (`client/src/features/landing/components/LandingPage.tsx`) |

---

## 2. 레이아웃 구조

### 2.1 전역 컨테이너 (`#root`)

- **소스**: `client/src/index.css` 66-72행
- `max-width: 430px` — 모바일 퍼스트, 최대 폭 430px
- `margin: 0 auto` — 수평 중앙 정렬
- `min-height: 100svh` — 최소 높이 viewport 전체
- `position: relative`
- `overflow-x: hidden`

### 2.2 페이지 루트 (`<div>`)

```
className="relative flex min-h-svh flex-col bg-[var(--color-bg-base)]"
```

- `min-height: 100svh`
- `display: flex`, `flex-direction: column`
- `position: relative`
- `background-color: var(--color-bg-base)` → `#121212`

### 2.3 `<main>` 영역

```
className="mt-5 flex flex-1 flex-col gap-8 px-5 pt-6 pb-[calc(7.75rem+env(safe-area-inset-bottom,0px))]"
```

| 속성 | Tailwind class | 환산값 |
|---|---|---|
| margin-top | `mt-5` | `1.25rem` (20px) |
| padding-left/right | `px-5` | `1.25rem` (20px) |
| padding-top | `pt-6` | `1.5rem` (24px) |
| padding-bottom | `pb-[calc(7.75rem+env(safe-area-inset-bottom,0px))]` | `7.75rem` (124px) + safe area inset bottom |
| flex | `flex-1` | `flex: 1 1 0%` |
| display | `flex flex-col` | 세로 정렬 |
| 섹션 간격 | `gap-8` | `2rem` (32px) |

### 2.4 섹션 순서

`<main>` 내부에서 위에서 아래로:

1. **Header** (`<Header />`) — `<main>` 바깥, 페이지 루트 `<div>` 직계 자식
2. **히어로 섹션** (`<section>`) — 제목 + 부제목
3. **상황판 섹션** (`<section>`) — 4개 StatCard grid
4. **오늘의 방화 지역 TOP 10** (`<RankingFeed />`)

**Footer** (CTA 버튼 영역)는 `<main>` 바깥, `<div>` 직계 자식으로 fixed 위치.

### 2.5 디자인 토큰 (CSS Custom Properties)

**소스**: `client/src/index.css` 4-44행

| 토큰 | 값 | 용도 |
|---|---|---|
| `--color-bg-base` | `#121212` | 페이지 배경 |
| `--color-bg-surface` | `#181818` | 카드/피드 아이템 배경 |
| `--color-bg-elevated` | `#1f1f1f` | hover 시 카드 배경 |
| `--color-bg-card` | `#252525` | 카드 대체 배경 |
| `--color-bg-card-alt` | `#272727` | 카드 대체 배경 2 |
| `--color-accent` | `#1ed760` | 브랜드 그린 (Spotify Green) |
| `--color-accent-border` | `#1db954` | accent border |
| `--color-text-base` | `#ffffff` | 기본 텍스트 |
| `--color-text-secondary` | `#b3b3b3` | 보조 텍스트 |
| `--color-text-bright` | `#cbcbcb` | 밝은 보조 텍스트 |
| `--color-text-max` | `#fdfdfd` | 최대 밝기 텍스트 |
| `--color-negative` | `#f3727f` | 부정/위험 |
| `--color-warning` | `#ffa42b` | 경고/주의 |
| `--color-info` | `#539df5` | 정보 |
| `--color-border` | `#4d4d4d` | 기본 border |
| `--color-border-light` | `#7c7c7c` | 밝은 border |
| `--color-separator` | `#b3b3b3` | 구분선 |
| `--shadow-heavy` | `rgba(0,0,0,0.5) 0px 8px 24px` | 강한 그림자 |
| `--shadow-medium` | `rgba(0,0,0,0.3) 0px 8px 8px` | 중간 그림자 |
| `--radius-badge` | `2px` | |
| `--radius-subtle` | `4px` | |
| `--radius-standard` | `6px` | |
| `--radius-comfortable` | `8px` | |
| `--radius-panel` | `16px` | |
| `--radius-pill-lg` | `500px` | |
| `--radius-pill` | `9999px` | pill 형태 |

### 2.6 글로벌 타이포그래피

- **소스**: `client/src/index.css` 53-63행
- `font-family: -apple-system, BlinkMacSystemFont, 'Helvetica Neue', helvetica, arial, 'Hiragino Sans', 'Hiragino Kaku Gothic ProN', Meiryo, sans-serif`
- `font-size: 16px` (base)
- `line-height: 1.4`
- `-webkit-font-smoothing: antialiased`
- `-moz-osx-font-smoothing: grayscale`

---

## 3. Header

**소스**: `client/src/components/Header.tsx`

랜딩 페이지에서는 `isMap = false`이므로 map 전용 스타일(`absolute`, `z-[1000]`)이 적용되지 않는다.

### 3.1 레이아웃

```
className="flex items-center justify-between px-5 h-14 bg-[var(--color-bg-base)]"
```

| 속성 | 값 |
|---|---|
| height | `h-14` → `3.5rem` (56px) |
| padding-left/right | `px-5` → `1.25rem` (20px) |
| background | `var(--color-bg-base)` → `#121212` |
| display | `flex`, `align-items: center`, `justify-content: space-between` |

### 3.2 로고 (좌측)

```html
<Link to="/">
  <span className="text-[1.125rem] font-extrabold text-[var(--color-text-base)] leading-none tracking-tight">
    화르르
  </span>
</Link>
```

| 속성 | 값 |
|---|---|
| 텍스트 | `화르르` |
| font-size | `1.125rem` (18px) |
| font-weight | `extrabold` (800) |
| color | `var(--color-text-base)` → `#ffffff` |
| line-height | `1` (none) |
| letter-spacing | `tight` → `-0.025em` |
| link target | `/` |

### 3.3 연결 상태 인디케이터 (우측)

```
className="inline-flex shrink-0 items-center gap-1.5 rounded-full bg-[var(--color-bg-surface)] py-1 pl-1.5 pr-2.5 shadow-[var(--shadow-medium)]"
```

| 속성 | 값 |
|---|---|
| background | `var(--color-bg-surface)` → `#181818` |
| border-radius | `rounded-full` → `9999px` |
| padding | `py-1`(4px), `pl-1.5`(6px), `pr-2.5`(10px) |
| shadow | `var(--shadow-medium)` → `rgba(0,0,0,0.3) 0px 8px 8px` |
| gap | `1.5` → `0.375rem` (6px) |

**상태별 인디케이터**:

| WebSocket 상태 | label 텍스트 | 색상 CSS 변수 | pulse 애니메이션 |
|---|---|---|---|
| `connected` | `Live` | `--color-accent` (`#1ed760`) | O (ping) |
| `connecting` / `reconnecting` | `Reconnecting` | `--color-warning` (`#ffa42b`) | O (ping) |
| `offline` | `Offline` | `--color-negative` (`#f3727f`) | X |
| `idle` (기본값 포함) | `Idle` | `--color-text-secondary` (`#b3b3b3`) | X |

- 인디케이터 점(dot) 크기: `size-2` → `0.5rem` (8px), `rounded-full`
- pulse가 활성이면 `animate-ping` 클래스 적용 (Tailwind 기본: `animation: ping 1s cubic-bezier(0,0,0.2,1) infinite`)
- label 텍스트: `text-[0.6875rem]`(11px), `font-bold`(700), `uppercase`, `tracking-wide`(0.025em), `color: var(--color-text-secondary)` → `#b3b3b3`

---

## 4. 히어로 섹션

**소스**: `client/src/features/landing/components/LandingPage.tsx` 17-34행

### 4.1 섹션 컨테이너

```
className="pt-20 pb-2"
```

| 속성 | 값 |
|---|---|
| padding-top | `pt-20` → `5rem` (80px) |
| padding-bottom | `pb-2` → `0.5rem` (8px) |

### 4.2 메인 제목 (`<h1>`)

```
className="text-[2.375rem] font-bold text-[var(--color-text-max)] leading-[1.08] tracking-[-0.035em]"
style={{ textWrap: 'balance' }}
```

**렌더링 텍스트**:

```
지금, 이 자리에서
불을 질러라.
```

- 첫째 줄: `지금, 이 자리에서` (일반 텍스트)
- `<br />` 강제 줄바꿈
- 둘째 줄: `불을 질러라.` — `<span>` 으로 감싸 accent 색상 적용

| 속성 | 값 |
|---|---|
| font-size | `2.375rem` (38px) |
| font-weight | `bold` (700) |
| color (기본) | `var(--color-text-max)` → `#fdfdfd` |
| color (강조 span) | `var(--color-accent)` → `#1ed760` |
| line-height | `1.08` |
| letter-spacing | `-0.035em` |
| text-wrap | `balance` (CSS `text-wrap: balance`) |

### 4.3 부제목 (`<p>`)

```
className="mt-7 mb-1 max-w-[28ch] text-[0.9375rem] text-[var(--color-text-secondary)] leading-[1.65]"
style={{ textWrap: 'pretty' }}
```

**렌더링 텍스트**:

```
이 자리의 좌표가 출발점이에요.
지도 위에 익명으로 불을 올리고, 전국과 같은 맵을 실시간으로 함께 봅니다.
```

- 첫째 줄: `이 자리의 좌표가 출발점이에요.`
- `<br />` 강제 줄바꿈
- 둘째 줄: `지도 위에 익명으로 불을 올리고, 전국과 같은 맵을 실시간으로 함께 봅니다.`

| 속성 | 값 |
|---|---|
| margin-top | `mt-7` → `1.75rem` (28px) |
| margin-bottom | `mb-1` → `0.25rem` (4px) |
| max-width | `28ch` |
| font-size | `0.9375rem` (15px) |
| color | `var(--color-text-secondary)` → `#b3b3b3` |
| line-height | `1.65` |
| text-wrap | `pretty` |

---

## 5. 상황판 (Stats)

**소스**: `client/src/features/landing/components/LandingPage.tsx` 37-67행, `client/src/features/landing/components/StatCard.tsx`

### 5.1 섹션 컨테이너

```
className="flex flex-col gap-4"
```

- `gap: 1rem` (16px) — 섹션 제목과 카드 grid 사이 간격

### 5.2 섹션 제목

```html
<h2 className="text-[1rem] font-bold leading-none tracking-tight text-[var(--color-text-base)]">
  상황판
</h2>
```

| 속성 | 값 |
|---|---|
| 텍스트 | `상황판` |
| font-size | `1rem` (16px) |
| font-weight | `bold` (700) |
| line-height | `1` (none) |
| letter-spacing | `tight` → `-0.025em` |
| color | `var(--color-text-base)` → `#ffffff` |

### 5.3 카드 Grid 레이아웃

```
className="grid grid-cols-2 gap-2.5"
```

- `display: grid`
- `grid-template-columns: repeat(2, minmax(0, 1fr))` — 2열
- `gap: 0.625rem` (10px)

### 5.4 StatCard 4개 구성

| 순서 | label | value 필드 | unit | tone |
|---|---|---|---|---|
| 1 (좌상) | `실시간 화재 구역` | `stats.activeGrids` | `곳` | `accent` |
| 2 (우상) | `실시간 불타는 건수` | `stats.totalFires` | `건` | `warning` |
| 3 (좌하) | `오늘 방화 건수` | `stats.dailyFires` | `건` | `negative` |
| 4 (우하) | `누적 총 방화 건수` | `stats.cumulativeFires` | `건` | `base` |

### 5.5 StatCard 컴포넌트 상세

**소스**: `client/src/features/landing/components/StatCard.tsx`

#### Props 인터페이스

```typescript
interface StatCardProps {
  label: string
  value: number | undefined
  unit: string
  tone: 'accent' | 'warning' | 'negative' | 'base'
}
```

#### tone별 숫자 색상 매핑

| tone | CSS 변수 | Hex 값 |
|---|---|---|
| `accent` | `var(--color-accent)` | `#1ed760` |
| `warning` | `var(--color-warning)` | `#ffa42b` |
| `negative` | `var(--color-negative)` | `#f3727f` |
| `base` | `var(--color-text-base)` | `#ffffff` |

#### 카드 외부 컨테이너

```
className="min-w-0 rounded-[12px] bg-[var(--color-bg-surface)] p-4 shadow-[var(--shadow-medium)]"
```

| 속성 | 값 |
|---|---|
| min-width | `0` |
| border-radius | `12px` |
| background | `var(--color-bg-surface)` → `#181818` |
| padding | `p-4` → `1rem` (16px) 전방향 |
| box-shadow | `var(--shadow-medium)` → `rgba(0,0,0,0.3) 0px 8px 8px` |

#### label 텍스트

```
className="mb-3 text-[0.8125rem] font-medium leading-snug text-[var(--color-text-base)]"
```

| 속성 | 값 |
|---|---|
| margin-bottom | `mb-3` → `0.75rem` (12px) |
| font-size | `0.8125rem` (13px) |
| font-weight | `medium` (500) |
| line-height | `snug` → `1.375` |
| color | `var(--color-text-base)` → `#ffffff` |

#### 숫자 값 행

```
className="flex flex-wrap items-baseline gap-x-1 leading-none"
style={{ fontVariantNumeric: 'tabular-nums' }}
```

- `display: flex`, `flex-wrap: wrap`, `align-items: baseline`
- `column-gap: 0.25rem` (4px)
- `font-variant-numeric: tabular-nums` — 고정 폭 숫자

**숫자 (`displayValue`)**:

```
className="text-[1.5rem] font-bold"
style={{ color: TONE_COLOR[tone] }}
```

| 속성 | 값 |
|---|---|
| font-size | `1.5rem` (24px) |
| font-weight | `bold` (700) |
| color | tone에 따라 (위 표 참조) |
| 포맷 | `value.toLocaleString('ko-KR')` — 천 단위 쉼표 |

**단위 텍스트**:

```
className="text-[0.8125rem] font-medium text-[var(--color-text-base)]"
```

| 속성 | 값 |
|---|---|
| font-size | `0.8125rem` (13px) |
| font-weight | `medium` (500) |
| color | `var(--color-text-base)` → `#ffffff` |

#### 로딩 상태 (value === undefined)

- `displayValue`가 `'--'`로 표시
- `aria-label`이 `"{label} 불러오는 중"`으로 변경
- 별도의 skeleton/pulse 애니메이션은 없음. `'--'` 텍스트만 정적 렌더링

#### 에러 상태

- `useStats` 훅이 TanStack Query 기반이므로 별도의 에러 UI를 LandingPage에서 처리하지 않음
- `stats` 자체가 `undefined`이면 4개 카드 모두 `value={undefined}` → `'--'` 표시

### 5.6 API: useStats 훅

**소스**: `client/src/features/landing/api/useStats.ts`

#### 엔드포인트

```
GET {API_URL}/api/stats
```

#### 응답 인터페이스

```typescript
interface StatsData {
  activeGrids: number    // 실시간 화재 구역 수
  totalFires: number     // 실시간 불타는 건수
  cumulativeFires: number // 누적 총 방화 건수
  dailyFires: number     // 오늘 방화 건수
  onlineUsers: number    // 접속 유저 수 (현재 랜딩에서 미사용)
}
```

#### TanStack Query 설정

| 옵션 | 값 |
|---|---|
| `queryKey` | `['stats']` |
| `queryFn` | `fetchStats` (위 endpoint fetch) |
| `refetchInterval` | `10_000` (10초) |

- 10초마다 자동으로 재요청하여 실시간 수치 갱신
- 에러 발생 시 throw: `'통계 API 호출 실패'`

---

## 6. 속보 (News Feed)

> **주의**: `NewsFeed` 컴포넌트는 `client/src/features/landing/components/NewsFeed.tsx`에 구현되어 있으나, 현재 `LandingPage.tsx`에서 import 및 렌더링하고 있지 않다. 즉 **현재 랜딩 페이지에 표시되지 않는** 미사용(dormant) 컴포넌트이다. 아래는 컴포넌트 자체의 상세 명세이다.

**소스**: `client/src/features/landing/components/NewsFeed.tsx`, `client/src/features/landing/api/useNews.ts`, `server/routes/news.py`

### 6.1 SectionHeader

```html
<div className="flex items-center gap-2.5">
  <h2 className="text-[1rem] font-bold leading-none tracking-tight text-[var(--color-text-base)]">
    속보
  </h2>
  <span className="rounded-[var(--radius-pill)] bg-[rgba(255,68,68,0.12)] px-2.5 py-1 text-[0.6875rem] font-bold uppercase leading-none tracking-wide text-[#ff4444]">
    Live
  </span>
</div>
```

| 요소 | 속성 | 값 |
|---|---|---|
| 제목 텍스트 | 내용 | `속보` |
| | font-size | `1rem` (16px) |
| | font-weight | `bold` (700) |
| | color | `var(--color-text-base)` → `#ffffff` |
| Live 뱃지 | background | `rgba(255,68,68,0.12)` |
| | color | `#ff4444` |
| | font-size | `0.6875rem` (11px) |
| | font-weight | `bold` (700) |
| | text-transform | `uppercase` |
| | letter-spacing | `wide` → `0.025em` |
| | border-radius | `var(--radius-pill)` → `9999px` |
| | padding | `px-2.5`(10px) `py-1`(4px) |
| 컨테이너 gap | | `0.625rem` (10px) |

### 6.2 뉴스 아이템 구조

각 뉴스 아이템은 아래 구조:

```
┌─────────────────────────────────────────┐
│ [icon]  headline_parts (Headline)       │
│         time · detail                   │
└─────────────────────────────────────────┘
```

#### 아이템 컨테이너

```
className="flex gap-3 items-start rounded-[12px] bg-[var(--color-bg-surface)] p-4 transition-colors duration-150 hover:bg-[var(--color-bg-elevated)]"
```

| 속성 | 값 |
|---|---|
| display | `flex` |
| gap | `gap-3` → `0.75rem` (12px) |
| align-items | `flex-start` |
| border-radius | `12px` |
| background | `var(--color-bg-surface)` → `#181818` |
| hover background | `var(--color-bg-elevated)` → `#1f1f1f` |
| padding | `p-4` → `1rem` (16px) |
| transition | `background-color 150ms` |

#### 아이콘 영역

```
className="flex h-8 w-8 shrink-0 items-center justify-center rounded-[8px] text-[0.875rem]"
style={{ background: item.icon_bg }}
```

| 속성 | 값 |
|---|---|
| 크기 | `32px x 32px` |
| border-radius | `8px` |
| font-size | `0.875rem` (14px) — emoji 크기 |
| background | 서버에서 전달된 `icon_bg` (예: `rgba(255,68,68,0.15)`) |
| 내용 | 서버에서 전달된 `icon` (예: `🔥`) |
| flex-shrink | `0` |

#### Headline (제목)

```
className="text-[0.8125rem] font-medium leading-[1.5] text-[var(--color-text-base)]"
```

| 속성 | 값 |
|---|---|
| font-size | `0.8125rem` (13px) |
| font-weight | `medium` (500) |
| line-height | `1.5` |
| color (기본) | `var(--color-text-base)` → `#ffffff` |

`HeadlinePart[]`를 순회하며 렌더링. highlight에 따라:

| highlight 값 | 적용 class |
|---|---|
| `"accent"` | `text-[var(--color-accent)] font-bold` → 색상 `#1ed760`, 굵기 700 |
| `"warning"` | `text-[var(--color-warning)] font-bold` → 색상 `#ffa42b`, 굵기 700 |
| `null` | 스타일 없음 (부모 스타일 상속) |

#### 하단 상세 행

```
className="mt-1.5 flex items-center gap-1.5 text-[0.6875rem] text-[var(--color-text-secondary)]"
```

| 속성 | 값 |
|---|---|
| margin-top | `mt-1.5` → `0.375rem` (6px) |
| font-size | `0.6875rem` (11px) |
| color | `var(--color-text-secondary)` → `#b3b3b3` |
| gap | `0.375rem` (6px) |

구성: `{time}` · `{detail}`

- 구분자 `·`의 색상: `var(--color-border)` → `#4d4d4d`

### 6.3 서버 뉴스 생성 로직

**소스**: `server/routes/news.py`

#### 전체 흐름

1. `_get_engine()`으로 Fire engine (Redis 연결) 획득
2. `engine._get_active_grid_ids()`로 모든 활성 grid ID 조회
3. 각 grid에 대해 `engine._get_active_count(grid_id, now)`로 활성 방화 수 조회
4. `get_stage(count)`로 `FireStage` 결정
5. `_get_latest_ignite_ts(redis, grid_id)`로 최근 점화 시각 추정
6. `(stage, count)` 기준 내림차순 정렬
7. 상위 `MAX_NEWS_ITEMS`(5)개만 선택
8. 각 grid에 대해 stage에 따른 템플릿 적용

#### stage별 뉴스 템플릿

| 조건 | 빌더 함수 | icon | icon_bg | headline 패턴 |
|---|---|---|---|---|
| `stage.value >= 4` (대화재/전소) | `_build_big_fire` | `🔥` | `rgba(255,68,68,0.15)` | `{location}` **accent** + ` 일대 ` + `{stage_label}` **warning** + ` 발생` |
| `stage.value >= 2` (모닥불/화재) | `_build_fire_active` | `🔥` | `rgba(255,140,0,0.2)` | `{location}` **accent** + ` 반경 ` + `{stage_label}` **warning** + ` 확산 중` |
| `stage.value < 2` (불씨) | `_build_small_fire` | `🔥` | `rgba(255,140,0,0.12)` | `{location}` **accent** + ` 부근 ` + `불씨` **warning** + ` 감지` |

#### detail 텍스트

| 빌더 | detail 형식 |
|---|---|
| `_build_big_fire` | `활성 방화범 {count}명 · {현재stage_label}단계 지속 중` |
| `_build_fire_active` | `활성 방화범 {count}명` |
| `_build_small_fire` | `활성 방화범 {count}명` |

#### 시간 표시 (`_format_time_ago`)

| 조건 | 출력 |
|---|---|
| `< 60초` (minutes < 1) | `방금 전` |
| `1분 ~ 59분` | `{N}분 전` |
| `>= 60분` | `{N}시간 전` |

#### 위치 매핑 3단계 (`_get_location_name`)

grid ID → `grid_id_to_center(grid_id)` → `(lat, lng)` 좌표

**Tier 1 — Landmarks** (10개): 좁은 범위, 구체적 장소명

예시: `강남 테헤란로`, `서초 교대역`, `송파 잠실`, `종로 광화문`, `홍대입구`, `이태원`, `여의도`, `디지털단지`, `성수동`, `판교 테크노밸리`

**Tier 2 — Districts** (서울 25개 구 + 수도권 + 광역시 주요 구/군, 총 약 40개):

예시: `서울 종로구`, `서울 강남구`, `성남 분당구`, `부산 해운대구`, `대전 유성구` 등

**Tier 3 — Provinces** (18개 시/도 단위):

예시: `서울`, `인천`, `경기 북부`, `경기 남부`, `부산`, `대구`, `대전`, `광주`, `울산`, `세종`, `강원`, `충북`, `충남`, `전북`, `전남`, `경북`, `경남`, `제주`

**Fallback**: `N{lat:.1f}° E{lng:.1f}° 부근` (예: `N37.5° E127.0° 부근`)

매칭 로직: Tier 1 → Tier 2 → Tier 3 순서로 `lat` 범위와 `lng` 범위에 모두 들어가는 첫 번째 항목 반환. 모두 실패 시 fallback.

#### FireStage 참조

**소스**: `server/models/fire.py`

| Stage | enum 값 | label_ko | threshold (최소 active count) |
|---|---|---|---|
| `NONE` | 0 | `없음` | 0 |
| `BULSSSI` | 1 | `불씨` | 1 (1-9) |
| `MODAKBUL` | 2 | `모닥불` | 10 (10-39) |
| `HWAJAE` | 3 | `화재` | 40 (40-119) |
| `DAEHWAJAE` | 4 | `대화재` | 120 (120-279) |
| `JEONSO` | 5 | `전소` | 280+ |

#### 뉴스 아이템 ID 생성 규칙

| 빌더 | ID 패턴 |
|---|---|
| `_build_big_fire` | `news-bf-{grid_id}` |
| `_build_fire_active` | `news-fa-{grid_id}` |
| `_build_small_fire` | `news-sf-{grid_id}` |

#### 뉴스 TTL 설정

- `NEWS_TTL_SEC`: 환경변수 `NEWS_TTL_SEC`에서 읽으며 기본값 `86400` (1일)
- 소스: `server/config.py` — `NEWS_TTL_SEC = int(os.getenv("NEWS_TTL_SEC", 86400))`
- 최근 점화 시각 추정: `expiry_score - NEWS_TTL_SEC`

### 6.4 API: useNews 훅

**소스**: `client/src/features/landing/api/useNews.ts`

#### 엔드포인트

```
GET {API_URL}/api/news
```

#### 응답 인터페이스

```typescript
interface HeadlinePart {
  text: string
  highlight: "accent" | "warning" | null
}

interface NewsItem {
  id: string
  icon: string              // emoji 문자열 (예: "🔥")
  icon_bg: string           // CSS rgba 문자열 (예: "rgba(255,68,68,0.15)")
  headline_parts: HeadlinePart[]
  time: string              // 한국어 시간 (예: "방금 전", "3분 전")
  detail: string            // 상세 정보 (예: "활성 방화범 5명")
}
```

#### TanStack Query 설정

| 옵션 | 값 |
|---|---|
| `queryKey` | `['news']` |
| `queryFn` | `fetchNews` |
| `refetchInterval` | `10_000` (10초) |

### 6.5 로딩 상태

3개의 skeleton placeholder:

```
className="h-[72px] animate-pulse rounded-[12px] bg-[var(--color-bg-surface)]"
```

| 속성 | 값 |
|---|---|
| 개수 | 3개 |
| height | `72px` |
| border-radius | `12px` |
| background | `var(--color-bg-surface)` → `#181818` |
| 애니메이션 | `animate-pulse` (Tailwind: `animation: pulse 2s cubic-bezier(0.4,0,0.6,1) infinite`) |
| 컨테이너 gap | `gap-3` → `0.75rem` (12px) |

### 6.6 빈 상태 (데이터 없음)

```html
<div className="rounded-[12px] bg-[var(--color-bg-surface)] p-4 text-center">
  <p className="text-[0.8125rem] text-[var(--color-text-secondary)]">
    현재 활성 화재가 없습니다
  </p>
</div>
```

| 속성 | 값 |
|---|---|
| 텍스트 | `현재 활성 화재가 없습니다` |
| font-size | `0.8125rem` (13px) |
| color | `var(--color-text-secondary)` → `#b3b3b3` |
| text-align | `center` |
| border-radius | `12px` |
| background | `var(--color-bg-surface)` → `#181818` |
| padding | `1rem` (16px) |

---

## 7. 오늘의 방화 지역 TOP 10 (Ranking)

**소스**: `client/src/features/landing/components/RankingFeed.tsx`, `client/src/features/landing/api/useRanking.ts`

### 7.1 SectionHeader

`NewsFeed`의 `SectionHeader`와 동일한 패턴.

```html
<div className="flex items-center gap-2.5">
  <h2 className="text-[1rem] font-bold leading-none tracking-tight text-[var(--color-text-base)]">
    오늘의 방화 지역 TOP 10
  </h2>
  <span className="rounded-[var(--radius-pill)] bg-[rgba(255,68,68,0.12)] px-2.5 py-1 text-[0.6875rem] font-bold uppercase leading-none tracking-wide text-[#ff4444]">
    Live
  </span>
</div>
```

- 제목 텍스트: `오늘의 방화 지역 TOP 10`
- Live 뱃지: 속보 섹션과 동일 스타일 (`#ff4444`, `rgba(255,68,68,0.12)` 배경)

### 7.2 RankRow 컴포넌트

```
┌──────────────────────────────────────┐
│ [순위badge]  지역명          NNN 회  │
└──────────────────────────────────────┘
```

#### 컨테이너

```
className="flex items-center gap-3 rounded-[12px] bg-[var(--color-bg-surface)] px-4 py-3"
aria-label="{rank}위 {region} {displayCount}회"
```

| 속성 | 값 |
|---|---|
| display | `flex`, `align-items: center` |
| gap | `gap-3` → `0.75rem` (12px) |
| border-radius | `12px` |
| background | `var(--color-bg-surface)` → `#181818` |
| padding | `px-4`(16px), `py-3`(12px) |
| `aria-label` | `{rank}위 {region} {displayCount}회` |

#### 순위 뱃지

```
className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full text-[0.75rem] font-bold"
style={{ background: TONE_BG[tone], color: rankBadgeColor }}
```

| 속성 | 값 |
|---|---|
| 크기 | `24px x 24px` |
| border-radius | `9999px` (원형) |
| font-size | `0.75rem` (12px) |
| font-weight | `bold` (700) |
| 내용 | 순위 번호 (1, 2, 3, ...) |

#### 순위별 뱃지 색상

| 순위 | tone | 뱃지 배경색 (`TONE_BG`) | 뱃지 텍스트색 | 숫자 색상 (`TONE_TEXT`) |
|---|---|---|---|---|
| **1위** | `warning` | `var(--color-warning)` → `#ffa42b` (금색) | `#000000` | `var(--color-warning)` → `#ffa42b` |
| **2위** | `accent` | `var(--color-accent)` → `#1ed760` (은색 대신 accent green) | `#000000` | `var(--color-accent)` → `#1ed760` |
| **3위** | `negative` | `var(--color-negative)` → `#f3727f` (동색 대신 negative pink) | `#000000` | `var(--color-negative)` → `#f3727f` |
| **4위~10위** | `muted` | `var(--color-bg-elevated)` → `#1f1f1f` | `var(--color-text-secondary)` → `#b3b3b3` | `var(--color-text-base)` → `#ffffff` |

> 참고: 코드에서 1위=`warning`, 2위=`accent`, 3위=`negative`로 매핑. 전통적인 금/은/동 색상이 아닌 서비스 고유 시맨틱 컬러 사용.

#### 지역명

```
className="min-w-0 flex-1 truncate text-[0.875rem] font-medium text-[var(--color-text-base)]"
```

| 속성 | 값 |
|---|---|
| font-size | `0.875rem` (14px) |
| font-weight | `medium` (500) |
| color | `var(--color-text-base)` → `#ffffff` |
| overflow | `truncate` (text-overflow: ellipsis) |
| flex | `1 1 0%` |
| min-width | `0` |

#### 건수 표시

```
className="flex items-baseline gap-0.5 leading-none"
style={{ fontVariantNumeric: 'tabular-nums' }}
```

**숫자**:

| 속성 | 값 |
|---|---|
| font-size | `0.9375rem` (15px) |
| font-weight | `bold` (700) |
| color | tone에 따라 (`TONE_TEXT` 참조) |
| 포맷 | `count.toLocaleString('ko-KR')` — 천 단위 쉼표 |
| font-variant-numeric | `tabular-nums` |

**단위 "회"**:

| 속성 | 값 |
|---|---|
| font-size | `0.75rem` (12px) |
| font-weight | `medium` (500) |
| color | `var(--color-text-secondary)` → `#b3b3b3` |

숫자와 단위 사이 gap: `gap-0.5` → `0.125rem` (2px)

### 7.3 리스트 레이아웃

```
className="flex flex-col gap-2"
```

- `gap: 0.5rem` (8px) — 각 RankRow 사이 간격

### 7.4 API: useRanking 훅

**소스**: `client/src/features/landing/api/useRanking.ts`

#### 엔드포인트

```
GET {API_URL}/api/ranking/today
```

#### 응답 인터페이스

```typescript
interface RankingItem {
  rank: number    // 1~10
  region: string  // 예: "서울 강남구"
  count: number   // 방화 건수
}

interface RankingResponse {
  date: string          // 날짜 (예: "2026-04-06")
  items: RankingItem[]  // 최대 10개
}
```

#### TanStack Query 설정

| 옵션 | 값 |
|---|---|
| `queryKey` | `['ranking', 'today']` |
| `queryFn` | `fetchRanking` |
| `refetchInterval` | `15_000` (15초) |

- **15초 간격** — stats의 10초와 의도적으로 어긋나게 설정 (서버 부하 분산)
- 에러 발생 시 throw: `'랭킹 API 호출 실패'`

### 7.5 로딩 상태

10개의 skeleton placeholder:

```
className="h-[46px] animate-pulse rounded-[12px] bg-[var(--color-bg-surface)]"
```

| 속성 | 값 |
|---|---|
| 개수 | 10개 |
| height | `46px` |
| border-radius | `12px` |
| background | `var(--color-bg-surface)` → `#181818` |
| 애니메이션 | `animate-pulse` |
| 컨테이너 gap | `gap-2` → `0.5rem` (8px) |

### 7.6 빈 상태 (데이터 없음)

```html
<div className="rounded-[12px] bg-[var(--color-bg-surface)] p-4 text-center">
  <p className="text-[0.8125rem] text-[var(--color-text-secondary)]">
    오늘은 아직 집계된 지역이 없습니다
  </p>
</div>
```

| 속성 | 값 |
|---|---|
| 텍스트 | `오늘은 아직 집계된 지역이 없습니다` |
| font-size | `0.8125rem` (13px) |
| color | `var(--color-text-secondary)` → `#b3b3b3` |
| text-align | `center` |
| border-radius | `12px` |
| background | `var(--color-bg-surface)` → `#181818` |
| padding | `1rem` (16px) |

---

## 8. CTA 버튼 (Footer)

**소스**: `client/src/features/landing/components/LandingPage.tsx` 73-102행

### 8.1 Footer 컨테이너 (외부)

```
className="pointer-events-none fixed bottom-0 left-1/2 z-[100] w-full max-w-[430px] -translate-x-1/2"
```

| 속성 | 값 |
|---|---|
| position | `fixed` |
| bottom | `0` |
| left | `50%`, `transform: translateX(-50%)` — 중앙 정렬 |
| z-index | `100` |
| width | `100%` |
| max-width | `430px` |
| pointer-events | `none` (자식에서 개별 활성화) |

### 8.2 Footer 내부 컨테이너

```
className="pointer-events-auto border-t border-[color-mix(in_srgb,var(--color-border)_35%,transparent)] bg-[color-mix(in_srgb,var(--color-bg-base)_92%,transparent)] px-5 pt-3 shadow-[0_-12px_40px_rgba(0,0,0,0.35)] backdrop-blur-md supports-[backdrop-filter]:bg-[color-mix(in_srgb,var(--color-bg-base)_88%,transparent)] pb-[max(0.75rem,env(safe-area-inset-bottom,0px))]"
```

| 속성 | 값 |
|---|---|
| pointer-events | `auto` |
| border-top | `1px solid color-mix(in srgb, var(--color-border) 35%, transparent)` — `#4d4d4d`의 35% 불투명도 |
| background (기본) | `color-mix(in srgb, var(--color-bg-base) 92%, transparent)` — `#121212`의 92% 불투명도 |
| background (backdrop-filter 지원 시) | `color-mix(in srgb, var(--color-bg-base) 88%, transparent)` — 88% 불투명도 (더 투명하게) |
| backdrop-filter | `blur(12px)` (`backdrop-blur-md`) |
| padding-left/right | `px-5` → `1.25rem` (20px) |
| padding-top | `pt-3` → `0.75rem` (12px) |
| padding-bottom | `max(0.75rem, env(safe-area-inset-bottom, 0px))` — safe area 대응 |
| box-shadow | `0 -12px 40px rgba(0,0,0,0.35)` — 위쪽으로 퍼지는 그림자 |

### 8.3 CTA 버튼 (`<Link>`)

```
className="flex h-14 w-full items-center justify-center rounded-[14px] bg-[var(--color-accent)] text-[0.9375rem] font-extrabold tracking-[1px] text-[#000000] shadow-[var(--shadow-medium)] transition-[transform] duration-150 active:scale-[0.96]"
```

| 속성 | 값 |
|---|---|
| 텍스트 | `실시간 지도에서 불 지르기` |
| link target | `/map` |
| height | `h-14` → `3.5rem` (56px) |
| width | `w-full` (100%) |
| border-radius | `14px` |
| background | `var(--color-accent)` → `#1ed760` |
| font-size | `0.9375rem` (15px) |
| font-weight | `extrabold` (800) |
| letter-spacing | `1px` |
| color | `#000000` (검정) |
| box-shadow | `var(--shadow-medium)` → `rgba(0,0,0,0.3) 0px 8px 8px` |
| transition | `transform 150ms` |
| active 효과 | `scale(0.96)` — 누르는 동안 4% 축소 |
| display | `flex`, `align-items: center`, `justify-content: center` |

### 8.4 부가 텍스트

```html
<p className="mt-2 text-center text-[0.6875rem] text-[var(--color-text-secondary)]"
   style={{ textWrap: 'pretty' }}>
  완전 익명 · 로그인 불필요
</p>
```

| 속성 | 값 |
|---|---|
| 텍스트 | `완전 익명 · 로그인 불필요` |
| margin-top | `mt-2` → `0.5rem` (8px) |
| font-size | `0.6875rem` (11px) |
| color | `var(--color-text-secondary)` → `#b3b3b3` |
| text-align | `center` |
| text-wrap | `pretty` |

---

## 9. 피드백 버튼

**소스**: `client/src/features/landing/components/LandingPage.tsx` 89-100행

### 9.1 위치

CTA 버튼 하단, 부가 텍스트(`완전 익명 · 로그인 불필요`) 아래.

```html
<div className="mt-1 flex justify-center">
  <FeedbackButton>
    {(open) => (
      <button onClick={open} className="text-[0.6875rem] text-[var(--color-text-secondary)] underline-offset-2 hover:underline">
        의견 보내기
      </button>
    )}
  </FeedbackButton>
</div>
```

### 9.2 스타일

| 속성 | 값 |
|---|---|
| 컨테이너 margin-top | `mt-1` → `0.25rem` (4px) |
| 컨테이너 정렬 | `flex justify-center` |
| 버튼 텍스트 | `의견 보내기` |
| font-size | `0.6875rem` (11px) |
| color | `var(--color-text-secondary)` → `#b3b3b3` |
| underline-offset | `2px` |
| hover 효과 | `text-decoration: underline` |

### 9.3 동작

- `FeedbackButton`은 render props 패턴 사용 (`children: (open: () => void) => ReactNode`)
- 클릭 시 `open()` 호출 → 피드백 모달 오픈
- 모달은 `<FeedbackButton>` 내부에서 portal로 렌더링 (clipped container 회피)
- 소스: `client/src/features/feedback/components/FeedbackButton.tsx`

---

## 10. 접근성 (Accessibility)

### 10.1 aria-label

| 컴포넌트 | 요소 | aria-label 패턴 |
|---|---|---|
| `StatCard` | 숫자 행 `<p>` | 값 있음: `"{label} {displayValue}{unit}"` (예: `"실시간 화재 구역 1,234곳"`) |
| `StatCard` | 숫자 행 `<p>` | 값 없음: `"{label} 불러오는 중"` (예: `"실시간 화재 구역 불러오는 중"`) |
| `RankRow` | 행 `<div>` | `"{rank}위 {region} {displayCount}회"` (예: `"1위 서울 강남구 1,234회"`) |

### 10.2 시맨틱 HTML

- `<header>` — 페이지 헤더
- `<main>` — 주요 콘텐츠
- `<footer>` — CTA 영역
- `<section>` — 각 콘텐츠 섹션 (히어로, 상황판, 랭킹)
- `<h1>` — 히어로 메인 제목
- `<h2>` — 각 섹션 제목 (`상황판`, `오늘의 방화 지역 TOP 10`)
- `<nav>` — 없음 (단일 페이지 랜딩이므로)

### 10.3 키보드/포커스

- CTA 버튼: `<Link>` (TanStack Router) → 내부적으로 `<a>` 태그, 키보드 포커스 가능
- 피드백 버튼: `<button>` 태그, 키보드 포커스 가능
- 로고: `<Link>` → `<a>` 태그, 키보드 포커스 가능

### 10.4 숫자 포맷

- 모든 숫자 값에 `toLocaleString('ko-KR')` 적용 — 천 단위 쉼표
- `fontVariantNumeric: 'tabular-nums'` — 숫자 고정 폭으로 정렬 안정성 확보

---

## 부록: API 리페치 간격 요약

| 훅 | 엔드포인트 | refetchInterval | queryKey |
|---|---|---|---|
| `useStats` | `GET /api/stats` | 10초 | `['stats']` |
| `useNews` | `GET /api/news` | 10초 | `['news']` |
| `useRanking` | `GET /api/ranking/today` | 15초 | `['ranking', 'today']` |

## 부록: 파일 구조 요약

```
client/src/
├── routes/
│   └── index.tsx                          # Route 정의 (/ → LandingPage)
├── components/
│   └── Header.tsx                         # 공통 Header 컴포넌트
├── features/
│   ├── landing/
│   │   ├── api/
│   │   │   ├── useStats.ts               # 통계 API 훅
│   │   │   ├── useNews.ts                # 속보 API 훅
│   │   │   └── useRanking.ts             # 랭킹 API 훅
│   │   └── components/
│   │       ├── LandingPage.tsx            # 페이지 루트 컴포넌트
│   │       ├── StatCard.tsx               # 상황판 카드
│   │       ├── NewsFeed.tsx               # 속보 피드 (미사용)
│   │       └── RankingFeed.tsx            # 랭킹 피드
│   └── feedback/
│       └── components/
│           └── FeedbackButton.tsx         # 피드백 모달 트리거
├── index.css                              # 디자인 토큰 + 전역 스타일
└── lib/
    └── config.ts                          # API_URL 등 설정

server/
├── routes/
│   └── news.py                            # 속보 뉴스 API 엔드포인트
├── models/
│   └── fire.py                            # FireStage, STAGE_CONFIGS
└── config.py                              # NEWS_TTL_SEC 등 설정
```
