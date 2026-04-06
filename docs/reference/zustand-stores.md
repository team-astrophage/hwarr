# Zustand 스토어 레퍼런스

화르르 클라이언트에서 사용하는 모든 Zustand 스토어를 정리한 문서.

> **범례** — 초기값이 함수인 경우 시그니처만 표기. `(set)` 콜백 내부 로직은 "액션" 절에서 설명.

---

## 목차

1. [useFireStore](#usefirestore)
2. [useAnimationStore](#useanimationstore)
3. [useChatStore](#usechatstore)
4. [useSocketStore](#usesocketstore)

---

## useFireStore

**파일** — `client/src/features/fire-map/stores/fireStore.ts`

불(Fire) 격자 상태를 관리하는 핵심 스토어. Socket 이벤트로 수신한 격자별 불 정보를 `Map<gridId, FireCell>` 형태로 저장하며, 컴포넌트는 이 스토어를 구독해 리렌더한다.

### 타입

```ts
interface FireCell {
  gridId: string
  activeCount: number
  stage: number // 0~5
}
```

### 상태 필드

| 필드 | 타입 | 초기값 | 설명 |
|------|------|--------|------|
| `fires` | `Map<string, FireCell>` | `new Map()` | 격자 ID를 key로 하는 활성 불 맵. O(1) 조회. |
| `onlineUsers` | `number` | `0` | 서버에서 전달받은 실시간 접속자 수 (방화범 카운트). |

### 액션

| 액션 | 시그니처 | 설명 |
|------|----------|------|
| `updateFire` | `(cell: FireCell) => void` | 개별 격자 불 상태 업데이트. `activeCount > 0`이면 맵에 추가/갱신하고, `0`이면 삭제한다. `stage >= 5`(전소) 최초 도달 시 `useAnimationStore.triggerExplosion()`을 호출한다. |
| `syncFires` | `(cells: FireCell[]) => void` | 전체 활성 불 목록으로 맵을 교체한다. 서버 접속 또는 재접속 시 `fires:sync` 이벤트 수신 시 사용. |
| `setOnlineUsers` | `(count: number) => void` | 접속자 수를 설정한다. |

### 구독 패턴

| 소비처 | 사용 필드/액션 |
|--------|----------------|
| `useFireSocket` (훅) | `updateFire`, `syncFires`, `setOnlineUsers` — 소켓 이벤트 핸들러에서 호출 |
| `FireCanvas` (컴포넌트) | `fires` — Canvas 렌더링용 불 데이터 읽기 |
| `BottomPanel` (컴포넌트) | `onlineUsers` 또는 `fires` — 하단 패널 정보 표시 |
| `FiretruckOverlay` (컴포넌트) | `fires` — 소방차 오버레이 렌더링 |

### 크로스 스토어 의존성

- **useAnimationStore** — `updateFire` 내부에서 `stage >= 5` 조건 충족 시 `useAnimationStore.getState().triggerExplosion(gridId)`를 직접 호출한다.

---

## useAnimationStore

**파일** — `client/src/features/fire-map/stores/animationStore.ts`

성냥 던지기, 폭발 이펙트, 불 확산 궤적 등 시각 애니메이션 상태를 관리한다.

### 내부 타입

```ts
interface MatchThrow {
  id: number
  startTime: number   // performance.now()
  gridId: string
}

interface Explosion {
  id: number
  startTime: number
  gridId: string
}

interface SpreadTrajectory {
  id: number
  startTime: number
  fromGridId: string  // 불이 출발한 격자
  toGridId: string    // 불이 착륙한 격자
}
```

### 상태 필드

| 필드 | 타입 | 초기값 | 설명 |
|------|------|--------|------|
| `matches` | `MatchThrow[]` | `[]` | 활성 성냥 포물선 애니메이션 목록. |
| `explosions` | `Explosion[]` | `[]` | 전소 폭발 이펙트 목록. |
| `explodedGrids` | `Set<string>` | `new Set()` | 전소 달성한 격자 ID 추적 (중복 폭발 방지). |
| `postExplodeTapCount` | `number` | `0` | 5단계(전소) 달성 이후 누적 탭 카운트. 랜덤 폭발 트리거 판정에 사용. |
| `nextRandomExplosionAt` | `number` | `20~30` (랜덤) | 다음 랜덤 폭발이 발동될 탭 카운트 임계값. `RANDOM_EXPLOSION_INTERVAL_MIN(20)` ~ `RANDOM_EXPLOSION_INTERVAL_MAX(30)` 범위에서 결정. |
| `trajectories` | `SpreadTrajectory[]` | `[]` | 불 확산 궤적 애니메이션 목록. |

### 액션

| 액션 | 시그니처 | 설명 |
|------|----------|------|
| `throwMatch` | `(gridId: string) => void` | 성냥 포물선 애니메이션을 추가한다. `explodedGrids`가 비어있지 않으면(즉 전소 경험 후) 탭 카운트를 누적하고, `nextRandomExplosionAt`에 도달하면 폭발 이펙트도 함께 트리거한다. |
| `removeMatch` | `(id: number) => void` | 완료된 성냥 애니메이션을 목록에서 제거한다. |
| `triggerExplosion` | `(gridId: string) => void` | 해당 격자에 폭발 이펙트를 추가한다. 이미 `explodedGrids`에 포함된 격자면 무시 (중복 방지). |
| `removeExplosion` | `(id: number) => void` | 완료된 폭발 이펙트를 목록에서 제거한다. |
| `addTrajectory` | `(fromGridId: string, toGridId: string) => void` | 불 확산 궤적 애니메이션을 추가한다. |
| `removeTrajectory` | `(id: number) => void` | 완료된 궤적 애니메이션을 목록에서 제거한다. |

### 상수

| 상수 | 값 | 설명 |
|------|---|------|
| `RANDOM_EXPLOSION_INTERVAL_MIN` | `20` | 랜덤 폭발 간격 최소 (탭 횟수) |
| `RANDOM_EXPLOSION_INTERVAL_MAX` | `30` | 랜덤 폭발 간격 최대 (탭 횟수) |

### 구독 패턴

| 소비처 | 사용 필드/액션 |
|--------|----------------|
| `useFireSocket` (훅) | `addTrajectory` — `fire:spread` 소켓 이벤트 수신 시 `getState()`로 호출 |
| `MapPage` (컴포넌트) | `throwMatch` — 사용자 탭 이벤트 핸들링 |
| `FireCanvas` (컴포넌트) | `matches`, `explosions`, `trajectories`, `removeMatch`, `removeExplosion`, `removeTrajectory` — Canvas 애니메이션 렌더 및 완료 후 정리 |
| `fireStore` (스토어) | `triggerExplosion` — `updateFire`에서 `stage >= 5` 시 `getState()`로 호출 |

### 크로스 스토어 의존성

- 없음 (다른 스토어에 의존하지 않음). 단, **useFireStore**와 **useFireSocket**에서 역방향으로 호출된다.

---

## useChatStore

**파일** — `client/src/features/chat/useChatStore.ts`

채팅 메시지와 접속 상태를 관리한다. 소켓 이벤트에서 수신한 메시지를 스토어에 적재하고, `ChatPanel`이 구독하여 렌더링하는 파이프라인의 중간 계층이다.

### 타입

```ts
interface ChatMessage {
  id: string
  user_id: string
  nickname: string
  avatar: string
  avatar_bg: string
  name_color: string
  text: string
  timestamp: number
}

type ConnectionStatus = 'disconnected' | 'connecting' | 'connected'
```

### 상태 필드

| 필드 | 타입 | 초기값 | 설명 |
|------|------|--------|------|
| `messages` | `ChatMessage[]` | `[]` | 채팅 메시지 배열. 최대 `MESSAGE_CAP`(200)개까지 유지하며 오래된 메시지부터 drop. |
| `presenceCount` | `number` | `0` | 채팅 접속자 수. |
| `connectionStatus` | `ConnectionStatus` | `'disconnected'` | 채팅 연결 상태. |

### 액션

| 액션 | 시그니처 | 설명 |
|------|----------|------|
| `appendMessage` | `(msg: ChatMessage) => void` | 메시지를 추가한다. 동일 `id` 중복 방지 로직이 포함되어 있다 (자기 메시지 echo + 서버 broadcast 중복 대응). `MESSAGE_CAP` 초과 시 오래된 메시지를 앞에서 제거. |
| `setHistory` | `(msgs: ChatMessage[]) => void` | 전체 메시지 히스토리를 교체한다. 최근 `MESSAGE_CAP`개만 유지. |
| `setPresence` | `(count: number) => void` | 접속자 수를 설정한다. |
| `setStatus` | `(status: ConnectionStatus) => void` | 연결 상태를 설정한다. |
| `reset` | `() => void` | 메시지 배열과 접속자 수를 초기값으로 리셋한다. |

### 상수

| 상수 | 값 | 설명 |
|------|---|------|
| `MESSAGE_CAP` | `200` | 보관 가능한 최대 메시지 수 |

### 구독 패턴

| 소비처 | 사용 필드/액션 |
|--------|----------------|
| `useChat` (훅) | `appendMessage`, `setHistory`, `setPresence`, `setStatus`, `reset` — 소켓 이벤트 핸들러에서 호출 |
| `ChatPanel` (컴포넌트) | `messages`, `presenceCount`, `connectionStatus` — 채팅 UI 렌더링 |

### 크로스 스토어 의존성

- 없음. 독립적으로 동작한다.

---

## useSocketStore

**파일** — `client/src/lib/socketManager.ts` (스토어가 소켓 매니저 내부에 임베딩됨)

Socket.IO 연결 수명주기 상태를 전역으로 노출하는 스토어. 소켓 인스턴스(`socket`)와 함께 `socketManager.ts`에 정의되어 있다.

### 타입

```ts
type ConnectionStatus =
  | 'idle'
  | 'connecting'
  | 'connected'
  | 'reconnecting'
  | 'offline'
```

### 상태 필드

| 필드 | 타입 | 초기값 | 설명 |
|------|------|--------|------|
| `status` | `ConnectionStatus` | `'idle'` | 현재 소켓 연결 상태. `idle` → `connecting` → `connected` 순으로 전이하며, 끊어지면 `reconnecting`, 최종 실패 시 `offline`. |
| `reconnectAttempt` | `number` | `0` | 현재 재접속 시도 횟수. 연결 성공 시 `0`으로 리셋. |

### 액션

| 액션 | 시그니처 | 설명 |
|------|----------|------|
| `setStatus` | `(s: ConnectionStatus) => void` | 연결 상태를 설정한다. |
| `setAttempt` | `(n: number) => void` | 재접속 시도 횟수를 설정한다. |

### 소켓 매니저 함수 (스토어 외부)

스토어와 함께 export되는 소켓 관련 유틸리티:

| 함수 | 시그니처 | 설명 |
|------|----------|------|
| `startSocket` | `() => void` | 소켓 연결을 시작한다. 앱에서 1회만 호출. `StrictMode` 더블마운트 방어 가드(`started`) 적용. 내부에서 `connect`, `disconnect`, `connect_error`, `reconnect_attempt`, `reconnect_failed` 이벤트 리스너를 등록하고, heartbeat 타이머(10초 간격)를 관리한다. |
| `stopSocket` | `() => void` | 소켓을 정리한다. heartbeat 중단, 리스너 전체 제거, 소켓 disconnect, 상태를 `idle`로 리셋. 테스트/HMR 전용. |
| `socket` | `Socket` (인스턴스) | 앱 전역에서 공유되는 단일 Socket.IO 인스턴스. `autoConnect: false`로 생성되며, `auth` 콜백으로 익명 `user_id`를 전달한다. |

### 소켓 설정

| 설정 | 값 | 설명 |
|------|---|------|
| `transports` | `['polling', 'websocket']` | polling 우선, 이후 websocket upgrade |
| `reconnectionAttempts` | `8` | 최대 재접속 시도 횟수 |
| `reconnectionDelay` | `500ms` | 재접속 초기 딜레이 |
| `reconnectionDelayMax` | `5,000ms` | 재접속 최대 딜레이 |
| `HEARTBEAT_MS` | `10,000ms` | 서버 heartbeat 간격 (서버 reaper timeout=30s에 대응) |

### 구독 패턴

| 소비처 | 사용 필드/액션 |
|--------|----------------|
| `Header` (컴포넌트) | `status` — 연결 상태 표시 (인디케이터 등) |
| `socketManager.ts` (내부) | `setStatus`, `setAttempt` — 소켓 이벤트 핸들러에서 `getState()`로 호출 |

### 크로스 스토어 의존성

- 없음. 다만 소켓 이벤트를 통해 간접적으로 **useFireStore**, **useChatStore** 등의 데이터 흐름을 구동한다.

---

## 크로스 스토어 의존성 요약

```
useFireSocket (훅)
  ├─ useFireStore.updateFire / syncFires / setOnlineUsers
  └─ useAnimationStore.addTrajectory  (getState() 직접 접근)

useFireStore.updateFire
  └─ useAnimationStore.triggerExplosion  (getState() 직접 접근, stage >= 5)

socketManager (startSocket)
  └─ useSocketStore.setStatus / setAttempt  (getState() 직접 접근)

useChat (훅)
  └─ useChatStore.appendMessage / setHistory / setPresence / setStatus / reset
```

### 데이터 흐름 다이어그램

```
Socket.IO 서버
  │
  ├─ fire:update ──→ useFireSocket ──→ useFireStore.updateFire
  │                                        └─→ useAnimationStore.triggerExplosion (stage>=5)
  ├─ fires:sync  ──→ useFireSocket ──→ useFireStore.syncFires
  ├─ fire:spread ──→ useFireSocket ──→ useAnimationStore.addTrajectory
  ├─ users:count ──→ useFireSocket ──→ useFireStore.setOnlineUsers
  │
  ├─ chat:message ──→ useChat ──→ useChatStore.appendMessage
  ├─ chat:history ──→ useChat ──→ useChatStore.setHistory
  ├─ chat:presence ──→ useChat ──→ useChatStore.setPresence
  │
  └─ connect / disconnect / reconnect_* ──→ socketManager ──→ useSocketStore
```
