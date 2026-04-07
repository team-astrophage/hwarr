# 화르르(Hwarr) Chat Feature Specification

## 1. 기능 개요

전국 익명 실시간 채팅 기능. 서버 계정 없이 디바이스 단위로 익명 아이덴티티를 부여하여, 지도 위 바텀시트 형태로 전국 사용자가 실시간으로 대화할 수 있다. Socket.IO 기반 양방향 통신을 사용하며, 메시지는 Redis에 최대 100개 / 1시간 TTL로 임시 보관된다.

- **채팅 유형**: 전국 단일 글로벌 룸 (`chat:global`)
- **익명성**: 서버 계정 불필요, 디바이스당 동물 캐릭터 자동 배정
- **메시지 보관**: 서버 Redis 기준 최대 100개, 1시간 후 자동 삭제
- **클라이언트 메시지 캡**: 최대 200개 유지 (오래된 것부터 삭제)

---

## 2. 익명 아이덴티티 시스템

> 소스: `client/src/features/chat/identity.ts`

### 2.1 개요

디바이스당 1회 동물 이모지 + 색상 조합을 무작위로 뽑아 `localStorage`에 영속화한다. 서버 계정 없이 "같은 사람"을 식별하기 위한 최소 수단이다.

### 2.2 `ChatIdentity` interface

| 필드 | 타입 | 설명 |
|------|------|------|
| `userId` | `string` | 디바이스 고유 식별자 (UUID v4 또는 fallback) |
| `nickname` | `string` | 표시 닉네임 (`"익명의 {동물}"` 형식) |
| `avatar` | `string` | 동물 이모지 |
| `avatarBg` | `string` | 아바타 배경색 hex |
| `nameColor` | `string` | 닉네임 텍스트 색상 hex |

### 2.3 동물 프리셋 (12종)

| # | 이름 | 이모지 | 배경색 (`avatarBg`) | 텍스트 색상 (`nameColor`) |
|---|------|--------|---------------------|---------------------------|
| 1 | 여우 | `🦊` | `#3a2d5c` | `#a78bfa` |
| 2 | 곰 | `🐻` | `#2d3a5c` | `#7bb8fa` |
| 3 | 개구리 | `🐸` | `#3a5c2d` | `#7bfa90` |
| 4 | 토끼 | `🐰` | `#5c2d3a` | `#fa7b9e` |
| 5 | 고양이 | `🐱` | `#5c4a2d` | `#fac07b` |
| 6 | 강아지 | `🐶` | `#3a4a2d` | `#c4fa7b` |
| 7 | 판다 | `🐼` | `#333333` | `#e0e0e0` |
| 8 | 호랑이 | `🐯` | `#5c3a1f` | `#ffa42b` |
| 9 | 돼지 | `🐷` | `#5c2d4a` | `#fa9ec4` |
| 10 | 원숭이 | `🐵` | `#4a3a2d` | `#d4a574` |
| 11 | 사자 | `🦁` | `#5c4a2d` | `#ffd56b` |
| 12 | 펭귄 | `🐧` | `#2d3a4a` | `#7bdffa` |

### 2.4 닉네임 형식

```
익명의 {동물이름}
```

예시: `익명의 여우`, `익명의 판다`, `익명의 펭귄`

### 2.5 `user_id` 생성 (`genUserId`)

1. **우선**: `crypto.randomUUID()` -- 최신 브라우저에서 UUID v4 생성
2. **fallback**: `u-{Math.random().toString(36).slice(2, 10)}-{Date.now().toString(36)}` -- `crypto.randomUUID` 미지원 환경 대응

### 2.6 localStorage 영속화

- **키**: `"hwarr:chat:identity"`
- **저장 시점**: 최초 `getChatIdentity()` 호출 시 1회
- **읽기 흐름**:
  1. 메모리 캐시(`cached`) 확인 -> 있으면 즉시 반환
  2. `localStorage.getItem(STORAGE_KEY)` -> JSON parse -> `userId`와 `avatar` 필드 존재 여부 검증
  3. 유효하면 캐시에 저장 후 반환
  4. 실패 시(parse error 포함) 새 identity 생성
- **쓰기 흐름**: `localStorage.setItem(STORAGE_KEY, JSON.stringify(fresh))`
- **Private mode / localStorage 사용 불가 환경**: `try-catch`로 감싸 에러 무시, 메모리 캐시(`cached`)에만 유지. 세션 동안은 동일 identity 유지되지만, 탭 새로고침 시 새로운 identity가 생성된다.

