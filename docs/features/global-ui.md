# Global UI 기능 명세

> 화르르(hwarr) 클라이언트 애플리케이션의 전역 UI 구성 요소에 대한 상세 기능 명세.
> 대상 파일: `App.tsx`, `SocketProvider.tsx`, `router.tsx`, `Header.tsx`, `index.css`, `config.ts`, `socketManager.ts`

---

## 1. 앱 구조

### 1.1 Provider 계층

`App.tsx`에서 다음 세 계층의 Provider가 중첩된다. 순서는 바깥에서 안쪽으로:

```
QueryClientProvider  →  SocketProvider  →  RouterProvider
```

| 계층 | 컴포넌트 | 역할 |
|------|---------|------|
| 1 (최외곽) | `QueryClientProvider` | TanStack React Query의 캐시 컨텍스트 제공 |
| 2 | `SocketProvider` | Socket.IO 연결을 앱 수명 동안 1회 초기화 및 유지 |
| 3 (최내곽) | `RouterProvider` | TanStack Router 기반 클라이언트 라우팅 |

이 순서는 의도적이다. 소켓 연결은 라우트 변경과 무관하게 앱이 살아있는 동안 유지되어야 하므로, `RouterProvider` 바깥에 배치한다. `QueryClientProvider`는 소켓 이벤트 핸들러 내에서 query invalidation을 수행할 수 있으므로 가장 바깥에 위치한다.

### 1.2 QueryClient 설정값

```ts
const queryClient = new QueryClient()
```

별도의 `defaultOptions`를 전달하지 않으므로, TanStack React Query v5의 기본값이 적용된다:

- `staleTime`: `0` (즉시 stale)
- `gcTime` (구 `cacheTime`): `5 * 60 * 1000` (5분)
- `retry`: `3`
- `refetchOnWindowFocus`: `true`
- `refetchOnReconnect`: `true`

---

## 2. 헤더 (Header.tsx)

`Header` 컴포넌트는 모든 페이지 상단에 표시되는 전역 네비게이션 바이다.

### 2.1 레이아웃

| 속성 | 값 | 비고 |
|------|-----|------|
| display | `flex` (`items-center justify-between`) | 좌우 정렬 |
| 높이 | `h-14` = `3.5rem` = `56px` | 고정 높이 |
| 좌우 패딩 | `px-5` = `1.25rem` = `20px` | |
| 상하 패딩 | 없음 (높이로 수직 정렬) | |

#### 맵 페이지 (`/map`) 조건부 스타일링

`useRouterState()`를 통해 현재 `location.pathname`을 읽고, `/map`인지 판별한다.

| 조건 | 적용 클래스 | 효과 |
|------|-----------|------|
| `pathname === '/map'` | `absolute top-0 left-0 right-0 z-[1000]` | 지도 위에 떠 있는 오버레이 헤더. 배경 투명. |
| 그 외 페이지 | `bg-[var(--color-bg-base)]` (`#121212`) | 불투명 어두운 배경, 하단 border 없음 |

맵 페이지에서는 `position: absolute`로 지도 콘텐츠 위에 겹쳐지며, `z-index: 1000`으로 Leaflet 지도 타일 위에 표시된다. 배경색이 지정되지 않으므로 지도가 헤더 뒤로 비쳐 보인다.

### 2.2 로고

```tsx
<Link to="/" className="min-w-0 shrink-0">
  <span className="text-[1.125rem] font-extrabold text-[var(--color-text-base)] leading-none tracking-tight">
    화르르
  </span>
</Link>
```

| 속성 | 값 | 비고 |
|------|-----|------|
| 텍스트 | `화르르` | 한글 브랜드명 |
| 링크 | `/` (홈) | TanStack Router `<Link>` 사용 |
| 폰트 크기 | `1.125rem` = `18px` | |
| 폰트 굵기 | `font-extrabold` = `800` | |
| 색상 | `var(--color-text-base)` = `#ffffff` | 흰색 |
| line-height | `leading-none` = `1` | |
| letter-spacing | `tracking-tight` = `-0.025em` | |
| overflow 방지 | `min-w-0 shrink-0` | flex 컨테이너 내 축소 방지 |

### 2.3 소켓 상태 표시기

헤더 우측에 위치하며, 실시간 소켓 연결 상태를 시각적으로 피드백한다.

#### 2.3.1 상태 매핑 (`toIndicator`)

`ConnectionStatus` 타입의 5가지 상태에 대해 각각 `label`, `colorVar`, `pulse` 값이 매핑된다:

