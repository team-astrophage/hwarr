# Grid System

화르르의 핵심 공간 체계인 grid system을 설명한다. 모든 불(fire)은 GPS 좌표가 아닌 **grid cell** 단위로 집계되며, 서버와 클라이언트가 동일한 알고리즘으로 좌표를 grid ID로 변환한다.

---

## 개요

지도 위의 모든 좌표를 약 **100m x 100m** 크기의 격자(grid cell)로 양자화한다. 사용자가 불을 지르면 해당 GPS 좌표가 grid ID로 변환되고, 같은 grid cell에 속하는 불은 하나의 셀에 합산된다.

```
┌──────────┬──────────┬──────────┐
│          │          │          │
│  41111:  │  41111:  │  41111:  │
│  33518   │  33519   │  33520   │
│          │          │          │
├──────────┼──────────┼──────────┤
│          │ ★여기에  │          │
│  41110:  │  41110:  │  41110:  │
│  33518   │  33519   │  33520   │
│          │          │          │
├──────────┼──────────┼──────────┤
│          │          │          │
│  41109:  │  41109:  │  41109:  │
│  33518   │  33519   │  33520   │
│          │          │          │
└──────────┴──────────┴──────────┘

각 셀 ≈ 100m x 100m (한국 ~37°N 기준)
```

---

## 상수

| 상수 | 값 | 의미 |
|---|---|---|
| `LAT_UNIT` | `0.0009` | 위도 1셀 크기 (약 100m) |
| `LNG_UNIT` | `0.0011` | 경도 1셀 크기 (약 100m, 한국 ~37°N 기준) |

### 왜 위도와 경도의 단위가 다른가?

지구는 구(sphere)에 가까운 형태이므로, 위도 1도와 경도 1도가 나타내는 실제 거리가 다르다. 위도 37°N(한국 중부) 기준:

- **위도 1도** ≈ 111km → `0.0009도` ≈ 100m
- **경도 1도** ≈ 88.8km → `0.0011도` ≈ 97.7m ≈ 100m

이 값은 한국 전역(33°N ~ 38°N)에서 약 95~105m 범위로 충분히 유효하다.

### 정의 위치

| 파일 | 언어 |
|---|---|
| `server/grid.py` | Python |
| `client/src/lib/config.ts` | TypeScript |

---

## Grid ID 포맷

Grid ID는 문자열이며, 포맷은 다음과 같다:

```
"{floor(lat / LAT_UNIT)}:{floor(lng / LNG_UNIT)}"
```

예시:

| GPS 좌표 | 계산 | Grid ID |
|---|---|---|
| `(37.5665, 126.9780)` | `floor(37.5665 / 0.0009)` = `41740`, `floor(126.9780 / 0.0011)` = `115434` | `"41740:115434"` |
| `(37.5670, 126.9780)` | `floor(37.5670 / 0.0009)` = `41741`, `floor(126.9780 / 0.0011)` = `115434` | `"41741:115434"` |
| `(35.1796, 129.0756)` | `floor(35.1796 / 0.0009)` = `39088`, `floor(129.0756 / 0.0011)` = `117341` | `"39088:117341"` |

> 첫 번째와 두 번째 좌표는 위도가 약 55m 차이(0.0005도)지만, `floor` 연산에 의해 서로 다른 셀에 배정될 수 있다.

---

## 변환 함수

### `to_grid_id(lat, lng)` / `getGridId(lat, lng)`

GPS 좌표를 grid ID 문자열로 변환한다.

**서버 (Python)**

```python
# server/grid.py
def to_grid_id(lat: float, lng: float) -> str:
    grid_lat = math.floor(lat / LAT_UNIT)
    grid_lng = math.floor(lng / LNG_UNIT)
    return f"{grid_lat}:{grid_lng}"
```

**클라이언트 (TypeScript)**

```typescript
// client/src/features/fire-map/utils/grid.ts
export function getGridId(lat: number, lng: number): string {
  const gridLat = Math.floor(lat / LAT_UNIT)
  const gridLng = Math.floor(lng / LNG_UNIT)
  return `${gridLat}:${gridLng}`
}
```

---

### `grid_id_to_center(grid_id)` / `getGridCenter(gridId)`

Grid ID를 해당 셀의 **중심 좌표**로 역변환한다. 지도에 셀을 표시할 때 사용한다.

**서버 (Python)**

```python
def grid_id_to_center(grid_id: str) -> tuple[float, float]:
    parts = grid_id.split(":")
    grid_lat = int(parts[0])
    grid_lng = int(parts[1])
    center_lat = (grid_lat + 0.5) * LAT_UNIT
    center_lng = (grid_lng + 0.5) * LNG_UNIT
    return (center_lat, center_lng)
```

**클라이언트 (TypeScript)**

```typescript
export function getGridCenter(gridId: string): [number, number] {
  const [latStr, lngStr] = gridId.split(':')
  const lat = Number(latStr) * LAT_UNIT + LAT_UNIT / 2
  const lng = Number(lngStr) * LNG_UNIT + LNG_UNIT / 2
  return [lat, lng]
}
```