---

## 3. 채팅 패널 UI

> 소스: `client/src/components/ChatPanel.tsx`

### 3.1 컴포넌트 인터페이스

```typescript
interface ChatPanelProps {
  visible: boolean   // 패널 표시 여부
  onClose: () => void // 닫기 콜백
}
```

`visible`이 `false`이면 `null`을 반환하여 렌더링하지 않는다.

### 3.2 패널 컨테이너

- **위치**: `absolute bottom-0 left-0 right-0` -- 화면 하단 전체 너비
- **z-index**: `1100`
- **레이아웃**: `flex flex-col`
- **높이**: 사용자가 드래그로 조절 가능, `style={{ height: \`${panelHeight}vh\` }}`
  - **기본값**: `55vh`
  - **최소**: `30vh` (`MIN_HEIGHT`)
  - **최대**: `85vh` (`MAX_HEIGHT`)

### 3.3 바텀시트 래퍼

- **배경**: `bg-[var(--color-bg-surface)]` (CSS custom property)
- **모서리**: `rounded-t-[24px]` (상단 좌우 24px 라운드)
- **그림자**: `shadow-[0_-4px_24px_rgba(0,0,0,0.5)]` (위쪽 방향 확산 그림자)
- **오버플로**: `overflow-hidden`
- **레이아웃**: `flex flex-col flex-1`

### 3.4 드래그 핸들 + 닫기 버튼

- **드래그 핸들**: `w-9 h-1 bg-[var(--color-border)] rounded-full` (너비 36px, 높이 4px)
  - `cursor-row-resize touch-none select-none`
  - mouse/touch 이벤트로 패널 높이 조절 (`handleDragStart/Move/End`)
- **닫기 버튼**:
  - 위치: 헤더 우측
  - 크기: `w-8 h-8` (32x32px)
  - 아이콘: SVG X 마크 (`stroke=var(--color-text-secondary)`)
  - 배경: hover 시 `bg-[var(--color-bg-elevated)]`
  - 동작: 패널 높이를 `MIN_HEIGHT`로 리셋 후 `onClose` 호출

### 3.5 헤더 영역

- **패딩**: `px-5 pb-3` (좌우 20px, 하단 12px)
- **하단 경계선**: `border-b border-[rgba(255,255,255,0.06)]`
- **레이아웃**: `flex items-center justify-between`

#### 3.5.1 왼쪽: 제목 + LIVE 뱃지

- **제목**:
  - 텍스트: `"실시간 화재 공유방"`
  - 폰트: `text-[0.875rem] font-bold` (14px, 굵게)
  - 색상: `text-[var(--color-text-base)]`
- **LIVE 뱃지** (제목 옆):
  - connected 상태: `LIVE {presenceCount}` (예: `LIVE 25`)
  - connecting 상태: `연결 중…`
  - 폰트: `text-[0.625rem] font-bold uppercase tracking-wide` (10px)
  - 색상: `text-[var(--color-warning)]` (주황 계열)
  - 배경: `bg-[rgba(255,140,0,0.12)]` (주황 12% 투명도)
  - 패딩: `px-2 py-0.5`
  - 모서리: `rounded-full` (pill 형태)

#### 3.5.2 오른쪽: 닫기 버튼

- SVG X 아이콘 (`w-8 h-8`, `rounded-full`)
- 동작: 패널 높이 리셋 + `onClose` 호출

### 3.6 TTL 안내 문구

- **텍스트**: `"메시지는 1시간 후 자동 삭제됩니다"` (앞에 `⏳` 이모지)
- **스타일**: `text-center text-[0.625rem] text-[var(--color-text-secondary)] py-1.5` (10px, 가운데 정렬, 상하 6px 패딩)

### 3.7 메시지 영역

- **ref**: `messagesRef` (자동 스크롤용)
- **스타일**: `flex-1 overflow-y-auto px-4 py-3 flex flex-col gap-1.5`
  - 상하 12px, 좌우 16px 패딩
  - 메시지 간 간격: `gap-1.5` (6px)
  - `overflow-y-auto`: 메시지가 넘치면 수직 스크롤
- **자동 스크롤**: `messages.length` 변경 시 `el.scrollTop = el.scrollHeight`로 최하단 이동

#### 3.7.1 시스템 메시지 (입장 안내)

