# 화르르 불 시스템 디버깅 가이드

화르르의 불 시스템은 Redis Sorted Set을 기반으로 격자(grid)별 불 이벤트를 관리한다.
이 문서는 Redis CLI, 서버 로그, Socket.IO를 활용해 문제를 진단하는 방법을 정리한다.

---

## 핵심 구조 요약

| 개념 | 설명 |
|------|------|
| Grid ID | GPS 좌표를 100m x 100m 격자로 변환한 `"{grid_lat}:{grid_lng}"` 형식 |
| 불 이벤트 | Sorted Set member. score는 만료 시각(`now + FIRE_TTL_SEC`, 기본 43200초 = 12시간) |
| 단계(stage) | 활성 불 개수에 따라 0~5단계: 없음(0) / 불씨(1-5) / 모닥불(6-23) / 화재(24-71) / 대화재(72-169) / 전소(170+) |
| 확산(spread) | 격자당 불 300개 이상이면 인접 8방향 중 랜덤 격자로 cascade (최대 8hop) |

---

## 1. Redis CLI 명령어 모음

### 1.1 활성 격자 목록 확인

```bash
# 현재 불이 있는 모든 격자 ID 조회
redis-cli SMEMBERS active_grids
```

활성 격자가 몇 개인지만 확인하려면:

```bash
redis-cli SCARD active_grids
```

### 1.2 특정 격자의 불 개수 확인

활성 불 개수는 score가 현재 시각 이후인 member 수로 결정된다.

```bash
# 현재 시각의 Unix timestamp 구하기
NOW=$(date +%s)

# 특정 격자(예: 41111:115454)의 활성 불 개수
redis-cli ZCOUNT fire:41111:115454 $NOW +inf

# 만료 포함 전체 이벤트 수 (expired 포함)
redis-cli ZCARD fire:41111:115454

# 격자에 등록된 불 이벤트 목록 (score=만료시각과 함께)
redis-cli ZRANGE fire:41111:115454 0 -1 WITHSCORES
```

### 1.3 통계 확인

```bash
# 누적 총 불 개수
redis-cli GET stats:total_fires

# 오늘 날짜의 일별 불 개수 (KST 기준)
redis-cli GET stats:daily_fires:2026-04-06

# 오늘의 지역별 순위 (상위 10개)
redis-cli ZREVRANGE stats:daily_ranking:2026-04-06 0 9 WITHSCORES
```

### 1.4 채팅 확인

```bash
# 최근 채팅 메시지 10개 (newest-first 저장, JSON 형식)
redis-cli LRANGE chat:global:messages 0 9

# 채팅 메시지 총 개수 (최대 100개 cap)
redis-cli LLEN chat:global:messages

# 채팅 key의 남은 TTL 확인 (초)
redis-cli TTL chat:global:messages
```

### 1.5 키 패턴 전체 조회

```bash
# 불 관련 모든 키 조회 (주의: 프로덕션에서는 SCAN 사용 권장)
redis-cli KEYS "fire:*"

# 통계 관련 키 조회
redis-cli KEYS "stats:*"
```

> **주의**: `KEYS` 명령은 프로덕션 환경에서 성능에 영향을 줄 수 있다. 대신 `SCAN` 사용을 권장한다.

---

## 2. 서버 로그 패턴으로 문제 추적

서버는 Go `log` 패키지를 사용한다. 주요 로그 패턴은 다음과 같다.

### 2.1 단계 변경 (stage change)

```
Stage change: grid=41111:115454 stage=BULSSSI→MODAKBUL count=10
```

- `grid=` : 격자 ID
- `stage=` : 이전 단계 → 새 단계 (enum 이름 사용)
- `count=` : 현재 활성 불 개수

필터 예시:

```bash
# 단계 변경 로그만 추출
grep "Stage change:" server.log

# 특정 격자의 단계 변경 추적
grep "Stage change: grid=41111:115454" server.log
```

### 2.2 불 확산 (fire spread)

격자의 불이 300개 이상일 때 인접 격자로 확산되면 아래 로그가 출력된다.

```
fire spread: 41111:115454 → 41112:115454 (depth=1, event=fire-a1b2c3d4e5f6)
```

- 첫 번째 grid ID : 원래 요청된 격자
- 두 번째 grid ID : 실제 불이 놓인 격자
- `depth=` : cascade 횟수
- `event=` : 불 이벤트 ID

```bash
grep "fire spread:" server.log
```

### 2.3 소방관 NPC 생성 (firefighter spawn)

단계 4(대화재) 이상에서 소방관 NPC가 broadcast된다.

```
Firefighter spawn broadcast: npc_id=ff-xxxx grid=41111:115454 stage=4
```

```bash
grep "Firefighter spawn broadcast:" server.log
```

### 2.4 REST API 불 생성

```
POST /api/fire grid=41111:115454 count=15 stage=2
```

### 2.5 Socket.IO 불 생성

