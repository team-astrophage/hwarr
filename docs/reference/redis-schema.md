# Redis Schema Reference

화르르 서비스는 외부 RDBMS 없이 **Redis-only 아키텍처**로 동작한다.
모든 실시간 불 상태, 통계, 랭킹, 채팅 메시지가 Redis에 저장되며, TTL과 주기적 cleanup을 통해 데이터 라이프사이클이 관리된다.

---

## 키 패턴 요약

| 키 패턴 | Redis 타입 | 용도 | TTL 정책 |
|---|---|---|---|
| `fire:{gridId}` | Sorted Set | 그리드 셀별 불 이벤트 저장 | 자체 TTL 없음 (score 기반 만료) |
| `active_grids` | Set | 현재 활성 불이 있는 grid ID 인덱스 | 없음 (수동 관리) |
| `stats:total_fires` | String (Integer) | 누적 총 불 횟수 | 없음 (영구 보존) |
| `stats:daily_fires:{YYYY-MM-DD}` | String (Integer) | KST 기준 일별 불 횟수 | 48시간 (172,800초) |
| `stats:daily_ranking:{YYYY-MM-DD}` | Sorted Set | KST 기준 일별 행정동 랭킹 | 48시간 (172,800초) |
| `chat:global:messages` | List | 글로벌 채팅 메시지 저장 | 1시간 (3,600초) |

---

## 키 상세 설명

### `fire:{gridId}`

GPS 좌표를 grid 시스템으로 변환한 셀 단위로 불 이벤트를 관리하는 핵심 키.

| 항목 | 설명 |
|---|---|
| **타입** | Sorted Set |
| **키 예시** | `fire:41740:115434` |
| **Member** | 고유 이벤트 ID (예: `fire-a1b2c3d4e5f6`) |
| **Score** | 만료 시각 (Unix timestamp, `time.time() + FIRE_TTL_SEC`) |
| **TTL** | Redis TTL 미사용. Score 기반으로 논리적 만료 처리 |
| **기본 만료 시간** | 12시간 (`FIRE_TTL_SEC = 43200`) |

**주요 명령어:**

| 명령어 | 용도 | 코드 위치 |
|---|---|---|
| `ZADD key {event_id: expire_at}` | 새 불 이벤트 등록 | `server/internal/engine/fire_progression.go` `RegisterFire()` |
| `ZCOUNT key now +inf` | 현재 활성(미만료) 불 개수 조회 | `server/internal/engine/fire_progression.go` `getActiveCount()` |
| `ZREMRANGEBYSCORE key -inf now` | 만료된 이벤트 일괄 제거 | `server/internal/engine/cleanup.go` |
| `ZCARD key` | 전체 member 수 (cleanup 후 빈 키 확인) | `server/internal/engine/cleanup.go` |
| `DELETE key` | 빈 Sorted Set 삭제 | `server/internal/engine/cleanup.go` |

---

### `active_grids`

현재 활성 불이 존재하는 grid ID를 추적하는 인덱스 Set.

| 항목 | 설명 |
|---|---|
| **타입** | Set |
| **Member** | grid ID 문자열 (예: `41740:115434`) |
| **TTL** | 없음 (수동으로 SREM 처리) |

**주요 명령어:**

| 명령어 | 용도 | 코드 위치 |
|---|---|---|
| `SADD active_grids {gridId}` | 불 등록 시 grid를 활성 목록에 추가 | `server/internal/engine/fire_progression.go` `RegisterFire()` |
| `SMEMBERS active_grids` | 모든 활성 grid 조회 (progression scan) | `server/internal/engine/fire_progression.go` |
| `SREM active_grids {gridId}` | 불이 모두 소멸된 grid 제거 | `server/internal/engine/fire_progression.go` |

---

### `stats:total_fires`

서비스 시작 이후 누적된 총 불 횟수. Redis AOF 활성화 시 서버 재시작에도 보존된다.

| 항목 | 설명 |
|---|---|
| **타입** | String (Integer) |
| **TTL** | 없음 (영구 보존) |

**주요 명령어:**

| 명령어 | 용도 | 코드 위치 |
|---|---|---|
| `INCR stats:total_fires` | 불 등록마다 1 증가 | `server/internal/engine/fire_progression.go` `RegisterFire()` |
| `GET stats:total_fires` | 누적 횟수 조회 | `server/internal/handler/stats.go` |

---

### `stats:daily_fires:{YYYY-MM-DD}`

