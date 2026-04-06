# Frontend 데이터 흐름: Socket → Zustand → React

화르르 frontend는 **Socket.IO → Zustand store → React component** 단방향 데이터 흐름을 따른다.
이 문서는 이 패턴의 설계 의도와 각 계층의 역할을 설명한다.

---

## 전체 데이터 흐름 다이어그램

```
┌─────────────┐
│   Server     │
│  (Socket.IO) │
└──────┬───────┘
       │  fire:update, fires:sync, users:count,
       │  fire:spread, chat:message, chat:presence
       ▼
┌──────────────────┐
│  socketManager.ts │  싱글턴 소켓 인스턴스 + 연결 수명주기
│  (startSocket)    │  heartbeat, reconnect, status 관리
└──────┬───────────┘
       │  socket (공유 인스턴스)
       ▼
┌──────────────────────────────────────────────┐
│  구독 훅 (listener-only)                       │
│  ┌─────────────────┐  ┌──────────────────┐   │
│  │ useFireSocket   │  │ useChat          │   │
│  │ on/off listener │  │ on/off listener  │   │
│  └────────┬────────┘  └────────┬─────────┘   │
└───────────┼─────────────────────┼────────────┘
            │                     │
            ▼                     ▼
┌────────────────────┐  ┌──────────────────┐
│  fireStore         │  │  useChatStore    │
│  (Zustand)         │  │  (Zustand)       │
│  fires: Map        │  │  messages: []    │
│  onlineUsers       │  │  presenceCount   │
└────────┬───────────┘  └────────┬─────────┘
         │ cross-store call      │
         ▼                       │
┌────────────────────┐           │
│  animationStore    │           │
│  (Zustand)         │           │
│  explosions        │           │
│  trajectories      │           │
│  matches           │           │
└────────┬───────────┘           │
         │                       │
         ▼                       ▼
┌──────────────────────────────────────────────┐
│  React Components                             │
│  selector로 필요한 slice만 구독 → 최소 리렌더  │
└──────────────────────────────────────────────┘
```

---

## 1. 컴포넌트가 socket을 직접 구독하지 않는 이유

### 관심사 분리

컴포넌트가 `socket.on('fire:update', ...)` 을 직접 호출하면 **네트워크 프로토콜 세부사항**(이벤트명, payload 형태, 재접속 처리)이 UI 계층에 노출된다.
화르르는 이를 방지하기 위해 **소켓 이벤트를 Zustand store에 먼저 기록**하고, 컴포넌트는 store의 상태만 읽는다.

```
// 컴포넌트는 소켓을 모른다. store selector만 사용한다.
const fires = useFireStore((s) => s.fires)
const onlineUsers = useFireStore((s) => s.onlineUsers)
```

이 구조에서 소켓 프로토콜이 변경되더라도(예: WebSocket에서 SSE로 전환) 컴포넌트 코드를 수정할 필요가 없다.

### 리렌더 최적화

Socket.IO 이벤트는 초당 수십 회 발생할 수 있다. 컴포넌트가 이벤트마다 `setState`를 호출하면 **불필요한 리렌더**가 폭증한다.
Zustand store를 중간 계층으로 두면:

- **selector 기반 구독**: 컴포넌트는 `useFireStore((s) => s.fires)` 처럼 필요한 slice만 구독하므로, 관련 없는 상태 변경 시 리렌더가 발생하지 않는다.
- **batch 처리**: `syncFires()`는 전체 불 목록을 한 번의 `set()` 호출로 교체하여 Map 재생성을 1회로 제한한다.
- **React 외부 업데이트**: Zustand의 `getState()` 를 통해 React 트리 바깥에서 상태를 갱신할 수 있어, 소켓 콜백 내에서 안전하게 사용 가능하다.

---

## 2. useFireSocket 훅: 리스너 등록/해제 전용 패턴

`useFireSocket`은 소켓의 **connect/disconnect를 전혀 건드리지 않는다**. 오직 이벤트 리스너의 등록(`socket.on`)과 해제(`socket.off`)만 수행한다.

```typescript
// useFireSocket.ts (핵심 구조)
useEffect(() => {
  socket.on('fire:update', onFireUpdate)
  socket.on('fires:sync', onFiresSync)
  socket.on('users:count', onUsersCount)
  socket.on('fire:spread', onFireSpread)

  return () => {
    socket.off('fire:update', onFireUpdate)
    socket.off('fires:sync', onFiresSync)
    socket.off('users:count', onUsersCount)
    socket.off('fire:spread', onFireSpread)
  }
}, [updateFire, syncFires, setOnlineUsers])
```

이 패턴의 장점:

| 관점 | 설명 |
|------|------|
| **수명주기 분리** | 소켓 연결은 앱 전체 수명, 리스너는 feature 컴포넌트 mount 수명을 따른다 |
| **안전한 cleanup** | 컴포넌트 unmount 시 리스너만 해제되므로 다른 기능의 소켓 사용에 영향 없음 |
| **재접속 대응** | `socket.on('connect', requestSync)` 로 재접속 시 자동 동기화 요청 |

채팅(`useChat`)도 동일한 패턴을 따르되, `active` flag에 따라 `chat:join` / `chat:leave` room 관리를 추가로 수행한다.

---

## 3. socketManager.ts의 싱글턴 관리와 StrictMode 방어

### 싱글턴 보장

`socketManager.ts`는 모듈 스코프에서 소켓 인스턴스를 생성하고 `autoConnect: false`로 설정한다.
실제 연결은 `startSocket()` 호출 시에만 시작된다.

```typescript
export const socket: Socket = io(SOCKET_URL, {
  autoConnect: false,
  // ...
})

let started = false

export function startSocket(): void {
  if (started) return   // 가드
  started = true
  // 이벤트 핸들러 등록 + socket.connect()
}
```

### StrictMode 더블마운트 방어

React 18의 StrictMode는 development 환경에서 `useEffect`를 **mount → unmount → mount** 순서로 두 번 실행한다.
`SocketProvider`는 `useEffect` 안에서 `startSocket()`을 호출하는데, 모듈 변수 `started`가 두 번째 호출을 차단한다.

```typescript
// SocketProvider.tsx
useEffect(() => {
  startSocket()
  // cleanup에서 disconnect 하지 않음 — 앱 수명 동안 유지
}, [])
```

cleanup에서 `stopSocket()`을 호출하지 않는 것이 핵심이다.
만약 cleanup에서 disconnect했다면, StrictMode의 unmount → remount 사이에 연결이 끊겨버리기 때문이다.
`started` 가드와 cleanup 생략의 조합으로 **StrictMode에서도 소켓이 정확히 1회만 연결**된다.

### heartbeat 메커니즘

서버는 30초 timeout으로 죽은 연결을 정리(reaper)한다. 클라이언트는 10초 간격으로 `heartbeat` 이벤트를 송신하여 연결 유지를 알린다.
connect 시 heartbeat를 즉시 1회 보내어, 서버의 `last_heartbeat`이 connect 직후부터 갱신되도록 한다.

---

## 4. 크로스 스토어 패턴: fireStore → animationStore

`fireStore.updateFire()`는 격자의 `stage`가 5(전소)에 도달하면 `animationStore.triggerExplosion()`을 호출한다.

```typescript
// fireStore.ts — updateFire 내부
updateFire: (cell) =>
  set((state) => {
    const next = new Map(state.fires)
    if (cell.activeCount > 0) {
      next.set(cell.gridId, cell)
      if (cell.stage >= 5) {
        useAnimationStore.getState().triggerExplosion(cell.gridId)
      }
    } else {
      next.delete(cell.gridId)
    }
    return { fires: next }
  }),
```

### 왜 store 간 직접 호출인가

- **이벤트 버스 불필요**: 폭발은 `stage >= 5`라는 명확한 조건에서만 발생한다. 별도의 이벤트 시스템을 두면 오히려 추적이 어려워진다.
- **동기 실행**: `getState()`는 동기적이므로 fire 상태 갱신과 폭발 트리거가 같은 tick에서 일어난다.
- **중복 방지**: `animationStore.triggerExplosion()`은 내부의 `explodedGrids` Set을 확인하여 동일 격자에 대한 이중 폭발을 차단한다.

### 불 확산 궤적도 동일 패턴

`useFireSocket`은 `fire:spread` 이벤트 수신 시 `animationStore.addTrajectory()`를 호출하여 확산 궤적 애니메이션을 추가한다.
이 역시 `getState()`를 통한 크로스 스토어 호출이다.

```typescript
const onFireSpread = (data: FireSpreadPayload) => {
  const addTrajectory = useAnimationStore.getState().addTrajectory
  for (const segment of data.path) {
    addTrajectory(segment.from, segment.to)
  }
}
```

---

## 요약

| 계층 | 파일 | 역할 |
|------|------|------|
| **연결 관리** | `socketManager.ts` | 싱글턴 소켓, heartbeat, reconnect, 연결 상태 store |
| **연결 시작** | `SocketProvider.tsx` | 앱 mount 시 `startSocket()` 1회 호출 |
| **이벤트 구독** | `useFireSocket.ts`, `useChat.ts` | 리스너 on/off만 담당, 연결 자체는 건드리지 않음 |
| **상태 저장** | `fireStore.ts`, `useChatStore.ts` | 소켓 이벤트를 Zustand 상태로 변환 |
| **파생 상태** | `animationStore.ts` | 크로스 스토어 호출로 폭발/궤적 애니메이션 관리 |
| **UI** | React components | selector로 필요한 slice만 구독하여 렌더 |