| 상태 | label (맵) | label (기타) | CSS 변수 (`colorVar`) | 실제 색상값 | pulse 애니메이션 |
|------|------------|-------------|----------------------|------------|-----------------|
| `connected` | `실시간 접속자` | `Live` | `--color-accent` | `#1ed760` (녹색) | 활성 |
| `connecting` | `재연결 중` | `Reconnecting` | `--color-warning` | `#ffa42b` (주황) | 활성 |
| `reconnecting` | `재연결 중` | `Reconnecting` | `--color-warning` | `#ffa42b` (주황) | 활성 |
| `offline` | `오프라인` | `Offline` | `--color-negative` | `#f3727f` (붉은 분홍) | 비활성 |
| `idle` (기본값) | `대기` | `Idle` | `--color-text-secondary` | `#b3b3b3` (회색) | 비활성 |

`toIndicator(status, isMap)` 함수가 `isMap` 인자에 따라 한국어(맵)/영어(기타) label을 반환한다.

설계 의도: `connecting`과 `reconnecting`은 동일한 표시로 통합했다. 사용자에게 "연결 시도 중"이라는 하나의 의미만 전달하면 충분하기 때문이다. `offline`과 `idle`은 pulse를 끄는데, 이는 "죽은 점"으로 인지시켜 현재 통신이 없음을 명확히 한다.

#### 2.3.1a 온라인 접속자 수 (맵 페이지 전용)

맵 페이지(`isMap`)에서 `onlineUsers > 0`일 때, label 옆에 구분선(`w-px h-2.5 bg-[var(--color-border)]`)과 접속자 수를 추가 표시한다.

- **숫자 폰트**: `text-[0.6875rem] font-bold text-[var(--color-text-base)]`, `fontVariantNumeric: 'tabular-nums'`
- **데이터 소스**: `useFireStore((s) => s.onlineUsers)`
- 랜딩 페이지 등 맵이 아닌 페이지에서는 접속자 수를 표시하지 않는다.

#### 2.3.2 표시기 컨테이너 스타일

```
inline-flex shrink-0 items-center gap-1.5 rounded-full
bg-[var(--color-bg-surface)] py-1 pl-1.5 pr-2.5
shadow-[var(--shadow-medium)]
```

| 속성 | 값 | 비고 |
|------|-----|------|
| display | `inline-flex` + `items-center` | 인라인 수평 정렬 |
| 축소 방지 | `shrink-0` | flex 컨테이너 내 축소 불가 |
| dot-label 간격 | `gap-1.5` = `0.375rem` = `6px` | |
| 모서리 | `rounded-full` = `border-radius: 9999px` | 완전 pill 형태 |
| 배경색 | `var(--color-bg-surface)` = `#181818` | |
| 상하 패딩 | `py-1` = `0.25rem` = `4px` | |
| 좌측 패딩 | `pl-1.5` = `0.375rem` = `6px` | dot 쪽 여백 |
| 우측 패딩 | `pr-2.5` = `0.625rem` = `10px` | label 쪽 여백 (비대칭) |
| 그림자 | `var(--shadow-medium)` = `rgba(0,0,0,0.3) 0px 8px 8px` | |

#### 2.3.3 Ping Dot 애니메이션 상세

dot 영역은 `relative flex size-2 shrink-0` 컨테이너(`8px x 8px`) 안에 두 개의 `<span>`으로 구성된다.

**정적 dot (항상 표시):**

| 속성 | 값 |
|------|-----|
| 크기 | `size-2` = `0.5rem` = `8px` |
| 형태 | `rounded-full` = 원형 |
| 배경색 | `var(${indicator.colorVar})` (상태별 동적) |
| position | `relative` |

**Ping 애니메이션 레이어 (pulse=true 일 때만 렌더링):**

| 속성 | 값 |
|------|-----|
| 크기 | `size-full` = 부모와 동일 (`8px`) |
| position | `absolute` (정적 dot 위에 겹침) |
| 형태 | `rounded-full` = 원형 |
| 불투명도 | `opacity-75` = `0.75` |
| 배경색 | 정적 dot과 동일 |
| 애니메이션 | `animate-ping` (Tailwind 내장) |

`animate-ping`은 Tailwind CSS 기본 제공 애니메이션으로, 다음과 같이 동작한다:

```css
@keyframes ping {
  75%, 100% {
    transform: scale(2);
    opacity: 0;
  }
}
animation: ping 1s cubic-bezier(0, 0, 0.2, 1) infinite;
```

