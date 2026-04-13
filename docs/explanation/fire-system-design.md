# 불 시스템 설계 해설

> 화르르(hwarr)의 핵심인 불 등록-진행-소멸 파이프라인이 **왜** 이런 구조를 갖게 되었는지 설명한다.
> 코드 동작 방식(how)보다 설계 판단(why)에 초점을 맞춘다.

---

## 1. Sorted Set 기반 시간 만료 패턴

### 구조

```
Key:    fire:{gridId}          # grid 하나당 Sorted Set 하나
Score:  만료 시각 (Unix timestamp)  # now + FIRE_TTL_SEC (기본 43200초 = 12시간)
Member: 고유 event ID             # "fire-{uuid12}"
```

### 왜 Sorted Set인가

| 대안 | 문제점 |
|---|---|
| key마다 Redis TTL (`EXPIRE`) | member 개별 만료 불가 -- key 전체가 한꺼번에 사라진다 |
| 별도 만료 cron job | 정확도가 polling 주기에 종속, 추가 인프라 필요 |
| application 메모리 타이머 | 서버 재시작 시 유실, 수평 확장 불가 |

Sorted Set은 score에 만료 시각을 넣으면 다음 세 가지를 한 번에 해결한다.

1. **활성 개수 조회가 원자적이다.** `ZCOUNT fire:{gridId} {now} +inf` 한 번으로 만료되지 않은 member 수를 O(log N)에 얻는다. 별도 lock 없이 동시 쓰기와 읽기가 안전하다.
2. **만료 정리가 range 연산 하나다.** `ZREMRANGEBYSCORE fire:{gridId} -inf {now}`로 지난 member를 일괄 제거한다. 별도 cron이나 Keyspace Notification 없이 cleanup loop 안에서 처리된다.
3. **TTL 단위가 member별이다.** 같은 grid에 1초 간격으로 들어온 불 이벤트도 각자 독립적으로 만료된다. key 수준 TTL로는 이 granularity를 표현할 수 없다.

결과적으로 Redis 하나에 **저장 + 만료 판정 + 정리**가 모두 들어가므로, 별도 스케줄러 서비스나 메시지 큐 없이 단일 프로세스에서 불 생명주기를 완결할 수 있다.

---

## 2. 2초 스캔 루프 vs 이벤트 드리븐

화르르의 불 상태 변경은 **두 경로**로 감지된다.

```
경로 A: fire:ignite 이벤트 → register_fire() → 즉시 stage 비교 → broadcast
경로 B: _progression_loop (2초 주기) → 전체 active grid 스캔 → stage 비교 → broadcast
```

### 왜 둘 다 필요한가

**경로 A(즉시 전환)**는 사용자가 불을 놓는 순간의 **상승(escalation)** 을 잡는다. 클릭 직후 stage가 올라갔다면 2초를 기다릴 이유가 없으므로, `register_fire()` 안에서 곧바로 `_broadcast_stage_change()`를 호출한다.

하지만 경로 A만으로는 **하강(de-escalation)** 을 감지할 수 없다. 불 이벤트는 TTL이 지나면 score 비교에서 자연 탈락하는데, 이 탈락은 어떤 이벤트도 발생시키지 않는다 -- 아무도 "만료됨"을 알려주지 않는다. 그래서 **경로 B(2초 루프)** 가 주기적으로 `ZCOUNT`를 돌려 active count가 줄었는지 확인하고, stage가 내려갔으면 broadcast한다.

```
시간 →  ──────────────────────────────────────────►
         ↑ fire:ignite          ↑ TTL 만료 (silent)
         경로 A가 감지           경로 B가 감지
         (즉시 broadcast)       (최대 2초 뒤 broadcast)
```

2초라는 주기는 **사용자 체감 지연(< 3초)** 과 **Redis 부하(초당 0.5회 ZCOUNT per grid)** 사이의 트레이드오프다. 실시간 게임이 아닌 지도 서비스에서 2초 지연은 충분히 수용 가능하다.

### cleanup loop (60초)와의 역할 분리

| 루프 | 주기 | 역할 |
|---|---|---|
| `_progression_loop` | 2초 | stage 변화 감지 + broadcast |
| `_cleanup_loop` | 60초 | 만료 member 물리 삭제 (`ZREMRANGEBYSCORE`) + 빈 key 정리 |

Stage 감지는 `ZCOUNT`만으로 충분하고 member를 지울 필요가 없으므로 자주 돌아도 가볍다. 물리 삭제는 메모리 회수 목적이므로 느긋하게 60초마다 수행한다.

---

## 3. 확산 메커니즘 설계