- 메시지 영역 최상단에 고정 표시
- **구조**: 좌우 구분선(`h-px bg-[rgba(255,255,255,0.06)]`) 사이에 텍스트
- **텍스트**: `"{nickname}(으)로 입장했습니다"` (예: `"익명의 여우(으)로 입장했습니다"`)
- **스타일**: `text-[0.6875rem] text-[var(--color-text-secondary)]` (11px)

#### 3.7.2 메시지 버블 공통

- **최대 너비**: `max-w-[85%]`
- **레이아웃**: `flex gap-2 items-end`

#### 3.7.3 내 메시지 (`isMe = true`)

- **정렬**: `self-end flex-row-reverse` (우측 정렬, 아바타가 오른쪽)
- **버블 배경**: `bg-[var(--color-accent)]` (강조색, 초록 계열)
- **버블 텍스트 색상**: `text-[#000]` (검정), `font-medium`
- **border-radius**: `rounded-[18px] rounded-br-[4px]` (우측 하단만 뾰족)
- **닉네임 정렬**: `text-right`
- **닉네임 접미사**: `" (나)"` 표시
- **타임스탬프 정렬**: `text-right`

#### 3.7.4 상대방 메시지 (`isMe = false`)

- **정렬**: `self-start` (좌측 정렬, 아바타가 왼쪽)
- **버블 배경**: `bg-[var(--color-bg-card)]`
- **버블 텍스트 색상**: `text-[var(--color-text-base)]`
- **border-radius**: `rounded-[18px] rounded-bl-[4px]` (좌측 하단만 뾰족)
- **닉네임 정렬**: 기본 (좌측)
- **닉네임 접미사**: 없음
- **타임스탬프 정렬**: 기본 (좌측)

#### 3.7.5 아바타

- **크기**: `w-7 h-7` (28x28px)
- **모양**: `rounded-full` (원형)
- **이모지 크기**: `text-[0.75rem]` (12px)
- **배경색**: `msg.avatar_bg` (메시지 발신자의 동물 preset 색상, inline style)
- **텍스트 색상**: `msg.name_color` (inline style)
- **기타**: `flex-shrink-0`, `font-bold`

#### 3.7.6 닉네임

- **폰트**: `text-[0.6875rem] font-semibold` (11px, 세미볼드)
- **색상**: `msg.name_color` (inline style, 동물별 고유 색상)
- **하단 마진**: `mb-0.5` (2px)
- **내 메시지**: `" (나)"` 접미사 추가, `text-right`

#### 3.7.7 메시지 버블 텍스트

- **폰트**: `text-[0.875rem]` (14px)
- **line-height**: `leading-[1.45]`
- **패딩**: `px-3.5 py-2.5` (좌우 14px, 상하 10px)
- **줄바꿈**: `break-words`

#### 3.7.8 타임스탬프

- **형식**: `HH:MM` (24시간제, 0-padded)
  - 변환 함수 `formatTime(ts)`: Unix timestamp(초 단위)를 `new Date(ts * 1000)`으로 변환 후 `padStart(2, '0')` 적용
- **폰트**: `text-[0.625rem]` (10px)
- **색상**: `text-[var(--color-text-secondary)]`
- **상단 마진**: `mt-0.5` (2px)

### 3.8 입력 영역

- **컨테이너**: `flex gap-2 items-end px-4 pt-2.5 pb-7 border-t border-[rgba(255,255,255,0.06)]`
  - 좌우 16px, 상단 10px, 하단 28px (safe area 고려)
  - 상단 경계선 (6% 불투명 흰색)

#### 3.8.1 텍스트 입력 필드

- **레이아웃**: `flex-1` (전송 버튼 제외한 전체 너비)
- **패딩**: `px-4 py-2.5` (좌우 16px, 상하 10px)
- **배경**: `bg-[var(--color-bg-elevated)]`
- **테두리**: `border border-transparent`, focus 시 `border-[var(--color-accent)]`
- **모서리**: `rounded-[22px]` (pill 형태)
- **폰트**: `text-[0.875rem]` (14px), `text-[var(--color-text-base)]`
- **placeholder**:
  - connected 상태: `"부적절한 언행은 제재될 수 있습니다"`
  - connecting 상태: `"연결 중입니다..."`
  - 색상: `placeholder:text-[#555]`
- **최대 글자수**: `maxLength={300}` (HTML 속성) + `onChange`에서 `e.target.value.slice(0, 300)` 이중 제한
- **비활성 조건**: `disabled={!connected}` -- 소켓 미연결 시 비활성, `disabled:opacity-50`

#### 3.8.2 전송 버튼