- 원본 크기에서 시작하여 2배로 확대되며 사라진다.
- `cubic-bezier(0, 0, 0.2, 1)` easing으로 빠르게 확장 후 천천히 소멸.
- `1s` 주기로 무한 반복.
- `opacity-75` 기본값에서 `opacity: 0`까지 fade-out.

`pulse=false`일 때는 ping 레이어가 렌더링되지 않아 정적인 "죽은 점"만 남는다.

#### 2.3.4 Label 텍스트 스타일

```
text-[0.6875rem] font-bold leading-none tracking-wide
text-[var(--color-text-secondary)]
```

| 속성 | 값 | 비고 |
|------|-----|------|
| 폰트 크기 | `0.6875rem` = `11px` | |
| 폰트 굵기 | `font-bold` = `700` | |
| line-height | `leading-none` = `1` | |
| letter-spacing | `tracking-wide` = `0.025em` | |
| 색상 | `var(--color-text-secondary)` = `#b3b3b3` | 상태와 무관하게 고정 |

---

## 3. 디자인 토큰 (index.css)

`@theme` 블록 내에 Tailwind CSS v4의 커스텀 테마 토큰으로 선언된다. 주석에 "Spotify DESIGN.md exact spec"으로 명시된 바와 같이, Spotify 디자인 시스템을 기반으로 한 어두운 테마이다.

### 3.1 배경 색상 시스템 (Background Surfaces)

계층적 elevation 시스템으로, 숫자가 높을수록 밝아진다:

| CSS 변수 | 값 | 용도 |
|----------|-----|------|
| `--color-bg-base` | `#121212` | 최하위 배경 (body, 헤더 비맵 페이지) |
| `--color-bg-surface` | `#181818` | 기본 표면 (소켓 표시기 pill 등) |
| `--color-bg-elevated` | `#1f1f1f` | 팝오버, 모달 등 부유 요소 |
| `--color-bg-card` | `#252525` | 카드 컴포넌트 |
| `--color-bg-card-alt` | `#272727` | 카드 대체 배경 (짝수/홀수 구분 등) |

### 3.2 브랜드 강조색 (Accent)

| CSS 변수 | 값 | 용도 |
|----------|-----|------|
| `--color-accent` | `#1ed760` | 주 강조색, 소켓 connected 표시 등 |
| `--color-accent-border` | `#1db954` | 강조 요소의 테두리 |

### 3.3 텍스트 색상 시스템

| CSS 변수 | 값 | 용도 |
|----------|-----|------|
| `--color-text-base` | `#ffffff` | 기본 텍스트 (로고, 제목 등) |
| `--color-text-secondary` | `#b3b3b3` | 보조 텍스트 (소켓 label, 부제목 등) |
| `--color-text-bright` | `#cbcbcb` | base와 secondary 사이의 중간 밝기 텍스트 |
| `--color-text-max` | `#fdfdfd` | 최대 밝기 텍스트 (순백에 가까움) |

### 3.4 시맨틱 색상 (Semantic)

| CSS 변수 | 값 | 용도 |
|----------|-----|------|
| `--color-negative` | `#f3727f` | 에러, 오프라인 상태 표시 |
| `--color-warning` | `#ffa42b` | 경고, 재접속 중 상태 표시 |
| `--color-info` | `#539df5` | 정보성 안내 |

### 3.5 테두리 색상 (Borders)

| CSS 변수 | 값 | 용도 |
|----------|-----|------|
| `--color-border` | `#4d4d4d` | 기본 테두리 |
| `--color-border-light` | `#7c7c7c` | 밝은 테두리 |
| `--color-separator` | `#b3b3b3` | 구분선 |

### 3.6 그림자 시스템 (Shadows)

| CSS 변수 | 값 | 용도 |
|----------|-----|------|
| `--shadow-heavy` | `rgba(0,0,0,0.5) 0px 8px 24px` | 모달, 팝오버 등 높은 elevation |
| `--shadow-medium` | `rgba(0,0,0,0.3) 0px 8px 8px` | 카드, 소켓 표시기 pill 등 |

두 그림자 모두 `y-offset: 8px`이며, `blur-radius`로 차이를 둔다 (`24px` vs `8px`). 알파값도 `0.5` vs `0.3`으로 heavy가 더 진하다.

### 3.7 Border Radius 시스템

