# 불 단계 시스템 (Fire Stage System)

화르르의 핵심 게임 메커니즘인 불 단계 시스템을 설명합니다. 각 grid cell은 활성 fire event 수에 따라 단계가 결정되며, 단계별로 시각 효과와 소방관 NPC 스폰 여부가 달라집니다.

## 불 단계 테이블

grid cell 내 활성 fire 수(`active_count`)에 따라 6단계로 분류됩니다. 임계값 간격은 대략 2배씩 증가하는 지수적 진행(exponential progression)을 따릅니다.

| 단계 | 한국어 이름 | 영어 이름 | 임계값 (threshold) | 클릭 범위 | 소방관 스폰 | `remove_per_sweep` |
|:---:|:---:|:---:|:---:|:---:|:---:|:---:|
| 0 | 없음 | none | 0 | 0 | - | 0 |
| 1 | 불씨 | ember | 1 | 1 -- 9 | - | 0 |
| 2 | 모닥불 | campfire | 10 | 10 -- 39 | - | 0 |
| 3 | 화재 | fire | 40 | 40 -- 119 | - | 0 |
| 4 | 대화재 | big fire | 120 | 120 -- 279 | O | 2 |
| 5 | 전소 | total burn | 280 | 280+ | O | 3 |

> 소스: `server/models/fire.py` -- `STAGE_CONFIGS` dict

## 단계 전환 (Stage Transition) 로직

단계는 grid cell 내 **활성 fire event 수**를 기준으로 실시간 계산됩니다.

### 판별 알고리즘

`get_stage(active_count)` 함수는 내림차순 정렬된 threshold 목록을 순회하며, `active_count >= threshold`를 만족하는 **첫 번째(가장 높은)** 단계를 반환합니다.

```python
# server/models/fire.py

_STAGE_THRESHOLDS: list[tuple[int, FireStage]] = sorted(
    [(cfg.threshold, cfg.stage) for cfg in STAGE_CONFIGS.values()],
    key=lambda x: x[0],
    reverse=True,  # 내림차순: 280, 120, 40, 10, 1, 0
)

def get_stage(active_count: int) -> FireStage:
    for threshold, stage in _STAGE_THRESHOLDS:
        if active_count >= threshold:
            return stage
    return FireStage.NONE
```

### 전환 감지 및 broadcast

`FireProgressionEngine`이 2가지 경로로 단계 전환을 감지합니다:

1. **즉시 전환** -- `register_fire()` 호출 시 fire 등록 직후 `prev_stage`와 `new_stage`를 비교하여, 변경이 있으면 즉시 `fire:stage_transition` event를 broadcast합니다.
2. **주기적 scan** -- background loop(`_progression_loop`)가 `scan_interval`(기본 2초) 마다 모든 active grid를 순회하며 단계 변경을 감지합니다. TTL 만료로 active_count가 감소하여 단계가 내려가는 경우는 이 경로에서 포착됩니다.

전환 시 broadcast되는 Socket.IO event:

| Event | 범위 | 설명 |
|---|---|---|
| `fire:update` | grid room + global | grid state 전체 payload |
| `fire:stage_transition` | grid room + global | `prev_stage`, `new_stage` 포함 (애니메이션용) |

## 소방관 (Firefighter) NPC 규칙

소방관 NPC는 **시각 효과 전용(visual effect only)**입니다. 서버는 실제 fire suppression을 수행하지 않으며, Redis에 NPC 상태를 저장하지 않습니다. 프론트엔드가 spawn event를 받아 소방차 오버레이를 자체적으로 렌더링합니다.

### 스폰 조건

- **단계 4 (대화재, 120+) 이상**에서만 스폰됩니다.
- `triggers_firefighter` 필드가 `True`인 단계: 4(대화재), 5(전소)

### `remove_per_sweep` 값

| 단계 | `remove_per_sweep` | 비고 |
|:---:|:---:|---|
| 0 -- 3 | 0 | 소방관 스폰 없음 |
| 4 (대화재) | 2 | 정보 표시용 (실제 suppression 없음) |
| 5 (전소) | 3 | 정보 표시용 (실제 suppression 없음) |

### NPC 생성 흐름

```python
# server/models/firefighter.py

npc = FirefighterNPC(
    npc_id="ff-{uuid8}",       # 예: "ff-a1b2c3d4"
    grid_id=grid_id,
    status="dispatched",
    dispatched_at=time.time(),
    fires_removed=0,            # 항상 0 (suppression 없음)
    remove_per_sweep=max(remove_count, 1),
    target_stage=stage.value,
)
```

`firefighter:spawn` event가 grid room과 global 양쪽에 broadcast됩니다.

## 확산 (Spread) 메커니즘

하나의 grid cell이 포화 상태에 도달하면, 새로 등록되는 fire는 인접 cell로 "확산"됩니다.

### 핵심 설정값

| 상수 | 값 | 설명 |
|---|---|---|
| `FIRE_SPREAD_THRESHOLD` | 500 | cell당 active fire 상한. 이 값 이상이면 확산 발생 |
| `FIRE_MAX_CASCADE_DEPTH` | 8 | 최대 연쇄 확산 깊이 (hop 수) |