### 300 임계값의 게임디자인 의도

한 grid cell의 active fire가 300 이상이면 추가 클릭은 **랜덤 8방향 이웃**으로 넘어간다.

```python
FIRE_SPREAD_THRESHOLD = 300
```

이 숫자는 stage 체계(불씨 1 / 모닥불 6 / 화재 24 / 대화재 72 / 전소 170)의 최고 단계(전소, 170)보다 **의도적으로 높다**. 즉:

- **전소 단계에 도달한 뒤에도 한동안은 같은 셀에 계속 쌓인다.** 사용자에게 "이 셀은 완전히 탔다"는 시각 피드백을 충분히 준 뒤에야 확산이 시작된다.
- **확산 시작 = 플레이어 간 협업의 결과물.** 여러 사용자가 같은 셀을 집중 클릭해야 300에 도달하므로, 확산 자체가 "군중 이벤트"로 기능한다.
- **역설적 긴장감.** 확산이 시작되면 주변 셀에 불이 번지는데, 이는 지도 위에서 시각적으로 극적인 효과를 만든다. 300이라는 높은 임계값 덕분에 이 순간이 드물고, 드물기 때문에 인상적이다.

### 랜덤 이웃 선택

```python
neighbors = get_neighbors_8(current_grid)
next_grid = random.choice(neighbors)
```

확산 방향을 결정론적으로(예: 가장 비어있는 이웃) 정하지 않고 **완전 랜덤**으로 선택한 이유:

1. **예측 불가능성이 재미다.** 어디로 번질지 모르는 상황이 지도를 보는 사용자에게 흥미를 준다.
2. **구현 단순성.** 이웃 8개의 active count를 모두 조회하면 Redis 왕복이 8배 늘어난다. 랜덤 선택은 추가 조회 없이 O(1)이다.
3. **cascade 특성.** 랜덤으로 선택된 이웃도 300 이상이면 다시 다음 이웃으로 넘어가므로, 밀집 지역에서는 자연스럽게 "빈 셀을 찾아가는" 확률적 탐색이 된다.

### 8홉 제한의 안전장치

```python
FIRE_MAX_CASCADE_DEPTH = 8
```

이론적으로 모든 이웃이 300 이상이면 cascade가 무한히 이어질 수 있다. 8홉 제한은 이 **병적 경우(pathological case)** 를 방어한다. 실제로 8홉에 도달하려면 출발점 주변 8^8 규모의 영역이 전부 포화 상태여야 하므로, 정상 운영에서는 도달할 수 없는 값이다. 그럼에도 명시적 상한을 두는 것은 "만약"에 대비하는 방어적 프로그래밍이다.

도달 시에는 현재 위치에 강제 착지(force-land)하여, 요청이 절대 실패하지 않도록 보장한다.

---

## 4. 소방관 NPC -- 시각 효과 전용인 이유

`FirefighterNPC`는 서버에서 생성되지만 **Redis에 상태를 저장하지 않고, 실제 진화(suppression)도 수행하지 않는다.**

```python
fires_removed: int = Field(default=0, description="Always 0 (no suppression)")
```

### 해커톤 스코프에서의 의도적 절단

화르르는 해커톤 프로젝트로 시작되었다. 소방관이 실제로 불을 끄는 로직을 구현하려면:

- NPC 상태를 Redis에 persist해야 하고 (생성/이동/진화/소멸 lifecycle)
- 진화 속도 밸런싱이 필요하며 (너무 빨리 끄면 재미없고, 너무 느리면 의미없다)
- 동시성 문제가 생긴다 (두 NPC가 같은 불을 동시에 끄면?)

이 모든 복잡도를 회피하면서도 "대형 화재에 소방차가 출동한다"는 **시각적 피드백**은 제공하기 위해, broadcast-only 구조를 선택했다. 서버는 `firefighter:spawn` 이벤트만 쏘고, 프론트엔드가 소방차 오버레이의 표시와 제거를 자체적으로 관리한다.

`remove_per_sweep`이나 `target_stage` 같은 필드가 payload에 포함되어 있는 것은, 추후 실제 진화 로직을 구현할 때 프론트엔드 수정 없이 서버 로직만 추가하면 되도록 **확장 여지를 미리 확보**해 둔 것이다.

---

## 5. active_grids Set -- SCAN 회피를 위한 인덱스 패턴

### 문제

2초마다 "불이 활성화된 grid"를 모두 찾아야 한다. Redis에서 이를 수행하는 순진한 방법은:

```
SCAN 0 MATCH fire:* COUNT 100
```