- **크기**: `w-10 h-10` (40x40px)
- **모양**: `rounded-full` (원형)
- **배경**: `bg-[var(--color-accent)]` (강조색)
- **아이콘**: SVG 화살표 (`width="18" height="18"`, fill `#000`)
  - path: `M2 21l21-9L2 3v7l15 2-15 2v7z` (종이비행기 형태)
- **active 효과**: `active:scale-90` (누르면 90% 축소)
- **비활성 스타일**: `disabled:opacity-40 disabled:active:scale-100` (40% 투명도, scale 효과 제거)
- **비활성 조건** (`canSend`): `connected && inputValue.trim().length > 0 && inputValue.length <= 300`
- **기타**: `flex-shrink-0`, `transition-transform`

#### 3.8.3 키보드 전송

- **Enter 키**: `e.key === 'Enter' && !e.shiftKey && !isComposing` 조건 충족 시 `handleSend()` 호출
- **IME(한국어) composition 처리**:
  - `onCompositionStart`: `setIsComposing(true)` -- 한글 조합 시작
  - `onCompositionEnd`: `setIsComposing(false)` -- 한글 조합 완료
  - 조합 중에는 Enter 키가 전송을 트리거하지 않는다 (이중 전송 방지)

---

## 4. 채팅 상태 관리

> 소스: `client/src/features/chat/useChatStore.ts`

Zustand 기반 전역 상태 저장소. 소켓 이벤트 -> 스토어 -> ChatPanel 렌더 파이프라인의 중간 계층.

### 4.1 상수

| 상수 | 값 | 설명 |
|------|----|------|
| `MESSAGE_CAP` | `200` | 클라이언트 측 메시지 최대 보유 수 |

### 4.2 `ChatMessage` interface

| 필드 | 타입 | 설명 |
|------|------|------|
| `id` | `string` | 메시지 고유 ID (서버 생성, `msg-{uuid4_hex[:12]}`) |
| `user_id` | `string` | 발신자 식별자 |
| `nickname` | `string` | 발신자 닉네임 |
| `avatar` | `string` | 발신자 이모지 |
| `avatar_bg` | `string` | 아바타 배경색 hex |
| `name_color` | `string` | 닉네임 색상 hex |
| `text` | `string` | 메시지 본문 |
| `timestamp` | `number` | Unix timestamp (초 단위, `time.time()` 서버 기준) |

### 4.3 `ConnectionStatus` type

```typescript
type ConnectionStatus = 'disconnected' | 'connecting' | 'connected'
```

### 4.4 상태 필드

| 필드 | 타입 | 초기값 | 설명 |
|------|------|--------|------|
| `messages` | `ChatMessage[]` | `[]` | 채팅 메시지 목록 |
| `presenceCount` | `number` | `0` | 현재 채팅방 접속자 수 |
| `connectionStatus` | `ConnectionStatus` | `'disconnected'` | 소켓 연결 상태 |

### 4.5 액션

#### `appendMessage(msg: ChatMessage)`

새 메시지를 목록 끝에 추가한다.

1. **중복 방지**: `state.messages.some((m) => m.id === msg.id)` -- 동일 `id`의 메시지가 이미 존재하면 상태를 변경하지 않고 반환한다. 자기 메시지 에코 + 서버 broadcast 중복에 대응.
2. **cap 적용**: 추가 후 `next.length > MESSAGE_CAP`이면 `next.splice(0, next.length - MESSAGE_CAP)`으로 앞쪽(오래된) 메시지를 잘라낸다.

#### `setHistory(msgs: ChatMessage[])`

서버에서 받은 히스토리 메시지 배열을 한번에 설정한다. `msgs.slice(-MESSAGE_CAP)`으로 최대 `MESSAGE_CAP`개만 유지.

#### `setPresence(count: number)`

접속자 수를 업데이트한다. `{ presenceCount: count }`

#### `setStatus(status: ConnectionStatus)`

연결 상태를 업데이트한다. `{ connectionStatus: status }`

#### `reset()`

스토어를 초기 상태로 리셋한다. `{ messages: [], presenceCount: 0 }` -- `connectionStatus`는 리셋하지 않는다.

---

## 5. 소켓 연결 흐름

> 소스: `client/src/features/chat/useChat.ts`

`useChat(active: boolean)` 훅이 채팅 전용 lifecycle을 관리한다. 소켓 싱글톤(`lib/socket.ts`)을 공유하되 채팅 전용 이벤트 리스너만 이 훅에서 등록/해제한다.

