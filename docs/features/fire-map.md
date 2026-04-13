# Fire Map 기능 명세서

> 화르르(hwarr) 핵심 기능 — 실시간 방화 지도

---

## 1. 기능 개요

Fire Map은 화르르 서비스의 핵심 페이지로, 사용자가 자신의 현재 위치에 "불을 지르고" 실시간으로 다른 사용자들의 방화 현황을 지도 위에서 확인하는 기능이다.

- **URL**: `/map`
- **Route 정의**: TanStack Router의 `createFileRoute('/map')` 사용
- **진입 컴포넌트**: `MapPage` (`client/src/features/fire-map/components/MapPage.tsx`)
- **Route 파일**: `client/src/routes/map.tsx`

### 페이지 구성 요소

| 레이어 | 컴포넌트 | 역할 |
|--------|----------|------|
| 기본 지도 | `MapContainer` + `TileLayer` | Leaflet 기반 다크 테마 지도 |
| 불 시각화 | `FireOverlay` (= `FireCanvas`) | Canvas 파티클 기반 화염 렌더링 |
| 소방차 | `FiretruckOverlay` | 대형화재 격자 옆 소방차 아이콘 |
| 사용자 위치 | `UserLocationMarker` | 파란 원형 마커 |
| 주소 표시 | 역지오코딩 결과 텍스트 | 좌상단 도로명 주소 |
| 상단 헤더 | `Header` | 로고/네비게이션 |
| 지도 컨트롤 | `MapControls` (via `LocateButton`) | 줌 인/아웃/내 위치 |
| 하단 패널 | `BottomPanel` | 통계 + 불 지르기 버튼 |
| 채팅 | `ChatPanel` | 글로벌 익명 채팅 바텀시트 |
| 위치 권한 | `LocationPermissionModal` | 위치 권한 요청/차단 안내 |
| 면책 고지 | `DisclaimerModal` | 첫 접속 시 면책 확인 팝업 |

---

## 2. 지도 설정

> 소스: `client/src/features/fire-map/components/MapPage.tsx`

### Leaflet `MapContainer` props

| 설정 | 값 | 설명 |
|------|-----|------|
| `center` | `[36.5, 127.5]` (`KOREA_CENTER`) | 대한민국 중앙 좌표 |
| `zoom` | `13` | 초기 줌 레벨 |
| `zoomControl` | `false` | 기본 줌 컨트롤 숨김 (커스텀 사용) |
| `maxBounds` | `[[32.0, 124.0], [39.5, 132.5]]` (`KOREA_BOUNDS`) | 대한민국 전체 영역 (제주도 포함) |
| `maxBoundsViscosity` | `1.0` | bounds 밖으로 드래그 완전 차단 (1.0 = solid wall) |
| `minZoom` | `7` | 최소 줌 (대한민국 전체 보기) |
| `maxZoom` | `18` | 최대 줌 (건물 수준) |

### Tile Layer

| 설정 | 값 |
|------|-----|
| `url` | `https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png` |
| `attribution` | `&copy; <a href="https://carto.com/">CARTO</a>` |
| 테마 | CARTO Dark Matter (다크 테마) |

### 격자 시스템

> 소스: `client/src/lib/config.ts`, `client/src/features/fire-map/utils/grid.ts`

| 상수 | 값 | 의미 |
|------|-----|------|
| `LAT_UNIT` | `0.0009` | 위도 단위 (~100m) |
| `LNG_UNIT` | `0.0011` | 경도 단위 (~100m, 한국 ~37도N 기준) |
| `TTL_SECONDS` | `1800` | 불 지속 시간 (30분) |

격자 ID 생성 방식 (`getGridId`):
```
gridId = `${Math.floor(lat / LAT_UNIT)}:${Math.floor(lng / LNG_UNIT)}`
```

격자 중심 좌표 역산 (`getGridCenter`):
```
lat = Number(latStr) * LAT_UNIT + LAT_UNIT / 2
lng = Number(lngStr) * LNG_UNIT + LNG_UNIT / 2
```

### 줌 레벨별 렌더링 차이

| 줌 범위 | 렌더링 모드 | 설명 |
|---------|-----------|------|
| `< 15` | **Glow Dot** | 글로우 도트 (radial gradient 원) — 원거리 가시성 확보 |
| `>= 15` | **Particle** | Canvas 파티클 기반 화염 애니메이션 + 격자선 |
| `>= 18` | **Particle + Firetruck** | 화염 파티클 + 4단계 이상 격자 옆 소방차 아이콘 |

줌 임계값 상수:
- `GLOW_DOT_ZOOM = 15` (FireCanvas.tsx)
- `FIRETRUCK_MIN_ZOOM = 18` (FiretruckOverlay.tsx)

---

## 3. 위치 권한 흐름

> 소스: `client/src/features/fire-map/hooks/useGeolocation.ts`

### useGeolocation 훅

#### 반환값

```typescript
interface GeolocationState {
  lat: number | null
  lng: number | null
  error: string | null
  loading: boolean
  permissionDenied: boolean
  retry: () => void   // requestLocation 함수 참조
}
```

#### Geolocation API 옵션

| 옵션 | 값 | 설명 |
|------|-----|------|
| `enableHighAccuracy` | `true` | GPS/Wi-Fi 정밀 위치 사용 |
| `timeout` | `10000` (10초) | 위치 요청 타임아웃 |
| `maximumAge` | `0` | 캐시 사용 안 함 (항상 최신 위치) |

#### 동작 흐름

1. **마운트 시**: `navigator.permissions.query({ name: 'geolocation' })` 로 권한 사전 확인
   - `denied` 상태이면 즉시 `permissionDenied = true` 설정 (타임아웃 대기 없이 차단 UI 표시)
   - 권한 API 미지원 브라우저는 무시하고 통과
2. **마운트 시**: `requestLocation()` 호출
3. **requestLocation 내부**:
   - `window.__MELTTOWN_GPS` 백도어 우선 확인
   - `navigator.geolocation` 미지원 시 에러 메시지: `"Geolocation을 지원하지 않는 브라우저입니다"`
   - `getCurrentPosition` 성공 → lat/lng 설정, error/permissionDenied 초기화
   - `getCurrentPosition` 실패 → error 설정, `PERMISSION_DENIED` 코드 시 `permissionDenied = true`