`SCAN`은 전체 keyspace를 순회하므로, 불과 무관한 key(stats, session, cache 등)까지 훑는다. key가 수만 개일 때 latency가 불예측적으로 튀고, 반복 호출이 필요하다.

### 해결: 보조 인덱스 Set

```python
ACTIVE_GRIDS_KEY = "active_grids"   # Redis Set

# 불 등록 시
await self._redis.sadd(ACTIVE_GRIDS_KEY, landing_grid)

# 스캔 시
members = await self._redis.smembers(ACTIVE_GRIDS_KEY)

# 빈 grid 정리 시
await self._redis.srem(ACTIVE_GRIDS_KEY, grid_id)
```

`SMEMBERS`는 Set의 모든 member를 **한 번의 O(N) 호출**로 반환한다 (N = active grid 수). 전체 keyspace 크기와 무관하므로:

- active grid가 50개면 50개만 반환한다.
- `SCAN`의 cursor 관리가 필요 없다.
- 응답 시간이 active grid 수에만 비례하여 예측 가능하다.

대가로, 불 등록과 정리 시 `SADD`/`SREM`을 한 번씩 더 호출해야 하지만, 이는 O(1) 연산이므로 무시할 수 있다.

이 패턴은 Redis에서 **보조 인덱스(secondary index)** 를 직접 관리하는 고전적 기법으로, keyspace 크기가 커질수록 SCAN 대비 이점이 극대화된다.

---

## 6. 불 등록 흐름 전체 다이어그램

`register_fire(grid_id, event_id, expire_at)` 호출 시 내부에서 일어나는 전체 과정이다.

```
Client (fire:ignite / POST /api/fire)
  │
  ▼
to_grid_id(lat, lng) ─────────────────────► grid_id
  │
  ▼
register_fire(grid_id, event_id, expire_at)
  │
  ├─── [1] 확산 판정 루프 (최대 8홉) ──────────────────────────┐
  │     │                                                      │
  │     ▼                                                      │
  │    ZCOUNT fire:{current_grid} {now} +inf                   │
  │     │                                                      │
  │     ├── count < 300 → current_grid에 착지 (break)          │
  │     │                                                      │
  │     └── count >= 300 → random neighbor 선택                │
  │           spread_path에 (current, next) 기록               │
  │           current_grid = next_grid                         │
  │           └── 루프 반복 ───────────────────────────────────┘
  │
  ├─── [2] Redis 기록
  │     ├── ZADD fire:{landing_grid} {expire_at} {event_id}
  │     ├── SADD active_grids {landing_grid}
  │     ├── INCR stats:total_fires
  │     ├── INCR stats:daily_fires:{YYYY-MM-DD}
  │     └── ZINCRBY stats:daily_ranking:{YYYY-MM-DD} 1 {region}
  │         (AdminRegionResolver로 landing grid → 행정동 변환)
  │
  ├─── [3] Stage 전환 감지
  │     │
  │     ▼
  │    ZCOUNT fire:{landing_grid} {now} +inf → active_count
  │    get_stage(active_count) → new_stage
  │     │
  │     ├── new_stage == prev_stage → (변화 없음, skip)
  │     │
  │     └── new_stage != prev_stage
  │           ├── broadcast fire:update (room + global)
  │           ├── broadcast fire:stage_transition
  │           └── new_stage >= 대화재(4)?
  │                 └── YES → broadcast firefighter:spawn
  │
  ├─── [4] 확산 애니메이션 (spread_path가 있을 때만)
  │     └── broadcast fire:spread { path, event_id, timestamp }
  │
  └─── [5] Return FireRegistration
        { grid_id, active_count, spread_path }
              │
              ▼
        Caller(route/sio)가 응답 조립 + fire:ignite broadcast
```

### 핵심 설계 포인트 요약

| 단계 | 설계 판단 | 이유 |
|---|---|---|
| 확산 루프 | cascade 방식, 깊이 제한 8 | 한 번에 빈 셀을 찾되 무한 루프 방지 |
| Redis 기록 | ZADD + SADD 병행 | Sorted Set(만료 관리) + Set(인덱스) 역할 분리 |
| 통계 기록 | landing grid 기준 | 확산된 최종 위치가 실제 화재 지점이므로 |
| Stage 감지 | register 내 즉시 수행 | 사용자에게 클릭 즉시 피드백 제공 |
| Firefighter | stage 전환 시에만 spawn | 같은 stage 내 반복 spawn 방지 (dedup) |
| broadcast | room + global 이중 발행 | viewport 구독자(정밀) + 전체 지도 개요(광역) 모두 커버 |