### 5.1 패널 열기 (`active = true`)

```
1. 리스너 등록:
   - socket.on('connect', onConnect)
   - socket.on('disconnect', onDisconnect)
   - socket.on('connect_error', onConnectError)
   - socket.on('chat:message', onMessage)
   - socket.on('chat:presence', onPresence)

2. 분기:
   - socket.connected === true:
     -> setStatus('connected')
     -> emitJoin()
   - socket.connected === false:
     -> setStatus('connecting')
     -> socket.connect()

3. emitJoin():
   -> socket.emit('chat:join', {}, callback)
   -> ACK 수신:
      - ack.error 존재 시 -> console.warn, 중단
      - ack.history (배열) -> setHistory(ack.history)
      - ack.presence (숫자) -> setPresence(ack.presence)
```

### 5.2 패널 닫기 (`active = false`, cleanup 함수 실행)

```
1. socket.connected === true -> socket.emit('chat:leave', {})
2. 모든 리스너 해제:
   - socket.off('connect', onConnect)
   - socket.off('disconnect', onDisconnect)
   - socket.off('connect_error', onConnectError)
   - socket.off('chat:message', onMessage)
   - socket.off('chat:presence', onPresence)
3. reset() -> 스토어 초기화 (messages: [], presenceCount: 0)
```

### 5.3 재접속 시 자동 rejoin

- `onConnect` 콜백이 `setStatus('connected')` + `emitJoin()`을 수행
- 소켓이 일시 끊겼다가 재연결되면 자동으로 `chat:join`을 다시 emit하여 방에 재입장 + 히스토리/접속자 수를 갱신한다

### 5.4 에러 처리

| 이벤트 | 핸들러 | 동작 |
|--------|--------|------|
| `disconnect` | `onDisconnect` | `setStatus('connecting')` |
| `connect_error` | `onConnectError` | `setStatus('connecting')` |
| `chat:join` ACK error | callback | `console.warn('[chat] join failed:', ack.error)` |

### 5.5 `useEffect` 의존성 배열

```typescript
[active, appendMessage, setHistory, setPresence, setStatus, reset]
```

---

## 6. 메시지 전송 흐름

> 소스: `client/src/features/chat/useChat.ts` (`useSendChatMessage`), `client/src/components/ChatPanel.tsx` (`handleSend`)

### 6.1 전송 함수 (`useSendChatMessage`)

`useCallback`으로 메모이제이션된 함수를 반환. 의존성 배열은 `[]` (불변).

#### 클라이언트 측 검증 (전송 전)

| 조건 | 에러 코드 |
|------|-----------|
| `trimmed.length === 0` | `'text_too_short'` |
| `trimmed.length > 300` | `'text_too_long'` |

#### 전송 페이로드

```typescript
{
  text: string,        // trim된 메시지 본문
  user_id: string,     // identity.userId
  nickname: string,    // identity.nickname
  avatar: string,      // identity.avatar
  avatar_bg: string,   // identity.avatarBg
  name_color: string,  // identity.nameColor
}
```

#### ACK 응답 (`ChatSendAck`)

```typescript
interface ChatSendAck {
  status?: string   // 'ok'
  id?: string       // 서버가 생성한 메시지 ID
  error?: string    // 에러 코드
}
```

### 6.2 UI 전송 흐름 (`handleSend`)

```
1. canSend 검증: connected && inputValue.trim().length > 0 && inputValue.length <= 300
2. 현재 입력값을 임시 변수(text)에 저장
3. setInputValue('') -> 입력 필드 즉시 비움 (optimistic clear)
4. sendChatMessage(text) 호출 -> Promise<ChatSendAck>
5. ACK 결과:
   - 성공 (ack.error 없음): 완료
   - 실패 (ack.error 존재):
     -> console.warn('[chat] send failed:', ack.error)
     -> setInputValue(text) -> 원래 입력값 복원
```

### 6.3 글자 수 검증 요약

| 계층 | 최소 | 최대 | 비고 |
|------|------|------|------|
| HTML `maxLength` | - | 300 | 브라우저 네이티브 제한 |
| `onChange` slice | - | 300 | `e.target.value.slice(0, 300)` |
| `useSendChatMessage` | 1 (trim 후) | 300 (trim 후) | 빈 문자열/초과 시 즉시 반환 |
| 서버 (`chat_send.go`) | 1 (`CHAT_TEXT_MIN`) | 300 (`CHAT_TEXT_MAX`) | 최종 서버 측 검증 |