### window.__MELTTOWN_GPS 백도어

> 데모/개발용 GPS 오버라이드

```typescript
declare global {
  interface Window {
    __MELTTOWN_GPS?: { lat: number; lng: number }
  }
}
```

- `requestLocation()` 최초 진입 시 확인
- 설정되어 있으면 Geolocation API를 호출하지 않고 해당 좌표를 즉시 사용
- 브라우저 콘솔에서 `window.__MELTTOWN_GPS = { lat: 37.5665, lng: 126.978 }` 설정 후 페이지 리로드로 사용

### LocationPermissionModal

> 소스: `client/src/features/fire-map/components/LocationPermissionModal.tsx`

#### 표시 조건

`!loading && error` 일 때 표시 (위치 로딩 완료 후 에러가 있는 경우).

#### 두 가지 상태

| | **차단 상태** (`permissionDenied = true`) | **요청 상태** (`permissionDenied = false`) |
|---|---|---|
| **아이콘** | `🔒` | `📍` |
| **제목** | `위치 권한이 차단되었어요` | `위치 권한이 필요해요` |
| **설명** | `브라우저 주소창 왼쪽 자물쇠 아이콘 → 사이트 설정 → 위치 허용으로 변경 후 새로고침 해주세요` | `근처에서 불을 지르려면 위치 공유가 필요합니다` |
| **CTA 버튼 텍스트** | `새로고침` | `위치 권한 허용` |
| **CTA 동작** | `window.location.reload()` | `onRetry()` (= `requestLocation()`) |

#### 스타일

- **배경**: `fixed inset-0 z-[2000] bg-black/60 backdrop-blur-sm`
- **모달 카드**: `bg-[var(--color-bg-surface)] rounded-[16px] shadow-[var(--shadow-heavy)] w-full max-w-[320px] px-6 py-6`
- **정렬**: `flex items-start justify-center pt-[40vh]` (수직 40vh 지점)
- **아이콘**: `text-5xl mb-3`
- **제목**: `text-[1.0625rem] font-bold text-[var(--color-text-primary)] mb-2`
- **설명**: `text-[0.8125rem] text-[var(--color-text-secondary)] leading-relaxed mb-5`
- **CTA 버튼**: `w-full bg-[var(--color-accent)] text-white rounded-[12px] py-3 text-[0.9375rem] font-bold transition-transform active:scale-95`

---

## 4. 역지오코딩 (주소 표시)

> 소스: `client/src/features/fire-map/hooks/useReverseGeocode.ts`

### useReverseGeocode 훅

#### API 호출

| 항목 | 값 |
|------|-----|
| **API** | Nominatim (OpenStreetMap) |
| **URL** | `https://nominatim.openstreetmap.org/reverse?lat=${lat}&lon=${lng}&format=json&accept-language=ko&zoom=18&addressdetails=1` |
| **User-Agent** | `MELTTOWN/1.0` |
| **호출 조건** | `lat`과 `lng`이 모두 non-null일 때 (좌표 변경 시 1회) |
| **취소** | `AbortController`로 언마운트/좌표 변경 시 이전 요청 abort |

#### 주소 파싱 로직

Nominatim 응답의 `address` 객체에서 다음 순서로 추출:

| 필드 | 우선순위 (fallback) | 예시 |
|------|---------------------|------|
| `city` | `addr.city` → `addr.town` → `addr.county` | 서울특별시 |
| `district` | `addr.borough` → `addr.suburb` → `addr.quarter` | 광진구 |
| `road` | `addr.road` + `addr.house_number` (공백 결합) | 구의강변로 45 |

- 세 파트 중 하나라도 있으면 `parts` 객체 반환
- 세 파트 모두 비어있으면 `display_name`의 첫 번째 쉼표 구분 값을 `address`로 사용, `parts`는 `null`

#### 반환값

```typescript
interface ReverseGeocodeState {
  address: string | null       // 결합된 전체 주소 문자열
  parts: AddressParts | null   // { city, district, road }
  loading: boolean
}
```

### UI 표시

> 소스: `client/src/features/fire-map/components/MapPage.tsx` (108-126행)

- **위치**: `absolute top-14 left-4 z-[1000]`
- **표시 조건**: `parts`가 non-null일 때
- **각 줄 (city, district, road)**: 별도 `<p>` 태그
  - 폰트: `text-4xl font-extrabold text-white leading-tight`
  - 각 파트가 비어있으면 해당 줄 미표시

---

## 5. 불 놓기 (Fire Placement)

### useFire 훅

> 소스: `client/src/features/fire-map/hooks/useFire.ts`

#### fire(lat, lng) 함수

| 항목 | 상세 |
|------|------|
| **emit 이벤트** | `socket.emit('fire', { lat, lng })` |
| **쿨다운** | 없음 (즉시 발사) |
| **오프라인 정책** | `socket.connected`가 `false`이면 emit을 **드롭** (전송하지 않음) |
| **드롭 이유** | Socket.IO 기본 동작은 오프라인 emit을 버퍼링 후 재접속 시 flush하는데, 연속 발사와 맞물리면 재접속 직후 수십 건이 서버로 쏟아져 애니메이션/서버 부하 동시 발생 |
| **드롭 시 로그** | `console.warn('[Socket] fire dropped (disconnected)', { lat, lng })` |

### 불 지르기 흐름 (handleFire)

> `MapPage.tsx` 71-76행

1. `lat`, `lng`, `gridId`가 모두 존재하는지 확인
2. `throwMatch(gridId)` — animationStore에 성냥 던지기 애니메이션 등록
3. `fire(lat, lng)` — 소켓으로 서버에 이벤트 전송

### 성냥 던지기 애니메이션

> 소스: `client/src/features/fire-map/components/FireCanvas.tsx` (533-635행)
> `client/src/features/fire-map/stores/animationStore.ts`

#### 비행 시간

`MATCH_DURATION = 500` (ms)

#### 궤적 계산