### 확산 알고리즘

`register_fire()` 내부에서 다음과 같이 동작합니다:

1. 현재 grid의 active fire 수를 확인합니다.
2. `active_count >= FIRE_SPREAD_THRESHOLD`(500)이면, **8방향 이웃 중 하나를 무작위 선택**(`random.choice(get_neighbors_8(current_grid))`)하여 이동합니다.
3. 이동한 이웃도 포화 상태이면, 다시 그 이웃의 8방향 중 하나로 이동합니다 (cascade).
4. 이 과정을 최대 `FIRE_MAX_CASCADE_DEPTH`(8) 회까지 반복합니다.
5. 최대 깊이에 도달하면, 포화 여부와 무관하게 해당 grid에 강제 착지합니다.

```
[grid A: 500+] --spread--> [grid B: 500+] --spread--> [grid C: 200] -- 착지!
                                                        (cascade depth=2)
```

확산이 발생하면 `fire:spread` event가 global로 broadcast되어, 클라이언트가 확산 애니메이션을 표시할 수 있습니다:

```json
{
  "path": [
    {"from": "gridA", "to": "gridB"},
    {"from": "gridB", "to": "gridC"}
  ],
  "event_id": "unique-event-id",
  "timestamp": 1700000000.0
}
```

## TTL 정책

### 서버 설정

Fire event는 Redis Sorted Set에 저장되며, score가 만료 시각(expiration timestamp)입니다.

```
Key:    fire:{gridId}
Score:  now + FIRE_TTL_SEC (만료 시각)
Member: unique event ID
```

- **`FIRE_TTL_SEC`**: 기본 **2400초 (40분)**, 환경변수 `FIRE_TTL_SEC`로 override 가능

활성 fire 판별: `score > current_time`인 member만 활성으로 간주합니다.

### 만료 로직

두 단계로 만료를 처리합니다:

1. **활성 수 계산 시 자동 제외** -- `_get_active_count()`가 `ZCOUNT key now +inf`로 조회하므로, 만료된 event는 count에 포함되지 않습니다. 별도 삭제 없이도 단계 계산에서 즉시 제외됩니다.
2. **주기적 정리 (cleanup loop)** -- `cleanup_interval`(기본 60초) 마다 `ZREMRANGEBYSCORE key -inf now`로 만료된 member를 실제 삭제합니다. 빈 key는 `DEL` 후 `active_grids` set에서도 제거합니다.

### TTL 불일치 주의

> **Warning**: 서버와 클라이언트 간 TTL 값이 다릅니다.
>
> | 위치 | 상수명 | 값 |
> |---|---|---|
> | Server (`config.py`) | `FIRE_TTL_SEC` | **2400초 (40분)** |
> | Client | `TTL_SECONDS` | **1800초 (30분)** |
>
> 서버는 40분 동안 fire를 활성 상태로 유지하지만, 클라이언트는 30분 후에 자체적으로 fire를 제거할 수 있습니다. 이로 인해 서버에서는 아직 활성인 fire가 클라이언트 화면에서 사라지는 **10분간의 유령 fire(ghost fire)** 구간이 발생할 수 있습니다.

## 관련 설정값 (config.py)

`server/config.py`에 정의된 fire 시스템 관련 설정값 전체 목록입니다:

| 상수 | 기본값 | 환경변수 override | 설명 |
|---|---|---|---|
| `FIRE_TTL_SEC` | `2400` (40분) | `FIRE_TTL_SEC` | fire event의 TTL |
| `KST` | `Asia/Seoul` | - | 일별 통계 기준 timezone |
| `STATS_TOTAL_FIRES_KEY` | `"stats:total_fires"` | - | 누적 fire 수 Redis key |
| `STATS_DAILY_FIRES_PREFIX` | `"stats:daily_fires:"` | - | 일별 fire 수 key prefix |
| `STATS_DAILY_FIRES_TTL_SEC` | `172800` (48시간) | - | 일별 fire 수 key TTL |
| `STATS_DAILY_RANKING_PREFIX` | `"stats:daily_ranking:"` | - | 일별 지역 순위 key prefix |
| `STATS_DAILY_RANKING_TTL_SEC` | `172800` (48시간) | - | 일별 지역 순위 key TTL |

`fire_progression.py`에 정의된 추가 상수:

| 상수 | 값 | 설명 |
|---|---|---|
| `FIRE_KEY_PREFIX` | `"fire:"` | Redis sorted set key prefix |
| `ACTIVE_GRIDS_KEY` | `"active_grids"` | 활성 grid ID 추적용 Redis Set key |
| `FIRE_SPREAD_THRESHOLD` | `500` | cell당 fire 상한, 초과 시 이웃으로 확산 |
| `FIRE_MAX_CASCADE_DEPTH` | `8` | 확산 cascade 최대 깊이 |

`FireProgressionEngine` 기본 interval:

| 파라미터 | 기본값 | 설명 |
|---|---|---|
| `scan_interval` | `2.0`초 | 단계 전환 감지 주기 |
| `cleanup_interval` | `60.0`초 | 만료 event 정리 주기 |
