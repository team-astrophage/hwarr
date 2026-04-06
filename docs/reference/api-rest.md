# REST API Reference

화르르(hwarr) 서버의 REST API 문서입니다. 모든 endpoint는 JSON 응답을 반환하며, base path는 서버 루트(`/`)입니다.

**Base URL:** `https://<server-host>:<port>`

---

## 목차

- [Health Check](#health-check)
- [Fire (방화/화재)](#fire-방화화재)
- [Stats (통계)](#stats-통계)
- [Ranking (랭킹)](#ranking-랭킹)
- [News (속보)](#news-속보)
- [Map Config (지도 설정)](#map-config-지도-설정)
- [Demo (데모 모드)](#demo-데모-모드)
- [QR Code (QR 코드)](#qr-code-qr-코드)
- [Feedback (피드백)](#feedback-피드백)
- [공통 에러 응답](#공통-에러-응답)

---

## Health Check

ALB(Application Load Balancer) health check용 endpoint입니다.

### `GET /health`

서버 상태와 현재 연결 수, Fire engine 동작 여부를 반환합니다.

**응답 JSON:**

```json
{
  "status": "ok",
  "connections": 42,
  "engine_running": true
}
```

| 필드 | 타입 | 설명 |
|------|------|------|
| `status` | `string` | 항상 `"ok"` |
| `connections` | `int` | 현재 WebSocket 연결 수 |
| `engine_running` | `bool` | FireProgressionEngine 가동 여부 |

**curl 예시:**

```bash
curl http://localhost:8000/health
```

---

## Fire (방화/화재)

GPS 좌표 기반 방화 및 grid 상태 조회 endpoint입니다.

Router prefix: `/api`, tag: `fire`

### `POST /api/fire`

GPS 좌표에 불을 점화합니다. Redis에 fire event를 등록하고, Socket.IO 클라이언트에 `fire:ignite` 및 `fire:global_update` 이벤트를 broadcast합니다.

**요청 Body:**

| 필드 | 타입 | 필수 | 설명 |
|------|------|------|------|
| `lat` | `float` | O | 위도 (decimal degrees) |
| `lng` | `float` | O | 경도 (decimal degrees) |

**응답 (201 Created):**

```json
{
  "grid_id": "37.4979_127.0276",
  "event_id": "fire-a1b2c3d4e5f6",
  "active_count": 15,
  "stage": 2,
  "stage_info": {
    "label": "...",
    "firefighter_trigger": false
  }
}
```

| 필드 | 타입 | 설명 |
|------|------|------|
| `grid_id` | `string` | 불이 점화된 grid cell ID |
| `event_id` | `string` | 고유 fire event 식별자 (`fire-` prefix + 12자 hex) |
| `active_count` | `int` | 해당 grid의 현재 활성 fire 수 |
| `stage` | `int` | 현재 화재 단계 (0-5) |
| `stage_info` | `object` | 단계 metadata (label, firefighter trigger 등) |

**동작 흐름:**

1. GPS (lat, lng) -> grid cell ID 변환
2. 고유 event ID 생성, TTL 40분(`FIRE_TTL_SEC=2400`) 설정
3. Redis에 fire event 등록 (인접 grid 확산, 단계 판정, 소방관 NPC spawn 포함)
4. Socket.IO broadcast
5. 호출자에게 grid state 반환

**에러:**

| 코드 | 조건 |
|------|------|
| `503` | Redis 미연결 (Fire engine 사용 불가) |

**curl 예시:**

```bash
curl -X POST http://localhost:8000/api/fire \
  -H "Content-Type: application/json" \
  -d '{"lat": 37.4979, "lng": 127.0276}'
```

---

### `GET /api/grid/viewport`

지도 viewport(bounding box) 내의 모든 활성 화재 grid를 조회합니다.

**Query Parameters:**

| 파라미터 | 타입 | 필수 | 설명 |
|----------|------|------|------|
| `ne_lat` | `float` | O | 북동 꼭짓점 위도 |
| `ne_lng` | `float` | O | 북동 꼭짓점 경도 |
| `sw_lat` | `float` | O | 남서 꼭짓점 위도 |
| `sw_lng` | `float` | O | 남서 꼭짓점 경도 |

**응답 (200 OK):**

```json
{
  "grids": [
    {
      "grid_id": "37.4979_127.0276",
      "active_count": 15,
      "stage": 2,
      "stage_info": { "...": "..." },
      "lat": 37.4979,
      "lng": 127.0276
    }
  ],
  "total_active_grids": 1
}
```

| 필드 | 타입 | 설명 |
|------|------|------|
| `grids` | `list[object]` | 활성 fire가 있는 grid 상태 목록 |
| `total_active_grids` | `int` | 활성 grid 수 |

**에러:**

| 코드 | 조건 |
|------|------|
| `422` | 필수 query parameter 누락 또는 타입 오류 |
| `503` | Redis 미연결 |

**curl 예시:**

```bash
curl "http://localhost:8000/api/grid/viewport?ne_lat=37.6&ne_lng=127.1&sw_lat=37.4&sw_lng=126.9"
```

---

### `GET /api/grid/{grid_id}`

단일 grid cell의 현재 화재 상태를 조회합니다.

**Path Parameters:**

| 파라미터 | 타입 | 설명 |
|----------|------|------|
| `grid_id` | `string` | Grid cell ID (예: `"37.4979_127.0276"`) |

**응답 (200 OK):**

```json
{
  "grid_id": "37.4979_127.0276",
  "active_count": 15,
  "stage": 2,
  "stage_info": {
    "label": "...",
    "firefighter_trigger": false
  }
}
```

**에러:**

| 코드 | 조건 |
|------|------|
| `503` | Redis 미연결 |

**curl 예시:**

```bash
curl http://localhost:8000/api/grid/37.4979_127.0276
```

> **참고:** `/api/grid/viewport` 경로가 `/api/grid/{grid_id}`보다 먼저 등록되어 있으므로, `"viewport"` 문자열이 grid_id로 잘못 캡처되지 않습니다.

---

## Stats (통계)

랜딩 페이지용 전역 화재 통계 endpoint입니다.

Router prefix: `/api`, tag: `stats`

### `GET /api/stats`

전역 화재 통계를 camelCase로 반환합니다 (프론트엔드 호환).

**응답 (200 OK):**

```json
{
  "activeGrids": 5,
  "totalFires": 128,
  "cumulativeFires": 4523,
  "dailyFires": 312,
  "onlineUsers": 42
}
```

| 필드 | 타입 | 설명 |
|------|------|------|
| `activeGrids` | `int` | 현재 활성 화재가 있는 grid 수 |
| `totalFires` | `int` | 현재 활성 fire event 총 수 |
| `cumulativeFires` | `int` | 누적 fire event 수 (Redis AOF 영속) |
| `dailyFires` | `int` | 오늘(KST 기준) fire event 수 |
| `onlineUsers` | `int` | 현재 WebSocket 접속자 수 |

**에러:**

| 코드 | 조건 |
|------|------|
| `503` | Redis 미연결 |

**curl 예시:**

```bash
curl http://localhost:8000/api/stats
```

---

## Ranking (랭킹)

일간 지역별 방화 랭킹 endpoint입니다.

Router prefix: `/api`, tag: `ranking`

### `GET /api/ranking/today`

오늘(KST 기준) 가장 많은 방화가 발생한 지역 순위를 반환합니다.

**Query Parameters:**

| 파라미터 | 타입 | 필수 | 기본값 | 설명 |
|----------|------|------|--------|------|
| `limit` | `int` | X | `10` | 반환할 최대 지역 수 (1-50) |

**응답 (200 OK):**

```json
{
  "date": "2026-04-06",
  "items": [
    { "rank": 1, "region": "서울 강남구", "count": 234 },
    { "rank": 2, "region": "서울 종로구", "count": 189 },
    { "rank": 3, "region": "부산 해운대구", "count": 145 }
  ]
}
```

| 필드 | 타입 | 설명 |
|------|------|------|
| `date` | `string` | 조회 기준 날짜 (KST, `YYYY-MM-DD`) |
| `items` | `list[object]` | 랭킹 항목 목록 |
| `items[].rank` | `int` | 순위 (1부터 시작) |
| `items[].region` | `string` | 행정구역 이름 |
| `items[].count` | `int` | 방화 횟수 |

Admin region resolver가 비활성화되어 있거나 오늘 화재가 없으면 `items`는 빈 배열입니다.

**에러:**

| 코드 | 조건 |
|------|------|
| `422` | `limit` 범위 초과 (1-50) |
| `503` | Redis 미연결 |

**curl 예시:**

```bash
curl "http://localhost:8000/api/ranking/today?limit=5"
```

---

## News (속보)

실시간 화재 데이터 기반 속보 피드 endpoint입니다.

Router prefix: `/api`, tag: `news`

### `GET /api/news`

현재 활성 화재를 기반으로 속보 스타일의 뉴스 피드를 생성합니다. 최대 5개 항목을 반환하며, 화재 단계와 활성 fire 수 기준으로 내림차순 정렬됩니다.

**응답 (200 OK):**

```json
[
  {
    "id": "news-bf-37.4979_127.0276",
    "icon": "🔥",
    "icon_bg": "rgba(255,68,68,0.15)",
    "headline_parts": [
      { "text": "강남 테헤란로", "highlight": "accent" },
      { "text": " 일대 " },
      { "text": "대화재", "highlight": "warning" },
      { "text": " 발생" }
    ],
    "time": "5분 전",
    "detail": "활성 방화범 523명 · 5단계 지속 중"
  }
]
```

| 필드 | 타입 | 설명 |
|------|------|------|
| `id` | `string` | 뉴스 항목 고유 ID (`news-bf-`, `news-fa-`, `news-sf-` prefix) |
| `icon` | `string` | 아이콘 이모지 |
| `icon_bg` | `string` | 아이콘 배경색 (CSS rgba) |
| `headline_parts` | `list[object]` | 헤드라인 파트 목록 (highlight 렌더링용) |
| `headline_parts[].text` | `string` | 텍스트 내용 |
| `headline_parts[].highlight` | `string \| null` | highlight 타입 (`"accent"`, `"warning"`, 또는 `null`) |
| `time` | `string` | 상대 시간 (예: `"방금 전"`, `"5분 전"`, `"1시간 전"`) |
| `detail` | `string` | 상세 설명 |

**뉴스 템플릿 구분:**

| 화재 단계 | 템플릿 | ID prefix |
|-----------|--------|-----------|
| stage >= 4 | 대화재 발생 | `news-bf-` |
| stage 2-3 | 확산 중 | `news-fa-` |
| stage 0-1 | 불씨 감지 | `news-sf-` |

**위치 해석 (3계층):**

1. Landmark (좁은 범위 구체 장소: 강남 테헤란로, 홍대입구 등)
2. District (서울 25개 구 + 주요 도시 구/군)
3. Province (시/도 단위)
4. Fallback: 좌표 기반 (`"N37.5° E127.0° 부근"`)

**에러:**

| 코드 | 조건 |
|------|------|
| `503` | Redis 미연결 |

**curl 예시:**

```bash
curl http://localhost:8000/api/news
```

---

## Map Config (지도 설정)

프론트엔드 지도 초기화를 위한 설정 endpoint입니다.

Router prefix: `/api`, tag: `map`

### `GET /api/map/config`

지도 설정과 사전 정의된 클릭 가능 화재 위치 목록을 반환합니다. 응답 데이터는 static이며 서버 시작 시 생성됩니다.

**응답 (200 OK):**

```json
{
  "center_lat": 36.5,
  "center_lng": 127.8,
  "default_zoom": 7,
  "min_zoom": 6,
  "max_zoom": 18,
  "bounds": {
    "ne_lat": 38.6,
    "ne_lng": 131.9,
    "sw_lat": 33.0,
    "sw_lng": 124.5
  },
  "grid": {
    "lat_unit": 0.0009,
    "lng_unit": 0.0011,
    "cell_size_meters": 100
  },
  "locations": [
    {
      "id": "gwanghwamun",
      "name": "광화문광장",
      "lat": 37.576,
      "lng": 126.9769,
      "grid_id": "37.5756_126.9769",
      "description": "서울 광화문광장"
    }
  ]
}
```

| 필드 | 타입 | 설명 |
|------|------|------|
| `center_lat` | `float` | 기본 지도 중심 위도 |
| `center_lng` | `float` | 기본 지도 중심 경도 |
| `default_zoom` | `int` | 기본 zoom 레벨 |
| `min_zoom` | `int` | 최소 zoom 레벨 |
| `max_zoom` | `int` | 최대 zoom 레벨 |
| `bounds` | `object` | 대한민국 bounding box |
| `grid` | `object` | Grid system metadata |
| `grid.lat_unit` | `float` | Grid cell 위도 단위 (degrees) |
| `grid.lng_unit` | `float` | Grid cell 경도 단위 (degrees) |
| `grid.cell_size_meters` | `int` | Grid cell 크기 (약 100m) |
| `locations` | `list[object]` | 사전 정의 화재 위치 목록 (16개) |
| `locations[].id` | `string` | 위치 식별자 |
| `locations[].name` | `string` | 표시 이름 (한국어) |
| `locations[].lat` | `float` | 위도 |
| `locations[].lng` | `float` | 경도 |
| `locations[].grid_id` | `string` | 사전 계산된 grid cell ID |
| `locations[].description` | `string` | 위치 설명 |

**사전 정의 위치 목록:**

| ID | 이름 | 도시 |
|----|------|------|
| `gwanghwamun` | 광화문광장 | 서울 |
| `gangnam` | 강남역 | 서울 |
| `hongdae` | 홍대입구 | 서울 |
| `yeouido` | 여의도공원 | 서울 |
| `jamsil` | 잠실종합운동장 | 서울 |
| `namsan` | 남산타워 | 서울 |
| `itaewon` | 이태원 | 서울 |
| `busan_haeundae` | 해운대해수욕장 | 부산 |
| `daegu_dongseong` | 동성로 | 대구 |
| `incheon_songdo` | 송도센트럴파크 | 인천 |
| `gwangju_chungjang` | 충장로 | 광주 |
| `daejeon_dunsan` | 둔산동 | 대전 |
| `jeju_hallasan` | 한라산 | 제주 |
| `suwon_hwaseong` | 수원화성 | 수원 |
| `gyeongju_bulguksa` | 불국사 | 경주 |
| `coex` | 코엑스 | 서울 |

**curl 예시:**

```bash
curl http://localhost:8000/api/map/config
```

---

## Demo (데모 모드)

GPS를 사용할 수 없는 환경에서의 fallback endpoint입니다. `?demo=true` 또는 GPS 권한 거부 시 프론트엔드가 사용합니다.

Router prefix: `/api/demo`, tag: `demo`

### `GET /api/demo/location`

랜덤(또는 지정) 데모 위치를 반환합니다.

**Query Parameters:**

| 파라미터 | 타입 | 필수 | 설명 |
|----------|------|------|------|
| `location_id` | `string` | X | 특정 위치 ID (생략 시 랜덤 반환) |

**응답 (200 OK):**

```json
{
  "id": "gangnam",
  "name": "강남역",
  "lat": 37.4979,
  "lng": 127.0276,
  "grid_id": "37.4975_127.0275",
  "description": "서울 강남역 사거리",
  "demo": true
}
```

**에러:**

| 코드 | 조건 |
|------|------|
| `404` | 지정한 `location_id`가 존재하지 않음 |

**curl 예시:**

```bash
# 랜덤 위치
curl http://localhost:8000/api/demo/location

# 특정 위치
curl "http://localhost:8000/api/demo/location?location_id=gangnam"
```

---

### `GET /api/demo/locations`

사용 가능한 모든 데모 위치 목록을 반환합니다.

**응답 (200 OK):**

```json
{
  "locations": [
    {
      "id": "gwanghwamun",
      "name": "광화문광장",
      "lat": 37.576,
      "lng": 126.9769,
      "grid_id": "37.5756_126.9769",
      "description": "서울 광화문광장",
      "demo": true
    }
  ],
  "total": 16,
  "demo": true
}
```

| 필드 | 타입 | 설명 |
|------|------|------|
| `locations` | `list[object]` | 데모 위치 목록 |
| `total` | `int` | 전체 위치 수 |
| `demo` | `bool` | 항상 `true` |

**curl 예시:**

```bash
curl http://localhost:8000/api/demo/locations
```

---

### `POST /api/demo/fire`

데모 위치에서 불을 점화합니다. GPS 없이 사용 가능한 `POST /api/fire`의 데모 버전입니다.

**요청 Body:**

| 필드 | 타입 | 필수 | 설명 |
|------|------|------|------|
| `location_id` | `string` | X | 위치 ID (생략 시 랜덤 선택) |

Body 자체를 생략해도 됩니다 (빈 body 또는 `{}`).

**응답 (201 Created):**

```json
{
  "grid_id": "37.4975_127.0275",
  "event_id": "fire-demo-a1b2c3d4e5f6",
  "active_count": 3,
  "stage": 1,
  "stage_info": { "...": "..." },
  "location": {
    "id": "gangnam",
    "name": "강남역",
    "lat": 37.4979,
    "lng": 127.0276,
    "grid_id": "37.4975_127.0275",
    "description": "서울 강남역 사거리",
    "demo": true
  },
  "demo": true
}
```

| 필드 | 타입 | 설명 |
|------|------|------|
| `grid_id` | `string` | 점화된 grid cell ID |
| `event_id` | `string` | 고유 fire event ID (`fire-demo-` prefix) |
| `active_count` | `int` | 해당 grid 활성 fire 수 |
| `stage` | `int` | 화재 단계 (0-5) |
| `stage_info` | `object` | 단계 metadata |
| `location` | `object` | 사용된 데모 위치 정보 |
| `demo` | `bool` | 항상 `true` |

**에러:**

| 코드 | 조건 |
|------|------|
| `404` | 지정한 `location_id`가 존재하지 않음 |
| `503` | Redis 미연결 |

**curl 예시:**

```bash
# 랜덤 위치에 점화
curl -X POST http://localhost:8000/api/demo/fire

# 특정 위치에 점화
curl -X POST http://localhost:8000/api/demo/fire \
  -H "Content-Type: application/json" \
  -d '{"location_id": "gangnam"}'
```

---

## QR Code (QR 코드)

해커톤 현장에서 즉시 접속할 수 있도록 프론트엔드 URL을 QR 코드 이미지로 제공합니다.

Router prefix: `/api`, tag: `qr`

### `GET /api/qr`

프론트엔드 지도 페이지 URL이 인코딩된 QR 코드 PNG 이미지를 반환합니다.

**Query Parameters:**

| 파라미터 | 타입 | 필수 | 기본값 | 설명 |
|----------|------|------|--------|------|
| `demo` | `bool` | X | `false` | `true`이면 URL에 `?demo=true` 포함 |
| `size` | `int` | X | `10` | QR box size (4-40 pixels) |

**응답 (200 OK):**

- Content-Type: `image/png`
- Cache-Control: `public, max-age=3600`
- `X-QR-URL` header: 인코딩된 URL 값

응답은 PNG 바이너리 이미지입니다 (JSON이 아님).

QR에 인코딩되는 URL은 `FRONTEND_URL` 환경변수로 결정됩니다 (기본값: `https://bulpan.example.com`).

**curl 예시:**

```bash
# 기본 QR 코드 다운로드
curl -o qr.png http://localhost:8000/api/qr

# 데모 모드 QR (큰 사이즈)
curl -o qr-demo.png "http://localhost:8000/api/qr?demo=true&size=20"
```

---

## Feedback (피드백)

사용자 피드백을 Discord 채널로 전달하는 endpoint입니다.

Router prefix: `/api`, tag: `feedback`

### `POST /api/feedback`

사용자 피드백을 받아 Discord webhook으로 전송합니다.

**Rate Limiting:** IP 기반, 10분당 최대 3회 (`FEEDBACK_RATE_LIMIT_PER_10MIN=3`). Redis를 통해 관리되며, Redis 불가 시 rate limit 없이 통과합니다.

**요청 Body:**

| 필드 | 타입 | 필수 | 설명 |
|------|------|------|------|
| `category` | `string` | O | 카테고리: `"bug"`, `"idea"`, `"etc"` 중 하나 |
| `message` | `string` | O | 피드백 내용 (1-500자) |
| `email` | `string` | X | 답변 이메일 (최대 200자) |
| `page` | `string` | X | 현재 페이지 경로 (최대 200자) |

**응답 (201 Created):**

```json
{
  "ok": true
}
```

**에러:**

| 코드 | 조건 | `detail` 값 |
|------|------|-------------|
| `422` | 유효성 검사 실패 (category 오류, message 빈값/초과 등) | Pydantic validation error |
| `429` | Rate limit 초과 (10분 내 3회 초과) | `"rate_limited"` |
| `502` | Discord webhook 전송 실패 | `"webhook_failed"` |
| `503` | `FEEDBACK_DISCORD_WEBHOOK_URL` 미설정 | `"feedback_disabled"` |

**Discord Embed 구조:**

피드백은 다음과 같은 형태로 Discord에 전달됩니다:
- 카테고리별 라벨: 버그 제보 / 기능 제안 / 기타 의견
- 카테고리별 색상 코드: bug(빨강), idea(초록), etc(주황)
- 메시지 본문은 blockquote 형식
- Footer에 클라이언트 IP 및 User-Agent 표시
- CloudFront/ALB 환경에서 `X-Forwarded-For` 및 `cloudfront-viewer-user-agent` 헤더를 통해 실제 클라이언트 정보를 식별

**curl 예시:**

```bash
curl -X POST http://localhost:8000/api/feedback \
  -H "Content-Type: application/json" \
  -d '{
    "category": "bug",
    "message": "지도에서 불이 안 보여요",
    "email": "user@example.com",
    "page": "/map"
  }'
```

---

## 공통 에러 응답

모든 endpoint에서 공통으로 발생할 수 있는 에러입니다.

### 503 Service Unavailable

Redis 연결이 끊어졌거나 FireProgressionEngine이 초기화되지 않은 경우:

```json
{
  "detail": "Fire engine not available — Redis may be disconnected"
}
```

### 422 Unprocessable Entity

Pydantic validation 실패 시 (잘못된 타입, 필수 필드 누락 등):

```json
{
  "detail": [
    {
      "loc": ["body", "lat"],
      "msg": "field required",
      "type": "value_error.missing"
    }
  ]
}
```

### 일반 에러 형식

FastAPI 기본 에러 응답 형식을 따릅니다:

```json
{
  "detail": "에러 메시지"
}
```