**변환 예시:**

```
Grid ID: "41740:115434"

center_lat = (41740 + 0.5) * 0.0009 = 37.56645
center_lng = (115434 + 0.5) * 0.0011 = 126.97795

→ (37.56645, 126.97795)
```

> 원래 좌표 `(37.5665, 126.9780)`과 비교하면, 최대 오차는 셀 크기의 절반(약 50m)이다.

---

### `get_neighbors_8(grid_id)` (서버 전용)

주어진 grid cell의 8방향(상하좌우 + 대각선) 이웃 셀 ID를 반환한다. 불 번짐(fire spread) 로직에서 사용한다.

```python
def get_neighbors_8(grid_id: str) -> list[str]:
    parts = grid_id.split(":")
    grid_lat = int(parts[0])
    grid_lng = int(parts[1])
    return [
        f"{grid_lat + dlat}:{grid_lng + dlng}"
        for dlat in (-1, 0, 1)
        for dlng in (-1, 0, 1)
        if not (dlat == 0 and dlng == 0)
    ]
```

**예시:**

```
입력: "41740:115434"

출력 (8개):
  "41739:115433"  "41739:115434"  "41739:115435"
  "41740:115433"       (자기)      "41740:115435"
  "41741:115433"  "41741:115434"  "41741:115435"
```

화르르는 **unbounded sparse grid**를 사용하므로 경계 검사가 없다. 모든 이웃 좌표가 유효한 grid ID이다.

---

### `get_grids_in_viewport(ne_lat, ne_lng, sw_lat, sw_lng)` (서버 전용)

지도 viewport(bounding box)에 포함되는 모든 grid ID를 반환한다. 클라이언트가 현재 보고 있는 영역의 불 데이터를 요청할 때 사용한다.

```python
def get_grids_in_viewport(
    ne_lat: float, ne_lng: float, sw_lat: float, sw_lng: float
) -> list[str]:
    grids = []
    lat = math.floor(sw_lat / LAT_UNIT)
    lat_max = math.floor(ne_lat / LAT_UNIT)
    lng_min = math.floor(sw_lng / LNG_UNIT)
    lng_max = math.floor(ne_lng / LNG_UNIT)

    while lat <= lat_max:
        lng = lng_min
        while lng <= lng_max:
            grids.append(f"{lat}:{lng}")
            lng += 1
        lat += 1
    return grids
```

**예시:**

서울 시청 부근을 보고 있다면:

```
viewport: ne=(37.568, 126.980), sw=(37.565, 126.976)

lat  범위: floor(37.565/0.0009)=41738 ~ floor(37.568/0.0009)=41742  → 5행
lng 범위: floor(126.976/0.0011)=115432 ~ floor(126.980/0.0011)=115436 → 5열

총 25개 grid ID 반환
```

---

## 클라이언트-서버 동기화

### 동일 구현이 필요한 이유

클라이언트와 서버 양쪽에서 grid 변환을 수행하는 이유:

1. **클라이언트**: 사용자가 탭한 위치의 grid cell을 즉시 하이라이트하고, 수신된 grid ID를 지도 좌표로 변환하여 렌더링한다.
2. **서버**: 불 생성 요청의 GPS 좌표를 grid ID로 변환하여 Redis에 저장하고, 이웃 셀 계산 및 viewport 쿼리를 처리한다.

양쪽의 `floor` 연산과 상수가 **정확히 일치하지 않으면**, 클라이언트가 표시하는 셀 위치와 서버가 저장한 셀 위치가 어긋난다.

### 주의점

| 항목 | 설명 |
|---|---|
| **상수 동기화** | `LAT_UNIT`, `LNG_UNIT` 값을 변경할 때 반드시 `server/grid.py`와 `client/src/lib/config.ts` **양쪽 모두** 수정해야 한다. |
| **floor 연산** | Python의 `math.floor`와 JavaScript의 `Math.floor`는 음수에서도 동일하게 동작한다 (둘 다 음의 무한대 방향으로 내림). 예: `floor(-0.5)` = `-1`. |
| **부동소수점** | IEEE 754 double 기준으로 두 언어 모두 동일한 결과를 낸다. 다만, 극단적인 경계값에서 미세한 차이가 발생할 수 있으므로 테스트 시 주의한다. |
| **함수 이름 차이** | 서버(`to_grid_id`, `grid_id_to_center`)와 클라이언트(`getGridId`, `getGridCenter`)는 네이밍 컨벤션만 다르고 로직은 동일하다. |

---

## 관련 파일

| 파일 | 역할 |
|---|---|
| `server/grid.py` | 서버 grid 변환, 이웃 셀 계산, viewport 쿼리 |
| `server/config.py` | 서버 설정 (`FIRE_TTL_SEC` 등) |
| `client/src/lib/config.ts` | 클라이언트 grid 상수 (`LAT_UNIT`, `LNG_UNIT`, `TTL_SECONDS`) |
| `client/src/features/fire-map/utils/grid.ts` | 클라이언트 grid 변환 함수 |
| `server/models/fire.py` | 불 단계(FireStage) 정의 및 grid state 빌드 로직 |
