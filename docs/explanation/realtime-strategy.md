# 실시간 통신 전략

화르르는 GPS 기반 실시간 불 지도 서비스로, 수백~수천 명의 동시접속자가 같은 지도 위에서 불 이벤트를 주고받아야 한다. 이 문서는 Socket.IO 기반 실시간 통신 계층의 설계 결정과 그 근거를 설명한다.

## 목차

1. [Viewport 기반 Room 구독](#1-viewport-기반-room-구독)
2. [Heartbeat/Reaper 시스템](#2-heartbeatreaper-시스템)
3. [재접속 복원](#3-재접속-복원)
4. [채팅 Room 분리](#4-채팅-room-분리)
5. [클라이언트 소켓 관리](#5-클라이언트-소켓-관리)
6. [오프라인 Emit 정책](#6-오프라인-emit-정책)

---

## 1. Viewport 기반 Room 구독

### 문제

지도 서비스에서 전국의 모든 불 이벤트를 모든 클라이언트에 broadcast하면, 서울을 보는 사용자가 부산의 불 업데이트까지 수신하게 된다. 클라이언트 수가 늘어날수록 불필요한 트래픽이 선형으로 증가한다.

### 설계: 격자(grid) 기반 Room

클라이언트가 지도를 이동하거나 줌 레벨을 변경할 때마다 `subscribe:viewport` 이벤트를 emit한다. 서버는 viewport bounding box(`ne_lat`, `ne_lng`, `sw_lat`, `sw_lng`)를 격자 ID 목록으로 변환한 뒤, 해당 클라이언트를 그 격자들의 Socket.IO room에 join시킨다.

```
클라이언트 → subscribe:viewport { ne_lat, ne_lng, sw_lat, sw_lng }
서버       → get_grids_in_viewport() → grid ID 목록
           → manager.join_rooms(sid, grid_ids)  ← 이전 room은 전부 leave
           → 현재 활성 불 상태를 ack로 반환
```

핵심은 `join_rooms`가 **이전 구독을 완전히 교체**한다는 점이다. 한 클라이언트는 항상 하나의 viewport만 구독하므로, 이전 room을 모두 leave한 뒤 새 room에 join한다. 이로써 뷰포트 밖 이벤트는 자동으로 수신되지 않는다.

### 전역 vs Room-scoped 이중 Broadcast

불 이벤트(`fire:ignite`)가 발생하면 서버는 두 가지 경로로 broadcast한다:

| 경로 | 이벤트 | 대상 | 용도 |
|------|--------|------|------|
| Room-scoped | `fire:ignite` | 해당 격자 room 구독자 | 상세 정보 (좌표, event_id, stage_info 등) |
| 전역 | `fire:global_update` | 전체 접속자 | 미니맵, 전국 통계 등 overview 갱신 |

Room-scoped broadcast는 해당 격자를 보고 있는 사용자에게만 상세 payload를 전달하여 대역폭을 절약한다. 전역 broadcast는 축소된 payload(`grid_id`, `active_count`, `stage`)만 포함하여 전국 단위 UI(미니맵, 랭킹 등)를 갱신하는 데 쓰인다.

### 재구독 타이밍

`subscribe:viewport`는 viewport 변경 시뿐 아니라 **재접속 시에도** 발행된다. `useFireSocket` 훅이 `connect` 이벤트에 `requestSync` 콜백을 등록하고 있어, 소켓이 reconnect되면 `get_fires`를 통해 전체 활성 불 상태를 재동기화한다. 또한 서버 측 `connect` 핸들러에서 `restore_rooms`로 이전 viewport room 구독이 자동 복원된다.

---

## 2. Heartbeat/Reaper 시스템

### 세 가지 상수와 설계 근거

```python
HEARTBEAT_INTERVAL_SEC = 10   # 클라이언트 → 서버 heartbeat 주기
HEARTBEAT_TIMEOUT_SEC  = 30   # 이 시간 동안 heartbeat 없으면 stale 판정
REAPER_INTERVAL_SEC    = 15   # stale 연결 정리 주기
```

**왜 10초 간격인가.** 모바일 환경에서 네트워크 전환(Wi-Fi ↔ LTE)이 빈번하다. 10초는 네트워크 불안정 상황에서 1~2회 heartbeat 손실을 허용하면서도 연결 상태를 합리적으로 추적할 수 있는 간격이다. Socket.IO 자체의 `ping_interval`(역시 10초)과 일치시켜 transport 레벨과 application 레벨의 생존 확인 주기를 통일했다.

**왜 30초 timeout인가.** heartbeat 3회분(10초 x 3)에 해당한다. 일시적인 네트워크 끊김(터널, 엘리베이터)에서 즉시 연결을 끊지 않으면서도, 완전히 이탈한 클라이언트가 서버 자원을 점유하는 시간을 30초 이내로 제한한다.

**왜 15초 reaper 주기인가.** reaper가 15초마다 순회하므로, timeout(30초) 도달 후 최대 15초 이내에 stale 연결이 정리된다. 즉, 실제 최악 정리 지연은 약 45초(30 + 15)이다. reaper 주기를 heartbeat 간격(10초)보다 길게 잡아 불필요한 순회를 줄이면서도 timeout의 절반 이내로 설정하여 응답성을 확보했다.

### 동작 흐름

```
클라이언트                    서버
   │                          │
   │──── heartbeat ──────────▶│  record_heartbeat(sid)
   │                          │  last_heartbeat = now
   │                          │
   │       (10초 후)           │
   │──── heartbeat ──────────▶│
   │                          │
   │    (연결 끊김)             │
   │          ✕                │
   │                          │  ... 15초마다 reaper 순회 ...
   │                          │  is_stale(sid) → now - last_heartbeat > 30
   │                          │  → sio.disconnect(sid)
```

Transport 레벨의 `ping_timeout`(5초)은 TCP 연결이 완전히 끊긴 경우를 빠르게 감지한다. Application 레벨의 heartbeat/reaper는 TCP는 살아있지만 클라이언트 앱이 멈춘 경우(탭 백그라운드, 프로세스 정지 등)를 보완한다.

---

## 3. 재접속 복원

### 익명 user_id

화르르는 로그인 없는 익명 서비스이다. 그러나 재접속 시 이전 상태(구독 중이던 viewport room 등)를 복원하려면 Socket.IO의 일회성 `sid`와 별개로 **안정적인 식별자**가 필요하다.

클라이언트는 최초 접속 시 `crypto.randomUUID()`로 UUID를 생성하여 `localStorage`에 저장한다(`hwarr:anonUserId` key). 이후 모든 Socket.IO 연결에서 `auth.user_id`로 이 값을 전송한다.

```typescript
// socketManager.ts
auth: (cb) => cb({ user_id: getOrCreateAnonUserId() }),
```

`localStorage` 접근이 실패하면(private 모드 등) 세션 한정 ID(`anon-{timestamp}`)로 fallback한다. 이 경우 재접속 복원은 동일 세션 내에서만 동작한다.

### 서버 측 복원 흐름

```
1. connect(sid, auth={ user_id: "abc-123" })
2. manager._user_sessions["abc-123"] 에서 이전 ConnectionInfo 조회
3. 이전 세션의 rooms 목록 복사
4. manager.add(sid, user_id="abc-123") → 새 ConnectionInfo 생성, reconnect_count 증가
5. manager.restore_rooms(sid, previous_rooms) → 이전 room에 다시 join
6. "connected" ack 전송 (restored_rooms 수 포함)
```

`_user_sessions` dict는 `user_id → ConnectionInfo` 매핑을 유지한다. `remove(sid)` 시에도 `_user_sessions`에서 삭제하지 않으므로, 연결이 끊긴 후 재접속해도 이전 room 정보가 보존된다.

---

## 4. 채팅 Room 분리

### 독립적인 `chat:global` Room

채팅은 viewport 격자 room과 완전히 독립된 `chat:global`이라는 단일 room에서 운영된다. 이 분리에는 두 가지 이유가 있다:

1. **구독 생명주기가 다르다.** viewport room은 지도를 pan/zoom할 때마다 교체된다(`join_rooms`가 이전 room을 전부 leave). 만약 채팅이 격자 room에 포함되어 있었다면, 지도를 움직일 때마다 채팅 연결이 끊겼다가 다시 연결되는 문제가 생긴다. `chat:global`은 `join_rooms`의 교체 대상에 포함되지 않는 별도의 room이므로 viewport 변경과 무관하게 유지된다.

2. **참여 범위가 다르다.** 불 이벤트는 지역적(해당 격자를 보는 사용자만 수신)이지만, 채팅은 전역적(모든 참여자가 모든 메시지를 수신)이다.

### 클라이언트 측 생명주기

`useChat(active)` 훅은 `active` flag로 채팅 참여를 제어한다:

- `active=true`: `chat:join` emit → 서버가 `chat:global` room에 join + 히스토리 반환
- `active=false`(cleanup): `chat:leave` emit → 서버가 room에서 leave + presence 갱신

소켓 인스턴스 자체는 `useChat`이 관리하지 않는다. 전역 `SocketProvider`가 소켓 연결을 유지하고, `useChat`은 채팅 전용 이벤트 리스너와 room join/leave만 담당한다.

### 메시지 영속성

채팅 메시지는 Redis에 LPUSH로 저장되며, 100개 상한(`LTRIM`)과 1시간 TTL로 관리된다. 새 사용자가 `chat:join`하면 최근 50개 히스토리를 역순으로 받아 시간순 표시한다. 이는 "ephemeral chat" 설계로, 장기 보관이 아닌 현재 활성 사용자 간의 소통에 초점을 맞춘다.

---

## 5. 클라이언트 소켓 관리

### 싱글턴 패턴

`socketManager.ts`는 모듈 스코프에서 `io()` 호출로 단일 `Socket` 인스턴스를 생성한다(`autoConnect: false`). 이 인스턴스는 앱 전체에서 import하여 공유한다.

```typescript
export const socket: Socket = io(SOCKET_URL, {
  autoConnect: false,
  transports: ['polling', 'websocket'],
  // ...
})
```

`autoConnect: false`로 설정한 이유는 소켓 생성 시점(모듈 로드)과 연결 시점(앱 마운트)을 분리하기 위함이다. 실제 연결은 `SocketProvider`가 마운트될 때 `startSocket()`을 호출하여 시작한다.

### StrictMode 방어

React 18의 StrictMode는 개발 모드에서 `useEffect`를 두 번 실행한다. `SocketProvider`가 두 번 마운트되면 소켓이 중복 연결될 수 있다. 이를 방어하기 위해:

1. **`started` 가드**: `startSocket()` 내부의 모듈 레벨 `started` boolean이 중복 호출을 차단한다.
2. **cleanup 생략**: `SocketProvider`의 `useEffect`는 cleanup에서 `disconnect`를 호출하지 않는다. 전역 소켓은 앱 수명 동안 유지되어야 하므로, StrictMode의 mount → unmount → mount 사이클에서 연결이 끊기지 않는다.

```typescript
// SocketProvider.tsx
useEffect(() => {
  startSocket()
  // cleanup 없음 — 의도적
}, [])
```

### 기능별 훅의 역할 분리

소켓 연결/해제는 `SocketProvider`가 전담하고, 기능별 훅(`useFireSocket`, `useChat`)은 리스너 등록/해제만 수행한다:

| 계층 | 책임 |
|------|------|
| `socketManager.ts` | 인스턴스 생성, heartbeat, 연결 상태 관리 |
| `SocketProvider.tsx` | 앱 마운트 시 1회 `startSocket()` 호출 |
| `useFireSocket.ts` | `fire:update`, `fires:sync` 등 리스너 등록/해제 |
| `useChat.ts` | `chat:join`/`chat:leave`, 메시지 리스너 등록/해제 |

이 구조 덕분에 각 훅이 마운트/언마운트되어도 소켓 연결 자체는 영향받지 않는다.

### 연결 상태 관리

`useSocketStore`(Zustand)가 연결 상태를 `idle → connecting → connected → reconnecting → offline` 상태 머신으로 관리한다. 이 상태는 UI에서 연결 표시기를 렌더링하는 데 사용된다. 재접속 시도 횟수(`reconnectAttempt`)도 추적하여 사용자에게 진행 상황을 보여줄 수 있다.

Socket.IO의 `reconnectionAttempts`는 8회로 제한되어 있으며, 모두 실패하면 `reconnect_failed` 이벤트에서 상태를 `offline`로 전환한다.

---

## 6. 오프라인 Emit 정책

### 끊긴 상태에서 emit은 드롭된다

`socketManager.ts`의 heartbeat 전송 코드에서 이 정책이 드러난다:

```typescript
if (socket.connected) socket.emit('heartbeat', { ts: Date.now() })
```

연결이 끊긴 상태에서는 emit을 시도하지 않고 조용히 무시한다. 이는 의도적인 설계이다.

### 큐잉하지 않는 이유

1. **데이터의 시간 민감성.** 불 이벤트와 위치 데이터는 본질적으로 "지금"의 정보이다. 30초 전의 `fire:ignite`를 재접속 후 뒤늦게 전송하면 서버의 상태와 불일치하며, 사용자 경험을 해칠 수 있다.

2. **서버가 진실의 원천(source of truth)이다.** 재접속 시 클라이언트는 `get_fires`로 전체 상태를 서버에서 다시 받아온다. 따라서 오프라인 동안 놓친 이벤트는 재접속 후 서버 상태 동기화로 자연스럽게 보정된다.

3. **단순성.** 오프라인 큐를 구현하면 순서 보장, 중복 방지, 만료 처리 등의 복잡성이 추가된다. 해커톤 프로젝트의 규모에서 이 복잡성은 정당화되지 않는다.

4. **채팅도 동일.** `useChat`의 `chat:leave`는 `socket.connected` 조건 하에서만 emit된다. 오프라인 상태에서 채팅방을 나가면 leave 메시지가 전송되지 않지만, 서버의 reaper가 stale 연결을 정리하면서 해당 클라이언트는 자동으로 room에서 제거된다.

---

## 요약: 설계 원칙

| 원칙 | 적용 |
|------|------|
| 필요한 데이터만 전달 | Viewport 기반 room 구독으로 불필요한 broadcast 제거 |
| 서버가 진실의 원천 | 재접속 시 서버에서 전체 상태 동기화, 오프라인 큐 불필요 |
| 관심사 분리 | 소켓 연결은 전역, 기능별 리스너는 각 훅에서 독립 관리 |
| 방어적 정리 | Heartbeat + reaper로 좀비 연결 방지, StrictMode 가드로 중복 연결 방지 |
| 생명주기 독립성 | 채팅 room과 viewport room을 분리하여 서로 간섭하지 않음 |