```
fire:ignite sid=abc123 requested=41111:115454 landed=41111:115454 count=15 stage=2 spread=0
```

- `requested=` : 클라이언트가 요청한 격자
- `landed=` : 실제 불이 놓인 격자 (spread 발생 시 다를 수 있음)
- `spread=` : 확산 경로 수 (0이면 확산 없음)

### 2.6 정리(cleanup) 로그

```
Cleanup: removed 42 expired fire events
```

60초 간격으로 만료된 이벤트를 정리한다. 이 로그가 안 보이면 cleanup loop가 멈춘 것이다.

### 2.7 엔진 시작/중지

```
FireProgressionEngine started (scan=2.0s, cleanup=60.0s)
FireProgressionEngine stopped
```

---

## 3. 일반적인 문제 시나리오와 진단법

### 3.1 불이 안 보일 때

**증상**: 클라이언트에서 불을 찍었는데 지도에 아무것도 안 나타남.

**진단 순서**:

1. **Redis에 데이터가 있는지 확인**
   ```bash
   redis-cli SMEMBERS active_grids
   # 비어 있으면 → Redis 자체에 불 데이터가 없음
   ```

2. **특정 격자에 활성 불이 있는지 확인**
   ```bash
   NOW=$(date +%s)
   redis-cli ZCOUNT fire:<grid_id> $NOW +inf
   # 0이면 → 이미 만료됐거나 등록 자체가 안 된 것
   ```

3. **서버 로그에서 ignite 이벤트 확인**
   ```bash
   grep "fire:ignite" server.log | tail -20
   # 로그가 없으면 → 클라이언트 요청이 서버에 도달하지 않음
   ```

4. **Socket.IO 연결 상태 확인** (아래 섹션 4 참고)

**흔한 원인**:
- Redis 연결 끊김 → 서버 로그에 connection error가 있는지 확인
- FIRE_TTL_SEC이 너무 짧게 설정됨 → 환경변수 `FIRE_TTL_SEC` 확인
- 클라이언트의 viewport subscribe가 안 됨 → `subscribe:viewport` 이벤트 로그 확인

### 3.2 단계 전환이 안 될 때

**증상**: 불을 많이 찍었는데 단계가 올라가지 않음.

**진단 순서**:

1. **실제 활성 불 개수 확인**
   ```bash
   NOW=$(date +%s)
   redis-cli ZCOUNT fire:<grid_id> $NOW +inf
   ```
   단계별 threshold: 불씨(1) / 모닥불(6) / 화재(24) / 대화재(72) / 전소(170)

2. **progression loop가 동작 중인지 확인**
   ```bash
   grep "Stage change:" server.log | tail -5
   # 최근 로그가 없으면 progression loop가 멈췄을 수 있음
   ```

3. **엔진 에러 확인**
   ```bash
   grep "Error in progression loop" server.log
   ```

**흔한 원인**:
- 만료된 이벤트가 포함된 총 개수를 보고 있음 → `ZCOUNT`가 아닌 `ZCARD`로 확인하면 만료 포함 수치가 나옴. 반드시 `ZCOUNT key $NOW +inf`로 활성 개수 확인
- progression loop 에러 → 로그의 exception traceback 확인
- `active_grids` Set에서 해당 격자가 누락됨 → `SISMEMBER active_grids <grid_id>` 확인

### 3.3 확산이 안 될 때

**증상**: 불이 300개 넘었는데 인접 격자로 퍼지지 않음.

**진단 순서**:

1. **해당 격자의 활성 불 개수 확인**
   ```bash
   NOW=$(date +%s)
   redis-cli ZCOUNT fire:<grid_id> $NOW +inf
   # 300 미만이면 확산 threshold에 도달하지 않은 것
   ```

2. **확산 로그 확인**
   ```bash
   grep "fire spread:" server.log | grep "<grid_id>"
   ```

3. **확산은 새 불 등록 시에만 발생**한다.
   기존 불이 300개 있어도, 새로운 `fire:ignite` 요청이 들어와야 `register_fire()`가 호출되면서 확산이 트리거된다.
   이미 있는 불이 자동으로 퍼지는 것이 아님에 주의.

**흔한 원인**:
- 실제 활성 개수가 300 미만 (만료된 것 포함해서 착각)
- 확산 cascade 최대 깊이(8)에 도달 → 인접 격자가 모두 가득 찬 극단적인 경우

### 3.4 통계가 안 맞을 때

**증상**: 표시되는 총 불 개수가 실제와 다름.

**진단 순서**:

1. **누적 카운터 확인**
   ```bash
   redis-cli GET stats:total_fires
   ```
   이 값은 `INCR`로만 증가하며, 불이 만료되어도 감소하지 않는다.
   즉, "지금까지 찍힌 총 불 횟수"이지 "현재 타고 있는 불 수"가 아니다.

2. **일별 카운터 확인**
   ```bash
   redis-cli GET stats:daily_fires:$(date +%Y-%m-%d)
   ```
   KST 기준 날짜이므로 UTC 시간대와 다를 수 있다.