| 파라미터 | 값 | 설명 |
|----------|-----|------|
| **시작점 X** | `sw / 2` (화면 중앙) | 화면 가로 중앙 |
| **시작점 Y** | `sh - 80` | 화면 하단에서 80px 위 |
| **도착점** | 대상 격자 중심의 container point | `map.latLngToContainerPoint` |
| **수평 이동** | `startX + (targetPt.x - startX) * t` | 선형 보간 |
| **수직 이동** | `baseY - parabola * 120` | 포물선 높이 120px |
| **포물선 공식** | `-4 * t * (t - 1)` | t=0.5에서 최대 높이 1.0 → 120px 상승 |
| **회전** | `t * Math.PI * 4` | 비행 중 4바퀴 회전 (720도) |
| **투명도** | `1 - t * 0.3` | 시작 1.0 → 끝 0.7 |

#### 성냥 외형 (Canvas 드로잉)

| 부위 | 드로잉 | 색상 |
|------|--------|------|
| **막대** | `fillRect(-4, -18, 8, 30)` | `#8B6914` (갈색) |
| **머리** | `arc(0, -18, 7, ...)` | `#ff4444` (빨간색) |
| **불꽃** (t < 0.8) | bezierCurveTo 곡선 | `#ffaa00` (주황), alpha: `0.8 - t` |

#### 착탄 임팩트 효과

> progress 85% ~ 100% 구간 (impactT)

| 요소 | 상세 |
|------|------|
| **중심 플래시** | radial gradient, 반경 `32 + impP * 60` px, 색상 `rgba(255,255,200)` → `rgba(255,180,40)` → `rgba(255,60,0)` |
| **스파크 방사** | 18개 파티클, 각도 균등 분포 + `impP * 0.5` offset, 거리 `impP * (45 + 가변)`, 크기 `(1 - impP) * (2.5 + 가변)` |
| **스파크 색상** | 3색 순환: `#ffffff`, `#ffdd44`, `#ff6600` |
| **불꽃 혀** | 10개 라인, lineWidth `2`, 길이 `impP * (20 + 가변)`, 2색 교대: `#ffaa00` / `#ff4400` |

#### 성냥 제거

`progress >= 1` 시 `removeMatch(m.id)` 호출.

### 랜덤 폭발 메커니즘

> 소스: `animationStore.ts`

5단계(전소) 최초 달성 이후, `throwMatch` 호출마다 `postExplodeTapCount` 증가:
- `RANDOM_EXPLOSION_INTERVAL_MIN = 20`
- `RANDOM_EXPLOSION_INTERVAL_MAX = 30`
- 20~30회 탭마다 랜덤으로 추가 폭발 이펙트 트리거

### BottomPanel: 불 지르기 버튼

> 소스: `client/src/features/fire-map/components/BottomPanel.tsx`

#### 버튼 스타일

| 속성 | 값 |
|------|-----|
| 높이 | `h-12` |
| 너비 | `w-full` |
| 배경색 | `#e4531b` |
| 글자색 | `text-white` |
| 둥근 모서리 | `rounded-[14px]` |
| 폰트 | `font-bold tracking-wide text-[0.8125rem]` |
| active 축소 | `active:scale-[0.96]` |
| active 밝기 | `active:brightness-[0.92]` |
| disabled 상태 | `opacity-35`, `cursor-not-allowed` |
| 그림자 | `shadow-[var(--shadow-medium)]` |
| 터치 최적화 | `select-none touch-none` |

#### 버튼 텍스트

| 조건 | 텍스트 |
|------|--------|
| `noLocation` | `위치를 찾을 수 없어요` |
| `gridId` 존재 | `현재 위치에 불 지르기` |
| `gridId` 없음 | `위치 감지 중...` |

#### 버튼 아이콘

SVG 불꽃 아이콘 (`width="16" height="16"`, `fill="currentColor"`, `shrink-0 opacity-95`).
Path: `M12 23c-3.866 0-7-3.134-7-7 0-3.866 4-9 7-12 3 3 7 8.134 7 12 0 3.866-3.134 7-7 7z`

#### 멀티터치 방지

- `activePointerIdRef` (useRef)로 첫 번째 `pointerId`만 추적
- `onPointerDown`: `activePointerIdRef.current`가 null일 때만 캡처, `setPointerCapture` 호출
- `onPointerUp`: 캡처된 pointer ID와 일치할 때만 `onFire()` 실행
- `onPointerCancel`: 캡처 해제만 수행 (fire 미실행)
- `onContextMenu`: `e.preventDefault()` (롱프레스 컨텍스트 메뉴 차단)

#### disabled 조건

`!lat || !lng` — 위치가 아직 확인되지 않은 상태.

---

## 6. 불 시각화 (FireCanvas)

> 소스: `client/src/features/fire-map/components/FireCanvas.tsx`
> Re-export: `client/src/features/fire-map/components/FireOverlay.tsx` (`FireCanvas`를 `FireOverlay`로 re-export)

### Canvas 설정

- HTML Canvas 엘리먼트를 Leaflet 지도 container에 직접 append
- `position: absolute; top: 0; left: 0; pointer-events: none; z-index: 450`
- DPR 대응: `canvas.width = size.x * window.devicePixelRatio`
- `map.on('resize')` 및 `map.on('zoom')` 이벤트에서 Canvas 리사이즈
- `requestAnimationFrame` 루프로 매 프레임 렌더링

### 줌 < 15: Glow Dot 렌더링

> `GLOW_DOT_ZOOM = 15`

격자마다 격자 중심에 **글로우 도트** 1개를 렌더링.

#### Pulse 애니메이션

```
pulse = Math.sin(frameCount * cfg.dotPulse) * 0.3 + 0.7
dotR = Math.max(cfg.dotSize, 6) * pulse
```

- pulse 범위: `0.4` ~ `1.0` (0.3 진폭, 0.7 기저)
- 최소 dotR: `6 * 0.4 = 2.4` px

#### 렌더링 구조 (globalCompositeOperation = `'lighter'`)

1. **외부 글로우**: radial gradient, 반경 `dotR * 2.5`
   - 색상 정지: `dotColor + '60'` (0%), `dotColor + '20'` (50%), `dotColor + '00'` (100%)
   - globalAlpha: `1`
2. **내부 코어**: radial gradient, 반경 `dotR`
   - 색상 정지: `#ffffcc` (0%), `dotColor` (40%), `dotColor + '00'` (100%)
   - globalAlpha: `pulse`