| CSS 변수 | 값 | 용도 |
|----------|-----|------|
| `--radius-badge` | `2px` | 배지, 태그 |
| `--radius-subtle` | `4px` | 미세한 둥글림 |
| `--radius-standard` | `6px` | 일반 버튼, 입력 필드 |
| `--radius-comfortable` | `8px` | 큰 버튼, 카드 등 |
| `--radius-panel` | `16px` | 패널, 모달 |
| `--radius-pill-lg` | `500px` | 큰 pill 형태 |
| `--radius-pill` | `9999px` | 완전 pill (소켓 표시기 등) |

### 3.8 폰트 설정

```css
body {
  font-family: -apple-system, BlinkMacSystemFont, 'Helvetica Neue', helvetica, arial,
    'Hiragino Sans', 'Hiragino Kaku Gothic ProN', Meiryo, sans-serif;
}
```

시스템 폰트 스택을 사용한다. Pretendard는 포함되어 있지 않으며, 다음 우선순위로 적용된다:

1. `-apple-system` / `BlinkMacSystemFont` -- macOS/iOS 시스템 폰트 (San Francisco)
2. `Helvetica Neue`, `helvetica`, `arial` -- cross-platform fallback
3. `Hiragino Sans`, `Hiragino Kaku Gothic ProN` -- 일본어 폰트 (macOS)
4. `Meiryo` -- 일본어 폰트 (Windows)
5. `sans-serif` -- 최종 fallback

추가 폰트 설정:

| 속성 | 값 |
|------|-----|
| `font-size` | `16px` |
| `line-height` | `1.4` |
| `-webkit-font-smoothing` | `antialiased` |
| `-moz-osx-font-smoothing` | `grayscale` |
| `user-select` | `none` |
| `-webkit-user-select` | `none` |

### 3.9 반응형 및 컨테이너 설정

```css
html, body, #root {
  height: 100%;
  width: 100%;
}

#root {
  max-width: 430px;
  margin: 0 auto;
  min-height: 100svh;
  position: relative;
  overflow-x: hidden;
}
```

| 속성 | 값 | 비고 |
|------|-----|------|
| `#root` max-width | `430px` | 모바일 퍼스트 (iPhone Pro Max 기준) |
| `#root` margin | `0 auto` | 데스크톱에서 가운데 정렬 |
| `#root` min-height | `100svh` | small viewport height (모바일 브라우저 주소창 고려) |
| `#root` position | `relative` | 자식 요소의 absolute 기준점 |
| `#root` overflow-x | `hidden` | 수평 스크롤 방지 |

### 3.10 전역 키프레임 애니메이션

#### `pulse` (내 위치 마커용)

```css
@keyframes pulse {
  0%   { transform: scale(1);   opacity: 1; }
  100% { transform: scale(2.5); opacity: 0; }
}
```

- 1배에서 2.5배로 확대되며 완전 투명으로 소멸.

#### `truck-wobble` (소방차 흔들림)

```css
@keyframes truck-wobble {
  0%   { transform: rotate(-3deg); }
  100% { transform: rotate(3deg); }
}
```

- -3도에서 +3도까지 회전 (총 6도 진폭).

#### `siren` (소방차 경광등 점멸)

```css
@keyframes siren {
  0%   { color: #ff0000; background: #ff0000; }
  100% { color: #0066ff; background: #0066ff; }
}
```

- 빨간색(`#ff0000`)에서 파란색(`#0066ff`)으로 교대 점멸.

#### `stage-shake` (화재 단계 변경 흔들림)

```css
@keyframes stage-shake {
  0%, 100% { transform: translateX(0); }
  15% { transform: translateX(-3px); }
  30% { transform: translateX(3px); }
  45% { transform: translateX(-2px); }
  60% { transform: translateX(2px); }
  75% { transform: translateX(-1px); }
  90% { transform: translateX(1px); }
}
```

- 좌우 교대 흔들림, 진폭 3px → 1px으로 감쇠.
- 500ms, `ease-out`.

#### `stage-glow` (화재 단계 변경 글로우)

```css
@keyframes stage-glow {
  0%   { opacity: 0.9; }
  100% { opacity: 0; }
}
```

- 아이콘 외곽의 `box-shadow` 글로우가 fade-out.
- 700ms, `ease-out`, `forwards`.

---

## 4. 라우팅

### 4.1 TanStack Router 설정

```ts
import { createRouter } from '@tanstack/react-router'
import { routeTree } from '../routeTree.gen'

export const router = createRouter({ routeTree })
```