---

## 7. 서버 채팅 로직

> 소스: `server/internal/sio/chat_join.go`, `server/internal/sio/chat_send.go`, `server/internal/sio/chat_leave.go`

### 7.1 상수

| 상수 | 값 | 설명 |
|------|----|------|
| `CHAT_ROOM` | `"chat:global"` | Socket.IO room 이름 |
| `CHAT_REDIS_KEY` | `"chat:global:messages"` | Redis list key |
| `CHAT_TTL_SEC` | `3600` | Redis key TTL (1시간) |
| `CHAT_MAX_MESSAGES` | `100` | Redis에 보관하는 최대 메시지 수 |
| `CHAT_HISTORY_SIZE` | `50` | `chat:join` 시 클라이언트에 전달하는 히스토리 개수 |
| `CHAT_TEXT_MIN` | `1` | 메시지 최소 길이 |
| `CHAT_TEXT_MAX` | `300` | 메시지 최대 길이 |

### 7.2 `chat:join` 이벤트

1. `manager.get(sid)`로 연결 정보 확인. 없으면 `{"error": "unknown_sid"}` 반환.
2. `sio.enter_room(sid, CHAT_ROOM)` -- Socket.IO room에 입장
3. `info.rooms.add(CHAT_ROOM)` -- ConnectionManager 상태에 room 추가
4. **Redis 히스토리 로드**:
   - `redis_client.lrange(CHAT_REDIS_KEY, 0, CHAT_HISTORY_SIZE - 1)` -- 최신 50개 조회
   - Redis list는 `LPUSH`로 저장되어 newest-first 순서 -> `reversed(raw)`로 시간순(chronological) 정렬
   - 각 항목을 `json.loads`로 파싱 (실패 시 skip)
5. **접속자 수 계산**: `_room_count()` -- `sio.manager.get_participants("/", CHAT_ROOM)` 순회 카운트
6. **presence broadcast**: `sio.emit("chat:presence", {"count": count}, room=CHAT_ROOM)` -- 방 전체에 접속자 수 전파
7. **ACK 반환**: `{"status": "ok", "history": [...], "presence": count}`

### 7.3 `chat:send` 이벤트

1. **유효성 검증**:
   - `data`가 `dict`인지 확인 -> 아닐 시 `{"error": "invalid_payload"}`
   - `text` strip 후 길이 검증: `< CHAT_TEXT_MIN` -> `{"error": "text_too_short"}`, `> CHAT_TEXT_MAX` -> `{"error": "text_too_long"}`
   - `user_id` 존재 및 문자열 여부 -> 실패 시 `{"error": "user_id_required"}`
2. **메시지 객체 생성**:
   ```json
   {
     "id": "msg-a1b2c3d4e5f6",
     "user_id": "user-uuid",
     "nickname": "익명",
     "avatar": "👤",
     "avatar_bg": "#333",
     "name_color": "#fff",
     "text": "메시지 내용",
     "timestamp": 1700000000.0
   }
   ```
3. **Redis 저장** (pipeline): `LPUSH` + `LTRIM`(최대 100개) + `EXPIRE`(1시간)
   - JSON으로 직렬화하여 저장
   - Redis 저장 실패 시 로깅하되, 메시지 broadcast는 계속 진행
4. **broadcast**: `chat:message` 이벤트로 방 전체에 메시지 전파 (발신자 포함)
5. **ACK 반환**: `{"status": "ok", "id": msg.id}`

### 7.4 `chat:leave` 이벤트

1. `manager.get(sid)`로 연결 정보 조회
2. `sio.leave_room(sid, CHAT_ROOM)` -- room 퇴장 (예외 발생 시 무시)
3. `info.rooms.discard(CHAT_ROOM)` -- ConnectionManager 상태에서 room 제거
4. `_broadcast_presence()` -- 방 전체에 갱신된 접속자 수 전파
5. **ACK 반환**: `{"status": "ok"}`

### 7.5 Presence 관리

- **`_room_count()`**: `sio.manager.get_participants("/", CHAT_ROOM)` sync iterator를 순회하여 인원 수를 카운트. 예외 발생 시 `0` 반환.
- **`_broadcast_presence()`**: `_room_count()` 결과를 `chat:presence` 이벤트로 room 전체에 emit.
- **broadcast 시점**:
  - `chat:join` 성공 시 (입장)
  - `chat:leave` 성공 시 (퇴장)