#### 단계별 Dot 설정

| 단계 | dotColor | dotSize | dotPulse | 시각적 효과 |
|------|----------|---------|----------|-----------|
| 1 (불씨) | `#ff7a45` | `9` | `0.028` | 느린 펄스, 작은 주황 점 |
| 2 (모닥불) | `#ff8c42` | `9` | `0.035` | 약간 빠른 펄스 |
| 3 (화재) | `#ff4500` | `14` | `0.05` | 중간 크기, 빨간 주황 |
| 4 (대화재) | `#ff2200` | `18` | `0.07` | 큰 빨간 점 |
| 5 (지옥불) | `#ff0000` | `22` | `0.09` | 최대 크기, 순수 빨강, 빠른 펄스 |

### 줌 >= 15: Particle 렌더링

격자 너비가 3px 미만이면 스킵 (`if (w < 3) continue`).

#### 격자선

- `globalCompositeOperation = 'source-over'`
- `strokeStyle = 'rgba(120, 60, 60, 0.25)'` (어두운 빨강 톤)
- `lineWidth`: zoom >= 15이면 `1.5`, 그 외 `1`
- `strokeRect(left + 0.5, top + 0.5, w - 1, h - 1)` (anti-alias offset)

#### 단계별 Particle 설정

##### 단계 1: 불씨 (잔불)

| 속성 | 값 |
|------|-----|
| `particleCount` | `26` |
| `maxHeight` | `0.48` (격자 높이의 48%) |
| `baseSpeed` | `0.0065` |
| `spreadX` | `0.16` |
| `turbulence` | `0.0022` |
| `baseSize` | `0.2` (격자 너비의 20%) |
| `colorStops` | `[255,210,90,0.92]` → `[255,100,35,0.65]` → `[220,45,12,0.35]` → `[100,20,0,0]` |
| `coreAlpha` | `0.34` |
| `glowRadius` | `0.44` (격자 너비의 44%) |
| `smokeCount` | `0` |
| `smokeMaxHeight` | `0` |
| `smokeAlpha` | `0` |

##### 단계 2: 모닥불

| 속성 | 값 |
|------|-----|
| `particleCount` | `22` |
| `maxHeight` | `0.6` |
| `baseSpeed` | `0.007` |
| `spreadX` | `0.18` |
| `turbulence` | `0.003` |
| `baseSize` | `0.2` |
| `colorStops` | `[255,230,120,0.95]` → `[255,140,25,0.7]` → `[200,50,0,0.3]` → `[100,15,0,0]` |
| `coreAlpha` | `0.4` |
| `glowRadius` | `0.45` |
| `smokeCount` | `0` |
| `smokeMaxHeight` | `0` |
| `smokeAlpha` | `0` |

##### 단계 3: 화재

| 속성 | 값 |
|------|-----|
| `particleCount` | `45` |
| `maxHeight` | `0.9` |
| `baseSpeed` | `0.01` |
| `spreadX` | `0.28` |
| `turbulence` | `0.004` |
| `baseSize` | `0.26` |
| `colorStops` | `[255,255,200,1.0]` → `[255,180,40,0.9]` → `[255,80,0,0.5]` → `[180,20,0,0]` |
| `coreAlpha` | `0.7` |
| `glowRadius` | `0.6` |
| `smokeCount` | `0` |
| `smokeMaxHeight` | `0` |
| `smokeAlpha` | `0` |

##### 단계 4: 대화재

| 속성 | 값 |
|------|-----|
| `particleCount` | `80` |
| `maxHeight` | `1.4` (격자 높이의 140% — 격자 밖으로 타오름) |
| `baseSpeed` | `0.014` |
| `spreadX` | `0.35` |
| `turbulence` | `0.006` |
| `baseSize` | `0.32` |
| `colorStops` | `[255,255,245,1.0]` → `[255,220,60,1.0]` → `[255,100,0,0.8]` → `[220,30,0,0.15]` |
| `coreAlpha` | `0.95` |
| `glowRadius` | `0.9` |
| `smokeCount` | `12` |
| `smokeMaxHeight` | `2.0` |
| `smokeAlpha` | `0.25` |

##### 단계 5: 지옥불 (전소)

| 속성 | 값 |
|------|-----|
| `particleCount` | `120` |
| `maxHeight` | `1.8` |
| `baseSpeed` | `0.018` |
| `spreadX` | `0.45` |
| `turbulence` | `0.008` |
| `baseSize` | `0.38` |
| `colorStops` | `[255,255,255,1.0]` → `[255,240,80,1.0]` → `[255,60,0,0.9]` → `[180,0,0,0.2]` |
| `coreAlpha` | `1.0` |
| `glowRadius` | `1.2` |
| `smokeCount` | `25` |
| `smokeMaxHeight` | `3.0` |
| `smokeAlpha` | `0.4` |

#### 화염 파티클 물리

파티클 타입 (`Flame`):
```typescript
interface Flame {
  rx: number    // 격자 내 수평 위치 0~1
  ry: number    // 높이 0(바닥)~1(꼭대기)
  vx: number    // 수평 속도
  vy: number    // 수직 속도 (상승)
  life: number  // 남은 수명 (프레임)
  maxLife: number
  size: number  // 파티클 크기
  seed: number  // 랜덤 시드
}
```

**생성 (spawnFlame)**:
- `rx`: `0.3 + Math.random() * 0.4` (격자 중앙 근처)
- `ry`: `0` (바닥에서 시작)
- `vx`: `(Math.random() - 0.5) * cfg.spreadX * 0.02`
- `vy`: `cfg.baseSpeed * (0.7 + Math.random() * 0.6)`
- `life`: `40 + Math.random() * 50` (40~90 프레임)
- `size`: `cfg.baseSize * (0.6 + Math.random() * 0.8)`

**매 프레임 업데이트**:
1. `ry += vy` (상승)
2. `rx += vx` (수평 이동)
3. `vx += (Math.random() - 0.5) * cfg.turbulence` (난류)
4. `vx *= 0.98` (감쇠)
5. `life--`
6. `life <= 0` 또는 `ry > cfg.maxHeight` 시 재생성
7. `rx` 범위 제한: `[0.05, 0.95]`

