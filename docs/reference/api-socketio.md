# Socket.IO API Reference

화르르 프로젝트의 Socket.IO 실시간 이벤트 전체 명세.

> **네이밍 규칙**: 서버 Python 코드는 `snake_case`, 클라이언트 TypeScript 코드는 `camelCase`를 사용한다.
> 호환용(mock server compat) 이벤트의 broadcast payload는 `camelCase` 키(`gridId`, `activeCount`)를 사용하고,
> 신규 이벤트의 payload는 `snake_case` 키(`grid_id`, `active_count`)를 사용한다.
> stage 변경 시 broadcast되는 `fire:update` payload에는 **양쪽 키가 모두** 포함된다.

---

## 목차

1. [연결 Lifecycle](#연결-lifecycle)
2. [Client → Server 이벤트](#client--server-이벤트)
3. [Server → Client 이벤트](#server--client-이벤트)
4. [호환용 이벤트](#호환용-이벤트-mock-server-compat)
5. [Room 구조](#room-구조)

---

## 연결 Lifecycle

```
Client                          Server
  │                               │
  ├─── connect (auth.user_id) ───>│  ConnectionManager.add(sid, user_id)
  │                               │  이전 세션 room 복원 (reconnect 시)
  │<─── "connected" (ack) ────────│  sid, heartbeat_interval, reconnect_count 등
  │<─── "users:count" ───────────│  전역 broadcast (접속자 수)
  │                               │
  ├─── "heartbeat" {ts} ────────>│  10초 간격, record_heartbeat(sid)
  │<─── ACK {server_ts} ─────────│
  │         ...반복...             │
  │                               │
  │    (30초간 heartbeat 없으면)    │
  │<─── disconnect (server reap) ─│  Reaper가 stale 연결 강제 해제
  │                               │
  ├─── disconnect ───────────────>│  ConnectionManager.remove(sid)
  │                               │  room 정리, users:count 재broadcast
```

**설정값** (서버 `connection_manager.go`):

| 상수 | 값 | 설명 |
|---|---|---|
| `HEARTBEAT_INTERVAL_SEC` | 10 | heartbeat 송신 간격 (클라이언트) |
| `HEARTBEAT_TIMEOUT_SEC` | 30 | 이 시간 동안 heartbeat 없으면 stale 판정 |
| `REAPER_INTERVAL_SEC` | 15 | stale 연결 검사 주기 |
| `ping_interval` | 10 | Engine.IO ping 간격 |
| `ping_timeout` | 5 | Engine.IO ping 응답 제한 |

**클라이언트 재접속 설정** (`socketManager.ts`):

| 설정 | 값 |
|---|---|
| `transports` | `['polling', 'websocket']` |
| `reconnectionAttempts` | 8 |
| `reconnectionDelay` | 500ms |
| `reconnectionDelayMax` | 5,000ms |
| `auth` | `{ user_id: <localStorage 영구 익명 ID> }` |

**연결 상태** (`ConnectionStatus` type):

`'idle'` → `'connecting'` → `'connected'` → `'reconnecting'` → `'offline'`

---

## Client → Server 이벤트

### `fire:ignite`

| 항목 | 내용 |
|---|---|
| **방향** | C → S |
| **페이로드** | `{ lat: float, lng: float, demo?: boolean }` |
| **설명** | 사용자가 지도에서 불을 놓는다. GPS 좌표를 grid ID로 변환 후 Redis에 fire event 등록. `demo: true`이고 좌표가 없으면 서버가 사전 정의 위치 중 랜덤 선택. |
| **Room 스코프** | N/A (요청자에게 ACK 반환) |
| **ACK 반환값** | 아래 참조 |

**ACK 성공 시:**
```typescript
{
  status: "ok"
  grid_id: string          // 실제 착화된 grid (spread 시 요청과 다를 수 있음)
  requested_grid_id: string
  event_id: string         // "fire-{uuid12}"
  active_count: number
  stage: number            // 0-4
  stage_info: {
    stage: number
    label_ko: string
    label_en: string
    triggers_firefighter: boolean
  }
  spread_path: { from: string, to: string }[]
  // demo 모드 추가 필드
  demo?: boolean
  lat?: number
  lng?: number
  demo_location?: string
}
```

**ACK 에러 시:**
```typescript
{ error: "lat and lng are required" }
{ error: "lat and lng must be numbers" }
```

**부수 효과:**
- `fire:ignite` → grid room broadcast (아래 S→C 참조)
- `fire:global_update` → 전역 broadcast
- stage 변경 시 engine이 `fire:update`, `fire:stage_transition` broadcast
- 인접 grid로 확산 시 `fire:spread` 전역 broadcast

---

### `subscribe:viewport`

| 항목 | 내용 |
|---|---|
| **방향** | C → S |
| **페이로드** | `{ ne_lat: float, ne_lng: float, sw_lat: float, sw_lng: float }` |
| **설명** | 현재 지도 viewport의 bounding box를 전송. 서버가 해당 영역의 grid room에 client를 join시킨다 (이전 viewport room은 자동 leave). |
| **Room 스코프** | N/A (room 구독 변경) |
| **ACK 반환값** | 아래 참조 |

**ACK 성공 시:**
```typescript
{
  status: "ok"
  subscribed_grids: number       // 구독된 grid 수
  active_fires: GridState[]      // 현재 viewport 내 활성 불 목록
}
```

`GridState`:
```typescript
{
  grid_id: string
  lat?: number
  lng?: number
  active_count: number
  stage: number
  stage_info: { stage: number, label_ko: string, label_en: string, triggers_firefighter: boolean }
}
```

**ACK 에러 시:**
```typescript
{ error: "ne_lat, ne_lng, sw_lat, sw_lng are required numbers" }
```

---

### `fire:state`

| 항목 | 내용 |
|---|---|
| **방향** | C → S |
| **페이로드** | `{ grid_ids?: string[] }` |
| **설명** | 특정 grid들의 현재 불 상태를 요청. `grid_ids` 생략 시 전체 활성 grid 반환. |
| **Room 스코프** | N/A (요청자에게 ACK 반환) |
| **ACK 반환값** | 아래 참조 |

**ACK:**
```typescript
{
  status: "ok"
  grids: GridState[]
  total_active_grids: number
}
```

---

### `heartbeat`

| 항목 | 내용 |
|---|---|
| **방향** | C → S |
| **페이로드** | `{ ts: number }` (클라이언트 timestamp, ms) |
| **설명** | 10초마다 클라이언트가 전송하는 앱 레벨 heartbeat. 서버가 `last_heartbeat`를 갱신하여 stale 판정에 사용. |
| **Room 스코프** | N/A |
| **ACK 반환값** | `{ status: "ok", server_ts: number, client_ts: number \| null }` 또는 `{ error: "unknown_sid" }` |

---

### `chat:join`

| 항목 | 내용 |
|---|---|
| **방향** | C → S |
| **페이로드** | `{}` (빈 객체) |
| **설명** | 글로벌 채팅방(`chat:global`) 입장. Redis에서 최근 히스토리 로드 후 반환. |
| **Room 스코프** | `chat:global` room에 join |
| **ACK 반환값** | 아래 참조 |

**ACK 성공 시:**
```typescript
{
  status: "ok"
  history: ChatMessage[]   // 최근 50개, 시간순
  presence: number         // 현재 채팅방 참가자 수
}
```

**ACK 에러 시:**
```typescript
{ error: "unknown_sid" }
```

**부수 효과:** `chat:presence` → `chat:global` room broadcast

---

### `chat:send`

| 항목 | 내용 |
|---|---|
| **방향** | C → S |
| **페이로드** | 아래 참조 |
| **설명** | 채팅 메시지 전송. 서버가 검증 후 Redis에 저장하고 `chat:message`로 room broadcast. |
| **Room 스코프** | N/A (요청자에게 ACK, 메시지는 room broadcast) |
| **ACK 반환값** | `{ status: "ok", id: string }` 또는 에러 |

**페이로드:**
```typescript
{
  text: string          // 1~300자
  user_id: string       // 필수
  nickname?: string     // 기본값 "익명"
  avatar?: string       // 기본값 "👤"
  avatar_bg?: string    // 기본값 "#333"
  name_color?: string   // 기본값 "#fff"
}
```

**ACK 에러 코드:**
| 에러 | 조건 |
|---|---|
| `invalid_payload` | data가 dict가 아님 |
| `text_too_short` | text 길이 < 1 |
| `text_too_long` | text 길이 > 300 |
| `user_id_required` | user_id 누락 또는 문자열 아님 |

**부수 효과:** `chat:message` → `chat:global` room broadcast

---

### `chat:leave`

| 항목 | 내용 |
|---|---|
| **방향** | C → S |
| **페이로드** | `{}` (빈 객체) |
| **설명** | 채팅방 퇴장. room에서 leave 후 참가자 수 재broadcast. |
| **Room 스코프** | `chat:global` room에서 leave |
| **ACK 반환값** | `{ status: "ok" }` |

**부수 효과:** `chat:presence` → `chat:global` room broadcast

---

## Server → Client 이벤트

### `connected`

| 항목 | 내용 |
|---|---|
| **방향** | S → C |
| **페이로드** | 아래 참조 |
| **설명** | 연결 직후 해당 클라이언트에게만 전송되는 welcome 메시지. |
| **Room 스코프** | 해당 client만 (`to=sid`) |

```typescript
{
  sid: string
  connected_at: number          // Unix timestamp
  active_connections: number    // 현재 전체 접속자 수
  heartbeat_interval: number    // 10 (초)
  reconnect_count: number       // 이 user_id의 재접속 횟수
  restored_rooms: number        // 복원된 room 수
}
```

---

### `users:count`

| 항목 | 내용 |
|---|---|
| **방향** | S → C |
| **페이로드** | `{ count: number }` |
| **설명** | 전체 접속자 수 변경 시 전역 broadcast. connect/disconnect 시마다 발생. |
| **Room 스코프** | 전역 broadcast (모든 클라이언트) |

---

### `fire:update`

| 항목 | 내용 |
|---|---|
| **방향** | S → C |
| **페이로드** | 아래 참조 |
| **설명** | grid의 불 상태가 변경될 때 broadcast. stage 변경 시와 소방관 진압으로 count 감소 시 발생. |
| **Room 스코프** | grid room + 전역 broadcast (이중 전송) |

**페이로드 (stage 변경 시 -- `snake_case` + `camelCase` 혼용):**
```typescript
{
  // snake_case (신규)
  grid_id: string
  active_count: number
  stage: number
  stage_info: { stage: number, label_ko: string, label_en: string, triggers_firefighter: boolean }
  lat?: number
  lng?: number
  timestamp: number

  // camelCase (호환용, 동일 payload에 함께 포함)
  gridId: string
  activeCount: number
}
```

> 호환용 `fire` 이벤트에서 발생하는 `fire:update`는 camelCase 키만 포함한다. (아래 [호환용 이벤트](#호환용-이벤트-mock-server-compat) 참조)

---

### `fire:ignite` (broadcast)

| 항목 | 내용 |
|---|---|
| **방향** | S → C |
| **페이로드** | 아래 참조 |
| **설명** | 새 불이 착화되었을 때 해당 grid room에 broadcast. ACK와는 별도로, 해당 grid를 구독 중인 모든 client가 즉시 수신. |
| **Room 스코프** | grid room (`grid_id`로 결정) |

```typescript
{
  grid_id: string
  event_id: string
  lat: number
  lng: number
  active_count: number
  stage: number
  stage_info: { stage: number, label_ko: string, label_en: string, triggers_firefighter: boolean }
  ignited_by: string       // 착화한 client의 sid
  timestamp: number
}
```

---

### `fire:global_update`

| 항목 | 내용 |
|---|---|
| **방향** | S → C |
| **페이로드** | 아래 참조 |
| **설명** | 불 착화 시마다 전역 broadcast. 지도 전체 보기(overview) 갱신용. `fire:ignite` 핸들러 및 REST API에서 발생. |
| **Room 스코프** | 전역 broadcast |

```typescript
{
  grid_id: string
  active_count: number
  stage: number
  stage_info: { stage: number, label_ko: string, label_en: string, triggers_firefighter: boolean }
  timestamp: number
}
```

---

### `fire:stage_transition`

| 항목 | 내용 |
|---|---|
| **방향** | S → C |
| **페이로드** | 아래 참조 |
| **설명** | 불 stage가 변경될 때 broadcast. 클라이언트 애니메이션 전환에 사용. |
| **Room 스코프** | grid room + 전역 broadcast (이중 전송) |

```typescript
{
  grid_id: string
  active_count: number
  prev_stage: number       // 이전 stage (0-4)
  new_stage: number        // 새 stage (0-4)
  stage_info: { stage: number, label_ko: string, label_en: string, triggers_firefighter: boolean }
  timestamp: number
}
```

---

### `fire:spread`

| 항목 | 내용 |
|---|---|
| **방향** | S → C |
| **페이로드** | 아래 참조 |
| **설명** | 불이 인접 grid로 확산되었을 때 전역 broadcast. 클라이언트에서 확산 궤적 애니메이션에 사용. |
| **Room 스코프** | 전역 broadcast |

```typescript
{
  path: { from: string, to: string }[]
  event_id: string
  timestamp: number
}
```

---

### `firefighter:spawn`

| 항목 | 내용 |
|---|---|
| **방향** | S → C |
| **페이로드** | 아래 참조 |
| **설명** | stage 4(대화재) 이상 진입 시 소방관 NPC 출현 이벤트. 시각 효과 전용, 실제 진압 로직 없음. |
| **Room 스코프** | grid room + 전역 broadcast (이중 전송) |

```typescript
{
  npc_id: string
  grid_id: string
  status: "dispatched"
  dispatched_at: number      // Unix timestamp
  fires_removed: 0           // 항상 0 (시각 효과 전용)
  remove_per_sweep: number
  target_stage: number       // 출현 트리거 stage
}
```

---

### `chat:message`

| 항목 | 내용 |
|---|---|
| **방향** | S → C |
| **페이로드** | 아래 참조 |
| **설명** | 새 채팅 메시지가 도착했을 때 채팅방 전체에 broadcast. |
| **Room 스코프** | `chat:global` room |

```typescript
{
  id: string              // "msg-{uuid12}"
  user_id: string
  nickname: string
  avatar: string
  avatar_bg: string
  name_color: string
  text: string
  timestamp: number       // Unix timestamp
}
```

---

### `chat:presence`

| 항목 | 내용 |
|---|---|
| **방향** | S → C |
| **페이로드** | `{ count: number }` |
| **설명** | 채팅방 참가자 수 변경 시 broadcast. join/leave/disconnect 시마다 발생. |
| **Room 스코프** | `chat:global` room |

---

## 호환용 이벤트 (Mock Server Compat)

초기 mock server와의 호환성을 위해 유지되는 이벤트. 신규 개발에서는 사용을 지양한다.

### `fire` (호환용)

| 항목 | 내용 |
|---|---|
| **방향** | C → S |
| **페이로드** | `{ lat: float, lng: float }` |
| **설명** | **호환용.** `fire:ignite`와 동일한 착화 로직 수행 후, camelCase 형식의 `fire:update`를 전역 broadcast. ACK 반환 없음. |
| **Room 스코프** | N/A |
| **ACK 반환값** | 없음 (void) |

**부수 효과** -- camelCase 전용 `fire:update` broadcast:
```typescript
{
  gridId: string
  activeCount: number
  stage: number
  lat: number
  lng: number
}
```

---

### `get_fires` (호환용)

| 항목 | 내용 |
|---|---|
| **방향** | C → S |
| **페이로드** | `{}` 또는 `null` |
| **설명** | **호환용.** 모든 활성 불 목록 요청. 응답은 `fires:sync` 이벤트로 요청자에게만 전송. |
| **Room 스코프** | N/A |
| **ACK 반환값** | 없음 (`fires:sync` 이벤트로 응답) |

---

### `fires:sync` (호환용)

| 항목 | 내용 |
|---|---|
| **방향** | S → C |
| **페이로드** | 아래 참조 |
| **설명** | **호환용.** `get_fires` 요청에 대한 응답. 요청 client에게만 전송. |
| **Room 스코프** | 해당 client만 (`to=sid`) |

```typescript
// 배열 형태 (wrapper 객체 없음)
[
  {
    gridId: string
    activeCount: number
    stage: number
  },
  ...
]
```

> `useFireSocket.ts`는 connect 시마다 `get_fires`를 emit하고 `fires:sync`를 수신하여 초기 동기화를 수행한다.

---

## Room 구조

| Room | 용도 | 가입 방법 |
|---|---|---|
| `<grid_id>` (예: `41740:115434`) | viewport 기반 불 이벤트 구독 | `subscribe:viewport` 이벤트 |
| `chat:global` | 글로벌 익명 채팅 | `chat:join` 이벤트 |

- **Grid room**: `subscribe:viewport` 호출 시 이전 viewport의 모든 grid room에서 leave하고 새 viewport의 grid room에 join한다. 한 client는 동시에 하나의 viewport만 구독.
- **Chat room**: `chat:join`으로 입장, `chat:leave`로 퇴장. disconnect 시 자동 퇴장 처리.
- Reconnect 시 `auth.user_id`를 기반으로 이전 세션의 room 구독이 자동 복원된다.

---

## 이벤트 요약 표

### Client → Server

| 이벤트 | 페이로드 요약 | ACK | 비고 |
|---|---|---|---|
| `fire:ignite` | `{ lat, lng, demo? }` | `{ status, grid_id, stage, ... }` | |
| `subscribe:viewport` | `{ ne_lat, ne_lng, sw_lat, sw_lng }` | `{ status, subscribed_grids, active_fires }` | |
| `fire:state` | `{ grid_ids? }` | `{ status, grids, total_active_grids }` | |
| `heartbeat` | `{ ts }` | `{ status, server_ts, client_ts }` | 10초 간격 |
| `chat:join` | `{}` | `{ status, history, presence }` | |
| `chat:send` | `{ text, user_id, nickname?, ... }` | `{ status, id }` | |
| `chat:leave` | `{}` | `{ status }` | |
| `fire` | `{ lat, lng }` | 없음 | **호환용** |
| `get_fires` | `{}` | 없음 (`fires:sync`로 응답) | **호환용** |

### Server → Client

| 이벤트 | 페이로드 요약 | 스코프 | 비고 |
|---|---|---|---|
| `connected` | `{ sid, heartbeat_interval, reconnect_count, ... }` | 해당 client | 연결 직후 |
| `users:count` | `{ count }` | 전역 | connect/disconnect 시 |
| `fire:update` | `{ grid_id, active_count, stage, gridId, activeCount, ... }` | grid room + 전역 | snake/camelCase 혼용 |
| `fire:ignite` | `{ grid_id, event_id, lat, lng, ignited_by, ... }` | grid room | 착화 즉시 |
| `fire:global_update` | `{ grid_id, active_count, stage, ... }` | 전역 | 매 착화마다 |
| `fire:stage_transition` | `{ grid_id, prev_stage, new_stage, ... }` | grid room + 전역 | stage 변경 시 |
| `fire:spread` | `{ path, event_id, timestamp }` | 전역 | 인접 grid 확산 시 |
| `firefighter:spawn` | `{ npc_id, grid_id, status, ... }` | grid room + 전역 | stage 4+ 진입 시 |
| `chat:message` | `{ id, user_id, nickname, text, ... }` | `chat:global` room | |
| `chat:presence` | `{ count }` | `chat:global` room | join/leave/disconnect 시 |
| `fires:sync` | `[{ gridId, activeCount, stage }, ...]` | 해당 client | **호환용** |