- TanStack Router의 파일 기반 라우팅을 사용한다.
- `routeTree.gen.ts`는 `client/src/routes/` 디렉토리에서 자동 생성된 라우트 트리이다.
- TypeScript module augmentation을 통해 `Register` 인터페이스에 라우터 타입을 등록하여, `<Link>`, `useNavigate()` 등에서 타입 안전한 경로 추론이 가능하다.

### 4.2 라우트 목록

| 경로 | 파일 | 설명 |
|------|------|------|
| `/` | `routes/index.tsx` | 홈 (인덱스) 페이지 |
| `/map` | `routes/map.tsx` | 지도 페이지 (실시간 화재 현황) |
| `/archive` | `routes/archive.tsx` | 아카이브 페이지 |

### 4.3 Root Layout 구조

```tsx
// routes/__root.tsx
export const Route = createRootRoute({
  component: () => <Outlet />,
})
```

Root layout은 `<Outlet />`만 렌더링하는 최소 구조이다. 전역 헤더(`<Header />`)는 root layout에 포함되지 않고, 각 페이지 컴포넌트에서 개별적으로 포함하거나 페이지별 layout에서 관리하는 구조이다.

---

## 5. 소켓 관리 (SocketProvider + socketManager)

### 5.1 아키텍처 개요

소켓 관리는 두 모듈로 분리된다:

- **`socketManager.ts`**: Socket.IO 인스턴스 생성, 이벤트 핸들러 등록, heartbeat 관리, Zustand 상태 스토어. 순수 로직 모듈.
- **`SocketProvider.tsx`**: React lifecycle에서 `startSocket()`을 1회 호출하는 브릿지 컴포넌트.

### 5.2 초기화 흐름

```
App mount
  └─ SocketProvider mount
       └─ useEffect([], startSocket)
            ├─ started 가드 체크 (StrictMode 더블마운트 방어)
            ├─ socket 이벤트 리스너 등록 (connect, disconnect, connect_error, reconnect_attempt, reconnect_failed)
            ├─ setStatus('connecting')
            └─ socket.connect()
```

1. `SocketProvider`의 `useEffect([], ...)`에서 `startSocket()` 호출.
2. `started` 플래그가 `false`일 때만 실행 (React StrictMode에서 `useEffect`가 2번 호출되는 것을 방어).
3. 소켓 이벤트 리스너 5종을 등록.
4. 상태를 `'connecting'`으로 설정.
5. `socket.connect()` 호출로 실제 연결 시작.
6. cleanup 함수에서 `disconnect`를 호출하지 않음 -- 앱 수명 동안 소켓을 유지하기 위함.

### 5.3 연결 옵션

```ts
export const socket: Socket = io(SOCKET_URL, {
  autoConnect: false,
  transports: ['polling', 'websocket'],
  reconnection: true,
  reconnectionAttempts: 8,
  reconnectionDelay: 500,
  reconnectionDelayMax: 5_000,
  auth: (cb) => cb({ user_id: getOrCreateAnonUserId() }),
})
```

| 옵션 | 값 | 비고 |
|------|-----|------|
| `autoConnect` | `false` | `startSocket()`에서 수동으로 `connect()` 호출 |
| `transports` | `['polling', 'websocket']` | HTTP long-polling으로 시작 후 WebSocket으로 업그레이드 |
| `reconnection` | `true` | 자동 재접속 활성화 |
| `reconnectionAttempts` | `8` (`MAX_RECONNECT_ATTEMPTS`) | 최대 재접속 시도 횟수 |
| `reconnectionDelay` | `500`ms | 첫 재접속 대기 시간 |
| `reconnectionDelayMax` | `5,000`ms | 최대 재접속 대기 시간 (exponential backoff 상한) |
| `auth` | `(cb) => cb({ user_id })` | 콜백 방식 인증. 매 연결 시 최신 user_id 전달 |

#### SOCKET_URL 설정 (`config.ts`)

```ts
export const SOCKET_URL = import.meta.env.VITE_SOCKET_URL ?? 'https://hwarr.com'
```

- 환경변수 `VITE_SOCKET_URL`이 설정되어 있으면 해당 값 사용.
- 미설정 시 프로덕션 URL `https://hwarr.com`을 기본값으로 사용.

#### 익명 사용자 ID (`getOrCreateAnonUserId`)

재접속 시 서버에서 이전 room 구독을 복원하기 위한 익명 식별자이다.