**렌더링**:
- compositeOperation: `'lighter'` (additive blending)
- 높이 비율(`heightRatio`) → `getFlameColor`로 색상 보간 (바닥: 밝은 노랑/흰, 중간: 주황, 상단: 빨강 → 투명)
- 크기 감쇠: `sizeDecay = 1 - heightRatio * 0.7` (위로 갈수록 작아짐, 역삼각형 형태)
- 파티클 반경: `w * f.size * sizeDecay * 0.5`, 최소 `0.5` px
- alpha: `ca * lifeRatio * sizeDecay`, 최소 `0.01`
- 소프트 radial gradient: 3단계 (중심 풀컬러 → 40% 감쇠 → 100% 투명)

#### 바닥 코어 글로우

각 격자 바닥 중앙에 radial gradient 글로우:
- 위치: `(cx, top + h)` (격자 바닥 중앙)
- 반경: `w * cfg.glowRadius`
- 4단계 gradient:
  - 0%: `rgba(255, 220, 100, coreAlpha)`
  - 30%: `rgba(255, 150, 30, coreAlpha * 0.6)`
  - 70%: `rgba(200, 50, 0, coreAlpha * 0.2)`
  - 100%: `rgba(150, 20, 0, 0)`

#### 클리핑 영역

격자 위로 불꽃/연기가 타오르도록 확장된 클리핑:
```
clipHeight = max(cfg.maxHeight, cfg.smokeMaxHeight)
ctx.rect(left - w * 0.3, top - h * clipHeight, w * 1.6, h * (1 + clipHeight))
```

### 연기 파티클 (단계 4-5)

> `smokeCount > 0`인 단계에서만 렌더링

파티클 타입 (`Smoke`):
```typescript
interface Smoke {
  rx: number; ry: number;
  vx: number; vy: number;
  life: number; maxLife: number;
  size: number;
}
```

**생성 (spawnSmoke)**:
- `rx`: `0.2 + Math.random() * 0.6`
- `ry`: `cfg.maxHeight * 0.6 + Math.random() * cfg.maxHeight * 0.3` (불꽃 위에서 시작)
- `vx`: `(Math.random() - 0.5) * 0.015`
- `vy`: `0.003 + Math.random() * 0.004`
- `life`: `60 + Math.random() * 80` (60~140 프레임)
- `size`: `0.3 + Math.random() * 0.4`

**렌더링**:
- compositeOperation: `'source-over'` (일반 블렌딩)
- 크기: `w * s.size * (0.8 + heightRatio * 0.5)` (위로 갈수록 커짐)
- alpha: `cfg.smokeAlpha * fadeIn * fadeOut * (1 - heightRatio * 0.5)`
  - fadeIn: `Math.min(1, (1 - lifeRatio) * 3)` (생성 초기 페이드인)
  - fadeOut: `lifeRatio` (수명 종료 시 페이드아웃)
- 색상: `gray = 30 + heightRatio * 20` (높을수록 밝아지는 회색, 30~50)
- radial gradient 3단계: 중심 → 60% → 100% (투명)

### 전소 폭발 효과 (단계 5)

> `EXPLOSION_DURATION = 700` ms

5단계 최초 도달 시 `fireStore.updateFire`에서 `triggerExplosion(gridId)` 호출 → `animationStore.explosions`에 등록.

#### 폭발 시퀀스

##### 1. 핵 플래시 (0~80ms)

- 반경: `20 + p * 140` (20px → 160px)
- 4단계 radial gradient:
  - 0%: `rgba(255, 255, 255, coreAlpha)` (순백)
  - 35%: `rgba(255, 245, 210, coreAlpha * 0.95)`
  - 70%: `rgba(255, 180, 80, coreAlpha * 0.6)`
  - 100%: `rgba(255, 120, 20, 0)`
- alpha: `1 - (elapsed / 80)` (80ms에 걸쳐 감소)

##### 2. 파이어볼 (progress 0~60%)

- 반경: `40 + easeOut * 120` (40px → ~160px)
- easeOut: `1 - (1 - progress)^3` (강한 ease-out)
- ballAlpha: `(1 - ballProgress)^1.1`
- 4단계 radial gradient:
  - 0%: `rgba(255, 250, 220, ballAlpha)`
  - 25%: `rgba(255, 180, 60, ballAlpha)`
  - 60%: `rgba(230, 80, 20, ballAlpha * 0.7)`
  - 100%: `rgba(120, 10, 0, 0)`

##### 3. 충격파 링 (이중)

**링 1**:
- 시간: 0~180ms
- 반경: `p * 180` (0 → 180px)
- 색상: `#ffffff`, lineWidth: `4 - p * 3`
- alpha: `(1 - p) * 0.95`

**링 2**:
- 시간: 60~280ms
- 반경: `p * 240` (0 → 240px)
- 색상: `#ffaa44`, lineWidth: `3 - p * 2.5`
- alpha: `(1 - p) * 0.7`

##### 4. 파편 파티클 (18개)

- `particleCount = 18`, `maxRadius = 110` px
- 각도: 균등 분포 `(i / 18) * 2PI` + 랜덤 jitter `(Math.random() - 0.5) * PI/2`
- 거리: `maxRadius * easeOut * speed` (speed: `0.4 + Math.random() * 0.9`)
- 크기: `(1 - progress)^1.4 * (3 + Math.random() * 5)`
- alpha: `(1 - progress)^1.4 * 0.7`
- 색상 4색 순환: `#ffaa44`, `#ff6622`, `#cc2200`, `#661100`

##### 5. 종료

`progress >= 1` 시 `removeExplosion(exp.id)` 호출.

### 확산 궤적 애니메이션

> 서버의 `fire:spread` 이벤트 수신 시 (300 하드캡 초과 → 이웃 그리드로 번짐)

| 상수 | 값 |
|------|-----|
| `TRAJECTORY_DURATION` | `400` ms |
| `TRAJECTORY_PARTICLES` | `6` 개 |
| `TRAJECTORY_ARC` | `0.35` (이동 거리 대비 포물선 높이) |

#### 궤적 파편 렌더링