KST(Asia/Seoul) 기준 일별 불 횟수 카운터.

| 항목 | 설명 |
|---|---|
| **타입** | String (Integer) |
| **키 예시** | `stats:daily_fires:2026-04-06` |
| **TTL** | 48시간 (`STATS_DAILY_FIRES_TTL_SEC = 172800`) |

48시간 TTL을 사용하는 이유는 KST 자정 경계에서의 키 전환 시 이전 날짜 키가 즉시 삭제되지 않도록 여유를 두기 위함이다.

**주요 명령어:**

| 명령어 | 용도 | 코드 위치 |
|---|---|---|
| `INCR stats:daily_fires:{date}` | 불 등록마다 1 증가 | `server/internal/engine/fire_progression.go` `RegisterFire()` |
| `EXPIRE stats:daily_fires:{date} 172800` | TTL 갱신 | `server/internal/engine/fire_progression.go` `RegisterFire()` |
| `GET stats:daily_fires:{date}` | 오늘의 불 횟수 조회 | `server/internal/handler/stats.go` |

---

### `stats:daily_ranking:{YYYY-MM-DD}`

KST 기준 일별 행정동(admin region)별 불 횟수 랭킹.

| 항목 | 설명 |
|---|---|
| **타입** | Sorted Set |
| **키 예시** | `stats:daily_ranking:2026-04-06` |
| **Member** | 행정동 이름 (예: `종로구 청운효자동`) |
| **Score** | 해당 행정동의 누적 불 횟수 |
| **TTL** | 48시간 (`STATS_DAILY_RANKING_TTL_SEC = 172800`) |

**주요 명령어:**

| 명령어 | 용도 | 코드 위치 |
|---|---|---|
| `ZINCRBY stats:daily_ranking:{date} 1 {region}` | 해당 행정동 불 횟수 1 증가 | `server/internal/engine/fire_progression.go` `RegisterFire()` |
| `EXPIRE stats:daily_ranking:{date} 172800` | TTL 갱신 | `server/internal/engine/fire_progression.go` `RegisterFire()` |
| `ZREVRANGE stats:daily_ranking:{date} 0 {limit-1} WITHSCORES` | 상위 N개 행정동 조회 | `server/internal/handler/ranking.go` |

---

### `chat:global:messages`

글로벌 익명 채팅의 최근 메시지 목록. 최신 메시지가 리스트 앞(index 0)에 위치한다.

| 항목 | 설명 |
|---|---|
| **타입** | List |
| **최대 길이** | 100개 (`CHAT_MAX_MESSAGES`) |
| **TTL** | 1시간 (`CHAT_TTL_SEC = 3600`) |
| **항목 형식** | JSON 문자열 (`{id, user_id, nickname, avatar, text, timestamp, ...}`) |

**주요 명령어:**

| 명령어 | 용도 | 코드 위치 |
|---|---|---|
| `LPUSH chat:global:messages {json}` | 새 메시지를 리스트 앞에 추가 | `server/internal/sio/chat_send.go` |
| `LTRIM chat:global:messages 0 99` | 최대 100개로 리스트 길이 제한 | `server/internal/sio/chat_send.go` |
| `EXPIRE chat:global:messages 3600` | TTL 갱신 (메시지 전송마다) | `server/internal/sio/chat_send.go` |
| `LRANGE chat:global:messages 0 49` | 최근 50개 히스토리 조회 | `server/internal/sio/chat_join.go` |

LPUSH + LTRIM + EXPIRE는 pipeline으로 묶어 atomic하게 실행된다.

---

## 설계 패턴

### Sorted Set score에 만료 시각을 넣는 패턴

`fire:{gridId}` Sorted Set에서 각 member의 score는 **해당 이벤트가 만료되는 Unix timestamp**이다.
이 패턴은 다음과 같은 이점을 제공한다:

1. **O(log N) 활성 카운트 조회**: `ZCOUNT key now +inf` 한 번으로 현재 시점에서 아직 유효한 이벤트 수를 즉시 계산한다. 별도의 TTL 만료 이벤트나 콜백 없이 현재 상태를 정확히 파악할 수 있다.

2. **일괄 cleanup 효율성**: `ZREMRANGEBYSCORE key -inf now`로 만료된 이벤트를 한 번에 제거한다. 개별 키에 TTL을 설정하는 방식 대비 관리 오버헤드가 적다.