3. **일별 카운터의 TTL 확인**
   ```bash
   redis-cli TTL stats:daily_fires:$(date +%Y-%m-%d)
   ```
   TTL은 48시간(172800초). -2가 반환되면 key가 이미 만료됨.

4. **지역 순위 데이터 확인**
   ```bash
   redis-cli ZREVRANGE stats:daily_ranking:$(date +%Y-%m-%d) 0 -1 WITHSCORES
   ```
   지역 순위는 `AdminRegionResolver`가 정상 로드되어야 기록된다.
   resolver가 없으면 순위 데이터가 쌓이지 않지만 다른 통계에는 영향 없음.

---

## 4. Socket.IO 디버깅

### 4.1 브라우저 DevTools에서 WebSocket 확인

1. Chrome DevTools 열기 (`F12` 또는 `Cmd+Option+I`)
2. **Network** 탭 선택
3. 필터에서 **WS** 클릭 (WebSocket만 표시)
4. 페이지를 새로고침하면 Socket.IO 연결이 잡힘
5. 해당 연결 클릭 후 **Messages** 탭에서 실시간 메시지 확인

### 4.2 주요 이벤트 메시지 형식

**클라이언트 → 서버**:

| 이벤트 | payload | 설명 |
|--------|---------|------|
| `fire:ignite` | `{"lat": 37.5, "lng": 127.0}` | 불 생성 요청 |
| `fire` | `{"lat": 37.5, "lng": 127.0}` | 불 생성 (mock server 호환) |
| `subscribe:viewport` | `{"ne_lat": .., "ne_lng": .., "sw_lat": .., "sw_lng": ..}` | viewport 구독 |
| `fire:state` | `{"grid_ids": ["41111:115454"]}` | 격자 상태 요청 |
| `get_fires` | `{}` | 전체 활성 불 요청 (mock 호환) |
| `chat:join` | `{}` | 채팅방 입장 |
| `chat:send` | `{"text": "...", "user_id": "...", "nickname": "..."}` | 채팅 전송 |

**서버 → 클라이언트**:

| 이벤트 | 설명 |
|--------|------|
| `fire:update` | 격자 상태 변경 (단계 전환 시 broadcast) |
| `fire:ignite` | 새 불 이벤트 (해당 격자 room에 broadcast) |
| `fire:global_update` | 전역 업데이트 (모든 클라이언트) |
| `fire:stage_transition` | 단계 전환 상세 (prev_stage, new_stage 포함) |
| `fire:spread` | 확산 발생 시 경로 정보 |
| `firefighter:spawn` | 소방관 NPC 등장 (단계 4 이상) |
| `fires:sync` | 전체 활성 불 동기화 (mock 호환) |
| `chat:message` | 채팅 메시지 수신 |
| `chat:presence` | 채팅방 접속자 수 변경 |

### 4.3 연결 문제 확인 포인트

- **WS 탭에 연결이 안 보임**: Socket.IO가 polling fallback 중일 수 있다. XHR/fetch 탭에서 `/socket.io/?transport=polling` 요청 확인.
- **연결은 됐는데 메시지가 안 옴**: `subscribe:viewport`를 보냈는지 확인. viewport 구독 없이는 room에 join되지 않아 `fire:update` 등을 수신할 수 없다.
- **ack가 error를 반환**: Messages 탭에서 서버 응답의 `error` 필드 확인. 예: `{"error": "lat and lng are required"}`

### 4.4 콘솔에서 수동 테스트

브라우저 콘솔에서 직접 Socket.IO 이벤트를 보낼 수 있다 (전역 socket 객체가 노출된 경우):

```javascript
// 불 생성 테스트
socket.emit("fire:ignite", { lat: 37.5665, lng: 126.9780 }, (ack) => {
  console.log("ack:", ack);
});

// 현재 활성 불 상태 요청
socket.emit("fire:state", {}, (ack) => {
  console.log("active fires:", ack);
});

// viewport 구독
socket.emit("subscribe:viewport", {
  ne_lat: 37.57, ne_lng: 126.99,
  sw_lat: 37.56, sw_lng: 126.97
}, (ack) => {
  console.log("subscribed:", ack);
});
```

---

## Redis 키 패턴 요약

| 키 패턴 | 타입 | 설명 |
|---------|------|------|
| `fire:{grid_lat}:{grid_lng}` | Sorted Set | 격자별 불 이벤트 (score = 만료 시각) |
| `active_grids` | Set | 현재 불이 있는 격자 ID 목록 |
| `stats:total_fires` | String (integer) | 누적 총 불 횟수 |
| `stats:daily_fires:{YYYY-MM-DD}` | String (integer) | 일별 불 횟수 (TTL 48h) |
| `stats:daily_ranking:{YYYY-MM-DD}` | Sorted Set | 일별 지역 순위 (TTL 48h) |
| `chat:global:messages` | List | 채팅 메시지 (최대 100개, TTL 1h) |