- 6개 파편이 각각 `60ms` 시차 (stagger: `i * 0.06`)를 두고 비행
- 궤적: 소스 격자 중심 → 타겟 격자 중심, ease-out (`1 - (1 - t)^2`)
- 포물선 offset: `-4 * 0.35 * dist * eased * (1 - eased)`
- jitter: `(sin(i * 1.7) + cos(i * 2.3)) * 4` px
- 크기: `2 + fade * 3` (fade = `sin(localP * PI)`, 중반 최대)
- alpha: `0.85 * fade`
- radial gradient: `rgba(255,230,140)` → `rgba(255,140,40)` → `rgba(180,40,0,0)`
- compositeOperation: `'lighter'`

#### 착지 팝 (마지막 25%)

- progress > 0.75 구간
- 반경: `3 + landP * 10` (3 → 13px)
- alpha: `(1 - landP) * 0.9` (페이드아웃)
- radial gradient: `rgba(255,220,130)` → `rgba(255,120,40)` → `rgba(180,40,0,0)`

---

## 7. 소방차 오버레이 (FiretruckOverlay)

> 소스: `client/src/features/fire-map/components/FiretruckOverlay.tsx`

### 표시 조건

- 격자의 `stage >= 4` (`FIRETRUCK_STAGE_THRESHOLD = 4`)
- 현재 줌 >= `18` (`FIRETRUCK_MIN_ZOOM = 18`)
- 두 조건 모두 충족 시에만 렌더링

### 위치

격자 오른쪽 하단 모서리:
```
lat = Number(latStr) * LAT_UNIT
lng = Number(lngStr) * LNG_UNIT + LNG_UNIT
```

### Leaflet Marker

- `interactive={false}` (클릭 불가)
- Icon: `L.divIcon`, 단일 인스턴스 `useMemo`로 캐싱

### SVG 상세

| 요소 | 좌표/크기 | 색상 |
|------|-----------|------|
| **차체 (메인)** | `rect x=4 y=10 w=24 h=14 rx=2` | `#cc2222` |
| **차체 (앞부분)** | `rect x=2 y=14 w=6 h=10 rx=1` | `#aa1111` |
| **운전석 유리** | `rect x=22 y=8 w=8 h=6 rx=1` | `fill=#333 stroke=#555 strokeWidth=0.5` |
| **전조등 1** | `rect x=6 y=12 w=4 h=3 rx=0.5` | `#ffdd00` |
| **전조등 2** | `rect x=12 y=12 w=4 h=3 rx=0.5` | `#ffdd00` |
| **왼쪽 바퀴** | `circle cx=8 cy=26 r=3` | `fill=#333 stroke=#555 strokeWidth=0.5` |
| **오른쪽 바퀴** | `circle cx=24 cy=26 r=3` | `fill=#333 stroke=#555 strokeWidth=0.5` |
| **사다리** | `rect x=18 y=12 w=2 h=8 rx=0.5` | `#ccc` |

Icon 크기: `32x32`, anchor: `[16, 28]` (하단 중앙)

### CSS 애니메이션

> 소스: `client/src/index.css`

#### 좌우 흔들림 (truck-wobble)

```css
@keyframes truck-wobble {
  0% { transform: rotate(-3deg); }
  100% { transform: rotate(3deg); }
}
```
- `animation: truck-wobble 0.8s ease-in-out infinite alternate`
- 전체 아이콘 div에 적용

#### 사이렌 경광등 점멸

```css
@keyframes siren {
  0% { color: #ff0000; background: #ff0000; }
  100% { color: #0066ff; background: #0066ff; }
}
```
- `animation: siren 0.4s linear infinite alternate`
- 사이렌 위치: 차체 상단 중앙 (`top: -4px; left: 50%; transform: translateX(-50%)`)
- 크기: `6x6` px, `border-radius: 50%`
- `box-shadow: 0 0 8px currentColor` (색상에 따라 글로우)

---

## 8. 지도 컨트롤 (MapControls)

> 소스: `client/src/features/fire-map/components/MapControls.tsx`

### 위치

`absolute right-4 top-1/2 -translate-y-1/2 z-[1000]` — 화면 오른쪽 중앙, 세로 플렉스 배치 (`flex flex-col gap-2`).

### 버튼 공통 스타일

- 크기: `w-10 h-10`
- 배경: `bg-[var(--color-bg-surface)]`
- 둥근 모서리: `rounded-[12px]`
- 그림자: `shadow-[var(--shadow-heavy)]`
- hover: `bg-[var(--color-bg-elevated)]`
- active: `scale-[0.96]`
- transition: `transition-[transform] duration-150`
- disabled: `opacity-30 cursor-not-allowed active:scale-100`

### 줌 인 버튼

- 텍스트: `+`
- `text-[var(--color-text-base)] text-lg font-bold`
- 클릭: `map.zoomIn()`
- **disabled 조건**: `zoom >= map.getMaxZoom()` (기본값 `18`)

### 줌 아웃 버튼

- 텍스트: `−` (minus sign entity `&minus;`)
- 스타일: 줌 인과 동일
- 클릭: `map.zoomOut()`
- **disabled 조건**: `zoom <= map.getMinZoom()` (기본값 `7`)

### 구분선

`h-px mx-1.5 bg-[var(--color-border)]`

### 내 위치 버튼

- 아이콘: SVG crosshair (`width=18 height=18`, `stroke=var(--color-accent)`, `strokeWidth=2`)
  - `circle cx=12 cy=12 r=4`
  - `path d="M12 2v4M12 18v4M2 12h4M18 12h4"`
- 클릭: `onLocate()` → `map.flyTo([lat, lng], 16, { duration: 1 })`
- disabled 조건: 없음 (위치 확보 시에만 렌더링됨)

### 줌 상태 추적

`useEffect`로 `map.on('zoomend')` 리스너 등록 → `useState`로 현재 줌 레벨 추적.

---

## 9. 하단 패널 (BottomPanel)

> 소스: `client/src/features/fire-map/components/BottomPanel.tsx`

### 위치 및 레이아웃

- `absolute bottom-0 left-0 right-0 z-[1000] p-4 pb-8`
- 카드: `bg-[var(--color-bg-surface)] rounded-[20px] shadow-[var(--shadow-heavy)] p-5`

