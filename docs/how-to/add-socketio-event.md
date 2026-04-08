# Socket.IO 이벤트 추가 가이드

화르르(hwarr) 프로젝트에 새로운 Socket.IO 이벤트를 추가하는 단계별 가이드.

기존 패턴(fire 이벤트, chat 이벤트)을 따르며, 서버(Go)와 클라이언트(TypeScript) 양쪽 작업을 다룬다.

---

## 목차

1. [서버: 이벤트 핸들러 작성](#1-서버-이벤트-핸들러-작성)
2. [서버: server.go에서 핸들러 등록](#2-서버-servergo에서-핸들러-등록)
3. [클라이언트: feature 훅에서 리스너 등록](#3-클라이언트-feature-훅에서-리스너-등록)
4. [클라이언트: Zustand 스토어에 상태/액션 추가](#4-클라이언트-zustand-스토어에-상태액션-추가)
5. [테스트 방법](#5-테스트-방법)
6. [API 문서 업데이트](#6-api-문서-업데이트)

---

## 1. 서버: 이벤트 핸들러 작성

`server/internal/sio/` 디렉토리 아래에 이벤트 핸들러 파일을 작성한다. 기존 핸들러(`fire_ignite.go`, `chat_send.go`)와 동일한 패턴을 따른다.

### 파일 생성

```
server/internal/sio/<feature>_<action>.go
```

### 템플릿

```go
package sio

import (
	"log"

	socketio "github.com/homeworldio/socketio-go"
)

// register<Feature><Action> registers the <feature>:<action> event handler.
func register<Feature><Action>(
	server *socketio.Server,
	manager *ConnectionManager,
	logger *log.Logger,
) {
	server.On("<feature>:<action>", func(s *socketio.Socket, data map[string]any) map[string]any {
		// ── 입력 검증 ──
		value, ok := data["field"].(string)
		if !ok {
			logger.Printf("<feature>:<action> from %s missing field: %v", s.ID(), data)
			return map[string]any{"error": "field is required"}
		}

		// ── 비즈니스 로직 ──
		resultPayload := map[string]any{
			"field": value,
		}

		// ── Broadcast ──
		server.To("<room_id>").Emit("<feature>:<event>", resultPayload)

		logger.Printf("<feature>:<action> sid=%s field=%s", s.ID(), value)

		// ── Ack 응답 ──
		return map[string]any{"status": "ok", "field": value}
	})
}
```

### 핵심 규칙

| 항목 | 규칙 |
|------|------|
| 이벤트 네이밍 | `<feature>:<action>` 형식 (예: `chat:send`, `fire:ignite`) |
| 입력 검증 | type assertion으로 안전하게 추출, 실패 시 `{"error": "..."}` 반환 |
| 로깅 | `logger.Printf()` 로 기록 |
| 반환값 | `map[string]any` — callback ack로 클라이언트에 전달됨 |
| Broadcast 방식 | `server.Emit()` (전체) / `server.To(room).Emit()` (room) |

---

## 2. 서버: server.go에서 핸들러 등록

`server/internal/server/server.go`의 `registerSocketEvents()` 함수에서 핸들러를 등록한다.

### 등록 호출 추가

```go
func registerSocketEvents(
	server *socketio.Server,
	manager *sio.ConnectionManager,
	redisClient *hredis.Client,
	resolver *geodata.Resolver,
	logger *log.Logger,
) {
	// 기존 핸들러 등록
	sio.RegisterFireEvents(server, manager, redisClient, logger)
	sio.RegisterChatEvents(server, manager, redisClient, logger)

	// 새 핸들러 등록 추가
	register<Feature><Action>(server, manager, logger)
}
```

### 주의사항

- Redis client나 engine 등 의존성이 필요하면, 해당 객체가 초기화된 **이후에** 등록 함수를 호출해야 한다.

---

## 3. 클라이언트: feature 훅에서 리스너 등록

`client/src/features/<feature>/hooks/` 디렉토리에 Socket.IO 리스너 훅을 만든다.

### 패턴 A: 항상 활성 (fire 패턴)

컴포넌트 마운트 시 자동으로 리스너를 등록하고, 언마운트 시 해제한다.

```typescript
/**
 * <feature> 이벤트 구독 훅
 *
 * 소켓 자체의 연결/해제는 전역 SocketProvider가 담당한다.
 * 이 훅은 리스너만 등록/해제한다.
 */

import { useEffect } from 'react'
import { socket } from '../../../lib/socket'
import { use<Feature>Store } from '../stores/<feature>Store'

interface <Feature>Payload {
  field: string
  // ...
}

export function use<Feature>Socket() {
  const updateData = use<Feature>Store((s) => s.updateData)

  useEffect(() => {
    // ── 리스너 정의 ──
    const requestSync = () => {
      socket.emit('<feature>:sync', {})
    }
    const onUpdate = (data: <Feature>Payload) => updateData(data)

    // 이미 연결되어 있으면 즉시 동기화 요청
    if (socket.connected) requestSync()
    // 재접속 시 자동 재동기화
    socket.on('connect', requestSync)
    socket.on('<feature>:<event>', onUpdate)

    // ── Cleanup: 반드시 on/off 쌍을 맞춘다 ──
    return () => {
      socket.off('connect', requestSync)
      socket.off('<feature>:<event>', onUpdate)
    }
  }, [updateData])
}
```

### 패턴 B: 조건부 활성 (chat 패턴)

`active` 파라미터로 구독 여부를 제어한다. 비활성화 시 서버에 leave를 보내고 리스너를 해제한다.

```typescript
import { useEffect } from 'react'
import { socket } from '../../lib/socket'
import { use<Feature>Store } from './<feature>Store'

interface JoinAck {
  status?: string
  history?: <Feature>Item[]
  error?: string
}

export function use<Feature>(active: boolean) {
  const setData = use<Feature>Store((s) => s.setData)
  const setStatus = use<Feature>Store((s) => s.setStatus)
  const reset = use<Feature>Store((s) => s.reset)

  useEffect(() => {
    if (!active) return

    const emitJoin = () => {
      socket.emit('<feature>:join', {}, (ack: JoinAck) => {
        if (ack?.error) {
          console.warn('[<feature>] join failed:', ack.error)
          return
        }
        if (Array.isArray(ack?.history)) setData(ack.history)
      })
    }

    const onConnect = () => {
      setStatus('connected')
      emitJoin()
    }
    const onDisconnect = () => setStatus('connecting')
    const onUpdate = (data: <Feature>Item) => {
      use<Feature>Store.getState().appendItem(data)
    }

    socket.on('connect', onConnect)
    socket.on('disconnect', onDisconnect)
    socket.on('<feature>:update', onUpdate)

    if (socket.connected) {
      setStatus('connected')
      emitJoin()
    } else {
      setStatus('connecting')
    }

    return () => {
      if (socket.connected) {
        socket.emit('<feature>:leave', {})
      }
      socket.off('connect', onConnect)
      socket.off('disconnect', onDisconnect)
      socket.off('<feature>:update', onUpdate)
      reset()
    }
  }, [active, setData, setStatus, reset])
}
```

### 핵심 규칙

| 항목 | 규칙 |
|------|------|
| `socket.on` / `socket.off` 쌍 | cleanup 함수에서 **반드시** 모든 `on`에 대응하는 `off`를 호출한다 |
| 콜백 참조 | `on`과 `off`에 **같은 함수 참조**를 전달해야 한다 (인라인 함수 사용 금지) |
| 재접속 처리 | `socket.on('connect', ...)` 로 reconnect 시 자동 재동기화 |
| deps 배열 | Zustand selector로 가져온 액션들을 `useEffect` deps에 포함한다 |
| socket import | 항상 `lib/socket.ts`의 싱글톤 인스턴스를 사용한다 |

---

## 4. 클라이언트: Zustand 스토어에 상태/액션 추가

`client/src/features/<feature>/stores/` 디렉토리에 Zustand 스토어를 만든다.

### 템플릿

```typescript
/**
 * <Feature> 상태 관리 (Zustand)
 *
 * Socket 이벤트 → Zustand 스토어 → React 컴포넌트 리렌더
 * (컴포넌트가 직접 소켓을 구독하지 않음 → 관심사 분리)
 */

import { create } from 'zustand'

export interface <Feature>Item {
  id: string
  // ... 도메인 필드
}

interface <Feature>State {
  // ── 상태 ──
  items: Map<string, <Feature>Item>  // 또는 배열
  status: 'connecting' | 'connected' | 'disconnected'

  // ── 액션 ──
  updateItem: (item: <Feature>Item) => void
  setItems: (items: <Feature>Item[]) => void
  reset: () => void
}

export const use<Feature>Store = create<<Feature>State>((set) => ({
  items: new Map(),
  status: 'connecting',

  updateItem: (item) =>
    set((state) => {
      const next = new Map(state.items)
      next.set(item.id, item)
      return { items: next }
    }),

  setItems: (items) =>
    set(() => {
      const next = new Map<string, <Feature>Item>()
      for (const item of items) {
        next.set(item.id, item)
      }
      return { items: next }
    }),

  reset: () => set({ items: new Map(), status: 'connecting' }),
}))
```

### 핵심 규칙

| 항목 | 규칙 |
|------|------|
| immutable 업데이트 | `Map`은 `new Map(state.items)` 으로 복사 후 변경 |
| O(1) 조회가 필요하면 | `Map<id, Item>` 사용 (fire 패턴) |
| 순서가 중요하면 | 배열 사용 (chat 패턴) |
| 외부 스토어 접근 | 훅 바깥에서 `useStore.getState()` 로 접근 가능 |

---

## 5. 테스트 방법

### 5-1. 서버 단위 테스트 (go test)

기존 테스트 파일(`fire_ignite_test.go`, `chat_send_test.go`)과 동일한 패턴으로 테스트를 작성한다. 테스트 파일은 핸들러와 같은 디렉토리에 `<feature>_<action>_test.go`로 생성한다.

```bash
cd server && go test ./internal/sio/ -v -run Test<Feature>
```

### 5-2. 수동 테스트 (브라우저 DevTools)

서버를 로컬에서 실행한 뒤, 브라우저 콘솔에서 직접 이벤트를 주고받을 수 있다.

```javascript
// 브라우저 콘솔에서 socket 인스턴스에 접근
// (React DevTools 또는 window 전역 노출 필요)

// 이벤트 전송 + ack 확인
socket.emit('<feature>:<action>', { field: 'value' }, (ack) => {
  console.log('ack:', ack)
})

// 서버 broadcast 수신 확인
socket.on('<feature>:<event>', (data) => {
  console.log('received:', data)
})
```

### 5-3. 통합 테스트

브라우저 또는 Socket.IO 클라이언트 라이브러리를 사용하여 `http://localhost:8000`에 연결 후 이벤트를 주고받아 테스트한다.

---

## 6. API 문서 업데이트

새 이벤트를 추가한 후 반드시 `docs/reference/api-socketio.md`를 업데이트한다.

### 추가할 내용

각 이벤트에 대해 다음 항목을 문서화한다:

```markdown
### `<feature>:<action>`

**방향:** Client → Server (또는 Server → Client)

**설명:** <이벤트가 하는 일>

**Client payload:**
| 필드 | 타입 | 필수 | 설명 |
|------|------|------|------|
| field | string | O | 필드 설명 |

**Server ack:**
```json
{
  "status": "ok",
  "field": "value"
}
```

**Broadcast:** `<feature>:<event>` → <대상 설명>
```

---

## 체크리스트

새 Socket.IO 이벤트를 추가할 때 아래 항목을 모두 확인한다:

- [ ] `server/internal/sio/<feature>_<action>.go` 에 핸들러 작성
- [ ] `server/internal/server/server.go`에서 핸들러 등록
- [ ] 클라이언트 feature 훅에서 `socket.on()`/`socket.off()` 쌍 등록
- [ ] Zustand 스토어에 상태 및 액션 추가
- [ ] `on`/`off` 에 동일한 함수 참조 사용 확인
- [ ] `useEffect` cleanup에서 모든 리스너 해제 확인
- [ ] 재접속(`connect` 이벤트) 시 자동 동기화 처리
- [ ] 서버 입력 검증 및 에러 응답 처리
- [ ] Go 단위 테스트 작성 (`<feature>_<action>_test.go`)
- [ ] `docs/reference/api-socketio.md` 업데이트