- `localStorage` 키: `hwarr:anonUserId`
- 생성 전략:
  1. `localStorage`에 기존 ID가 있으면 재사용.
  2. 없으면 `crypto.randomUUID()`로 생성 (지원 시).
  3. `crypto.randomUUID()` 미지원 시 `anon-${Date.now()}-${Math.random().toString(36).slice(2, 10)}` 형식으로 fallback.
  4. `localStorage` 접근 실패 시 (프라이빗 모드 등) `anon-${Date.now()}` 형식의 세션 한정 ID 생성.

### 5.4 Heartbeat

서버의 reaper가 `TIMEOUT=30s` 동안 heartbeat가 없는 클라이언트를 정리하므로, 클라이언트는 `10s` 간격으로 heartbeat를 전송한다.

| 상수 | 값 | 비고 |
|------|-----|------|
| `HEARTBEAT_MS` | `10,000`ms (10초) | 서버 `HEARTBEAT_INTERVAL_SEC=10`에 맞춤 |

#### Heartbeat 동작:

1. **시작** (`startHeartbeat`): `connect` 이벤트 발생 시 호출.
   - 즉시 첫 heartbeat 전송: `socket.emit('heartbeat', { ts: Date.now() })`.
   - 이후 `setInterval`로 10초마다 반복 전송.
   - 전송 전 `socket.connected` 체크.
2. **중지** (`stopHeartbeat`): `disconnect` 이벤트 발생 시 호출.
   - `clearInterval`로 타이머 해제.
3. **Payload**: `{ ts: number }` -- 밀리초 타임스탬프.

### 5.5 상태 관리 (useSocketStore)

Zustand를 사용한 전역 상태 스토어이다.

```ts
interface SocketState {
  status: ConnectionStatus
  reconnectAttempt: number
  setStatus: (s: ConnectionStatus) => void
  setAttempt: (n: number) => void
}
```

| 상태 | 타입 | 초기값 | 설명 |
|------|------|--------|------|
| `status` | `ConnectionStatus` | `'idle'` | 현재 소켓 연결 상태 |
| `reconnectAttempt` | `number` | `0` | 현재 재접속 시도 횟수 |

#### ConnectionStatus 상태 전이

```
idle ──(startSocket)──→ connecting ──(connect)──→ connected
                                                     │
                                              (disconnect)
                                                     │
                                                     ▼
                                               reconnecting
                                                  │      │
                                           (connect)  (reconnect_failed)
                                                  │      │
                                                  ▼      ▼
                                             connected  offline
```

| 이벤트 | 상태 전이 | 추가 동작 |
|--------|----------|----------|
| `startSocket()` 호출 | `idle` → `connecting` | `socket.connect()` |
| `connect` | → `connected` | `reconnectAttempt = 0`, heartbeat 시작 |
| `disconnect` | → `reconnecting` | heartbeat 중지. `io server disconnect` 사유 시 `socket.connect()` 수동 호출 |
| `reconnect_attempt` | → `reconnecting` | `reconnectAttempt = n` 업데이트 |
| `reconnect_failed` | → `offline` | 8회 재접속 실패 후 포기 |
| `stopSocket()` (테스트용) | → `idle` | heartbeat 중지, 리스너 제거, disconnect, `started = false` |

#### 서버 강제 해제 대응

Socket.IO는 `disconnect` 사유가 `'io server disconnect'`인 경우 자동 재접속을 수행하지 않는다. 이를 보정하기 위해 해당 사유 시 `socket.connect()`를 수동으로 호출한다:

```ts
socket.on('disconnect', (reason) => {
  stopHeartbeat()
  setStatus('reconnecting')
  if (reason === 'io server disconnect') {
    socket.connect()
  }
})
```

### 5.6 추가 설정 상수 (config.ts)

불 시스템 관련 상수도 `config.ts`에 정의되어 있으며, 서버(`internal/grid/grid.go`)와 동일한 값을 유지해야 한다:

| 상수 | 값 | 비고 |
|------|-----|------|
| `API_URL` | `VITE_API_URL` 또는 `'https://hwarr.com'` | REST API 엔드포인트 |
| `SOCKET_URL` | `VITE_SOCKET_URL` 또는 `'https://hwarr.com'` | Socket.IO 엔드포인트 |
| `LAT_UNIT` | `0.0009` | 위도 그리드 단위 (~100m) |
| `LNG_UNIT` | `0.0011` | 경도 그리드 단위 (~100m, 한국 기준 ~37도N) |
| `TTL_SECONDS` | `1800` | 불 TTL 30분 |