### 통계 행

두 통계를 가로로 배치 (`flex items-center gap-4 mb-4`), 중간에 구분선 (`w-px h-8 bg-[var(--color-border)]`).

#### 현재 위치 화재 단계 (좌측)

| 요소 | 상세 |
|------|------|
| **데이터 소스** | `currentCell?.stage ?? 0` (현재 위치 grid의 화재 단계) |
| **단계 표시** | `STAGE_LABELS` 매핑: 0=안전, 1=1단계·불씨, 2=2단계·모닥불, 3=3단계·불기둥, 4=4단계·불바다, 5=MAX·불지옥 |
| **아이콘** | SVG 불꽃, `fill={stageInfo.color}`, `14x14` |
| **아이콘 배경** | `color-mix(in srgb, {stageInfo.color} 15%, transparent)` |
| **단계명 폰트** | `text-[0.8125rem] font-bold`, 색상은 단계별 동적 |
| **라벨** | `현재 위치 화재 단계`, `text-[0.6875rem] text-[var(--color-text-secondary)]` |

##### 단계 변경 이펙트 (Shake + Glow)

`useRef`로 이전 단계를 추적하고, 단계가 변경되면 (`stage > 0`일 때만) 두 가지 애니메이션을 동시 트리거한다:

| 애니메이션 | 대상 | 상세 |
|-----------|------|------|
| **stage-shake** | 화재 단계 섹션 전체 (`flex-1` 컨테이너) | 좌우 미세 흔들림 500ms, 진폭 3px → 1px 감쇠 |
| **stage-glow** | 아이콘 외곽 오버레이 (`absolute inset-[-4px]`) | `box-shadow: 0 0 14px 4px {stageInfo.color}`, opacity 0.9 → 0 fade-out 700ms |

`onAnimationEnd`로 `flashing` 상태를 리셋하여 오버레이를 제거한다. 안전(0단계)으로 변경될 때는 이펙트를 트리거하지 않는다.

#### 실시간 화재 지역 수 (우측)

| 요소 | 상세 |
|------|------|
| **컨테이너** | `<button>` — 클릭 시 `onVisit` 호출 (랜덤 화재 지역으로 flyTo) |
| **데이터 소스** | `fires.size` (Map의 엔트리 수) |
| **아이콘** | SVG 지도 핀, `stroke=var(--color-text-secondary)`, `strokeWidth=2`, `14x14` |
| **아이콘 배경** | `w-8 h-8 bg-[var(--color-bg-elevated)] rounded-[10px]` |
| **숫자 폰트** | `text-[1rem] font-bold text-[var(--color-text-base)] leading-none`, `fontVariantNumeric: 'tabular-nums'` |
| **라벨** | `실시간 화재 지역 →`, `text-[0.6875rem] text-[var(--color-accent)] mt-0.5` |
| **비활성 조건** | `activeGrids === 0` |

### 이벤트 버블링 방지

컨테이너 div에서 `onPointerDown/Up/Move` 이벤트를 `stopPropagation()`하여 Leaflet 지도로의 이벤트 전파를 차단한다. 불 지르기 버튼은 `setPointerCapture`와 `isPrimary` 검증으로 멀티터치를 방지하고, `pointerUp` 시 버튼 영역 내 좌표 검증을 수행한다.

### 불 지르기 버튼

> 5. 불 놓기 섹션의 "BottomPanel: 불 지르기 버튼" 참조

---

## 10. 채팅 플로팅 버튼

> 소스: `client/src/features/fire-map/components/MapPage.tsx`
| **동작** | 클릭 시 `FeedbackModal` 렌더링 |

### 채팅 토글 버튼

| 항목 | 값 |
|------|-----|
| **위치** | `absolute bottom-[180px] right-4 z-[1000]` |
| **가시성 조건** | `!chatOpen` (채팅 패널이 닫혀있을 때) |
| **크기** | `w-12 h-12` |
| **배경** | `bg-[var(--color-bg-surface)]` |
| **모양** | `rounded-full` |
| **그림자** | `shadow-[var(--shadow-heavy)]` |
| **아이콘** | SVG 말풍선 (`width=20 height=20`, `stroke=var(--color-accent)`, `strokeWidth=2`, `strokeLinecap=round`, `strokeLinejoin=round`) |
| **Path** | `M21 15a2 2 0 01-2 2H7l-4 4V5a2 2 0 012-2h14a2 2 0 012 2z` |
| **active** | `active:scale-90` |
| **동작** | `setChatOpen(true)` → `ChatPanel` visible |

### ChatPanel

- `ChatPanel visible={chatOpen} onClose={() => setChatOpen(false)}`
- 글로벌 익명 채팅 (chat:global), 바텀시트 형태
- 서버 메시지 1시간 TTL, 최대 100개 보관

---

## 11. 소켓 이벤트 흐름

> 소스: `client/src/features/fire-map/hooks/useFireSocket.ts`
> `client/src/features/fire-map/hooks/useFire.ts`

### 클라이언트 → 서버 (emit)

| 이벤트 | 페이로드 | 발생 조건 |
|--------|---------|----------|
| `fire` | `{ lat: number, lng: number }` | 사용자가 "불 지르기" 탭 |
| `get_fires` | `{}` | 소켓 연결/재연결 시 초기 동기화 요청 |

### 서버 → 클라이언트 (on)

| 이벤트 | 페이로드 | 처리 |
|--------|---------|------|
| `fire:update` | `FireCell { gridId, activeCount, stage }` | `fireStore.updateFire()` — 개별 격자 상태 업데이트. `activeCount > 0`이면 Map에 set, 아니면 delete. `stage >= 5`이면 `triggerExplosion` |
| `fires:sync` | `FireCell[]` | `fireStore.syncFires()` — 전체 불 목록 교체 (Map 재생성) |
| `users:count` | `{ count: number }` | `fireStore.setOnlineUsers()` — 실시간 접속자 수 갱신 |
| `fire:spread` | `{ path: { from: string, to: string }[], event_id?, timestamp? }` | `animationStore.addTrajectory()` — path 배열의 각 segment별 확산 궤적 애니메이션 등록 |
| `connect` | (없음) | `socket.emit('get_fires', {})` — 재접속 시 자동 재동기화 |