3. **Redis TTL과의 독립성**: Redis의 key-level TTL은 키 전체를 삭제하지만, score 기반 만료는 **member 단위**로 논리적 만료를 처리한다. 하나의 grid에 시간차를 두고 추가된 여러 이벤트가 각각 독립적으로 만료된다.

4. **시간 기반 쿼리 유연성**: 특정 시간 범위의 이벤트만 조회하는 등, TTL만으로는 불가능한 시간 기반 쿼리가 가능하다.

---

### `active_grids` Set이 SCAN 대신 사용되는 인덱스 역할

Redis의 `SCAN` 명령은 `fire:*` 패턴으로 모든 불 관련 키를 찾을 수 있지만, 다음과 같은 문제가 있다:

- **O(N) 시간 복잡도**: 전체 keyspace를 순회하므로 키가 많을수록 느려진다.
- **비결정적 반복**: cursor 기반이라 한 번의 호출로 모든 결과를 보장하지 않는다.
- **불필요한 순회**: `fire:` 이외의 키(`stats:*`, `chat:*` 등)도 함께 순회한다.

`active_grids` Set은 이를 해결하는 **보조 인덱스(secondary index)** 역할을 한다:

- `SMEMBERS active_grids`는 **O(N)이지만 N은 활성 grid 수**에 한정된다 (전체 keyspace가 아님).
- 불 등록 시 `SADD`, 소멸 시 `SREM`으로 인덱스가 동기적으로 유지된다.
- 2초 간격의 progression scan과 60초 간격의 cleanup loop가 이 Set을 기반으로 동작하므로, 불필요한 keyspace 탐색을 완전히 제거한다.

---

### Redis-only 아키텍처에서의 데이터 라이프사이클

화르르는 외부 데이터베이스 없이 Redis만으로 모든 상태를 관리한다. 데이터의 생성부터 소멸까지 라이프사이클은 다음과 같다:

```
[사용자 클릭] ─── POST /api/fire 또는 fire:ignite (Socket.IO)
       │
       ▼
  ┌──────────────────────────────────────────────┐
  │ register_fire()                              │
  │  1. ZADD fire:{gridId} {expire_at} {eventId}│
  │  2. SADD active_grids {gridId}               │
  │  3. INCR stats:total_fires                   │
  │  4. INCR stats:daily_fires:{date}            │
  │  5. ZINCRBY stats:daily_ranking:{date}       │
  └──────────────────────────────────────────────┘
       │
       ▼
  ┌──────────────────────────────────────────────┐
  │ Progression Loop (2초 간격)                    │
  │  - SMEMBERS active_grids                     │
  │  - ZCOUNT fire:{gridId} now +inf             │
  │  - 단계 변화 감지 시 Socket.IO broadcast       │
  │  - 불 소멸 grid: SREM active_grids            │
  └──────────────────────────────────────────────┘
       │
       ▼
  ┌──────────────────────────────────────────────┐
  │ Cleanup Loop (60초 간격)                       │
  │  - ZREMRANGEBYSCORE fire:{gridId} -inf now   │
  │  - ZCARD → 0이면 DELETE key + SREM           │
  └──────────────────────────────────────────────┘
       │
       ▼
  ┌──────────────────────────────────────────────┐
  │ 자동 만료되는 키                                │
  │  - stats:daily_fires:{date}  → 48시간 후 삭제  │
  │  - stats:daily_ranking:{date} → 48시간 후 삭제 │
  │  - chat:global:messages       → 1시간 후 삭제  │
  └──────────────────────────────────────────────┘
```

**영구 보존 키:**
- `stats:total_fires` — TTL 없음. Redis AOF/RDB persistence로 서버 재시작 후에도 유지.

**논리적 만료 키 (score 기반):**
- `fire:{gridId}` — Redis TTL 미사용. Cleanup loop이 만료된 member를 주기적으로 제거하고, 빈 키는 삭제.

**자동 만료 키 (Redis TTL):**
- `stats:daily_fires:{date}` — 48시간 TTL. 매 INCR 시 EXPIRE로 갱신.
- `stats:daily_ranking:{date}` — 48시간 TTL. 매 ZINCRBY 시 EXPIRE로 갱신.
- `chat:global:messages` — 1시간 TTL. 매 메시지 전송 시 EXPIRE로 갱신.

이 구조 덕분에 별도의 마이그레이션이나 스키마 관리 없이, Redis 인스턴스 하나로 전체 서비스의 실시간 상태를 관리할 수 있다.