### 데이터 흐름 다이어그램

```
Socket 이벤트
    ↓
useFireSocket (리스너)
    ↓
┌──────────────┐    ┌──────────────────┐
│  fireStore   │    │  animationStore  │
│  (Zustand)   │    │  (Zustand)       │
│              │    │                  │
│ - fires Map  │    │ - matches[]      │
│ - onlineUsers│    │ - explosions[]   │
│              │    │ - trajectories[] │
│              │    │ - explodedGrids  │
└──────┬───────┘    └────────┬─────────┘
       │                     │
       └──────────┬──────────┘
                  ↓
         React 컴포넌트 리렌더
                  ↓
    ┌─────────────┼──────────────┐
    ↓             ↓              ↓
FireCanvas   BottomPanel   FiretruckOverlay
(파티클)     (통계/버튼)   (소방차)
```

### fireStore (Zustand)

> 소스: `client/src/features/fire-map/stores/fireStore.ts`

```typescript
interface FireCell {
  gridId: string
  activeCount: number
  stage: number  // 0~5
}

interface FireState {
  fires: Map<string, FireCell>  // O(1) 조회
  onlineUsers: number
  updateFire: (cell: FireCell) => void
  syncFires: (cells: FireCell[]) => void
  setOnlineUsers: (count: number) => void
}
```

`updateFire` 특수 동작: `cell.stage >= 5` 시 `useAnimationStore.getState().triggerExplosion(cell.gridId)` 호출 (store 간 직접 연동).

### animationStore (Zustand)

> 소스: `client/src/features/fire-map/stores/animationStore.ts`

```typescript
interface AnimationState {
  matches: MatchThrow[]           // 활성 성냥 애니메이션
  explosions: Explosion[]         // 전소 폭발 이펙트
  explodedGrids: Set<string>      // 전소 달성 격자 추적 (중복 방지)
  postExplodeTapCount: number     // 5단계 이후 누적 탭 카운트
  nextRandomExplosionAt: number   // 다음 랜덤 폭발 탭 횟수
  trajectories: SpreadTrajectory[] // 불 확산 궤적
  throwMatch: (gridId: string) => void
  removeMatch: (id: number) => void
  triggerExplosion: (gridId: string) => void
  removeExplosion: (id: number) => void
  addTrajectory: (fromGridId: string, toGridId: string) => void
  removeTrajectory: (id: number) => void
}
```

ID 채번: 모듈 레벨 `matchIdCounter` (모든 애니메이션 객체가 공유하는 단일 카운터).

---

## 12. 사용자 위치 마커

> 소스: `client/src/features/fire-map/components/MapPage.tsx` (34-49행)

### 표시 조건

`lat && lng` — 위치가 확보된 경우에만 렌더링.

### 외형

`L.divIcon`으로 커스텀 마커 생성 (Leaflet Marker `interactive={false}`).

#### 구조

| 요소 | 크기 | 스타일 |
|------|------|--------|
| **외부 원 (pulse)** | `24x24` (부모 전체) | `border-radius: 50%`, `background: rgba(66,133,244,0.2)`, `animation: pulse 2s ease-out infinite` |
| **내부 원 (코어)** | `12x12` | `border-radius: 50%`, `background: #4285f4`, `border: 2.5px solid #fff`, `box-shadow: 0 0 6px rgba(66,133,244,0.6)` |

#### pulse 애니메이션

> 소스: `client/src/index.css`

```css
@keyframes pulse {
  0% { transform: scale(1); opacity: 1; }
  100% { transform: scale(2.5); opacity: 0; }
}
```
- `2s ease-out infinite`
- 외부 반투명 원이 1배 → 2.5배로 확장하며 투명해짐

#### Icon 설정

| 속성 | 값 |
|------|-----|
| `iconSize` | `[24, 24]` |
| `iconAnchor` | `[12, 12]` (정중앙) |
| `className` | `''` (빈 문자열 — Leaflet 기본 스타일 제거) |

### FlyToUser

위치 확보 시 `map.flyTo([lat, lng], 16, { duration: 2 })` — 줌 레벨 16으로 2초에 걸쳐 부드럽게 이동.

### LocateButton (내 위치로 재이동)

`map.flyTo([lat, lng], 16, { duration: 1 })` — 줌 레벨 16으로 1초에 걸쳐 이동.

---

## 소스 파일 참조 목록

| 파일 경로 | 역할 |
|----------|------|
| `client/src/routes/map.tsx` | Route 정의 |
| `client/src/features/fire-map/components/MapPage.tsx` | 메인 페이지 컴포넌트 |
| `client/src/features/fire-map/components/FireCanvas.tsx` | Canvas 파티클 화염 렌더링 |
| `client/src/features/fire-map/components/FireOverlay.tsx` | FireCanvas re-export |
| `client/src/features/fire-map/components/FiretruckOverlay.tsx` | 소방차 오버레이 |
| `client/src/features/fire-map/components/LocationPermissionModal.tsx` | 위치 권한 모달 |
| `client/src/features/fire-map/components/MapControls.tsx` | 줌/위치 버튼 |
| `client/src/features/fire-map/components/BottomPanel.tsx` | 하단 통계 + 불 지르기 버튼 |
| `client/src/features/fire-map/hooks/useFire.ts` | 불 지르기 소켓 emit |
| `client/src/features/fire-map/hooks/useFireSocket.ts` | 소켓 이벤트 구독 |
| `client/src/features/fire-map/hooks/useGeolocation.ts` | 위치 권한 + GPS |
| `client/src/features/fire-map/hooks/useReverseGeocode.ts` | 역지오코딩 |
| `client/src/features/fire-map/stores/animationStore.ts` | 애니메이션 상태 (Zustand) |
| `client/src/features/fire-map/stores/fireStore.ts` | 불 상태 (Zustand) |
| `client/src/features/fire-map/utils/grid.ts` | 격자 ID 변환 유틸 |
| `client/src/lib/config.ts` | 환경 설정 + 격자 상수 |
| `client/src/index.css` | 글로벌 CSS 애니메이션 (pulse, truck-wobble, siren) |
| `client/src/features/feedback/components/FeedbackButton.tsx` | 피드백 플로팅 버튼 |
