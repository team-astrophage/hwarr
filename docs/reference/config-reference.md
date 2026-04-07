# 설정 레퍼런스

화르르(hwarr) 프로젝트의 모든 설정값을 한곳에 정리한 문서이다.
서버(`server/internal/config/config.go`)와 클라이언트(`client/src/lib/config.ts`)에 흩어진 설정을 통합하여 기술한다.

---

## 서버

### 환경변수

| 변수명 | 타입 | 기본값 | 설명 |
|--------|------|--------|------|
| `REDIS_URL` | `string` | `redis://localhost:6379/0` | Redis 연결 URL |
| `HOST` | `str` | `0.0.0.0` | 서버 바인드 주소 |
| `PORT` | `int` | `8000` | 서버 리스닝 포트 |
| `FIRE_TTL_SEC` | `int` | `43200` (12시간) | 불(fire) 데이터의 Redis TTL (초) |
| `NEWS_TTL_SEC` | `int` | `86400` (1일) | 뉴스 데이터의 Redis TTL (초) |
| `ADMIN_GEOJSON_PATH` | `str` | `server/data/admin_dong.geojson` | 행정동 GeoJSON 파일 경로. 일별 랭킹에 사용 |
| `FEEDBACK_DISCORD_WEBHOOK_URL` | `str` | `""` (빈 문자열) | 피드백 전송용 Discord Webhook URL. 미설정 시 피드백 전송 비활성화 |
| `FEEDBACK_RATE_LIMIT_PER_10MIN` | `int` | `3` | 10분당 피드백 제출 횟수 제한 |
| `FRONTEND_URL` | `str` | `https://bulpan.example.com` | QR 코드 생성 시 사용하는 프론트엔드 URL |

### 상수 (config.go / constants.go)

| 상수명 | 타입 | 값 | 설명 |
|--------|------|-----|------|
| `KST` | `*time.Location` | `Asia/Seoul` | 한국 표준시 timezone |
| `STATS_TOTAL_FIRES_KEY` | `str` | `stats:total_fires` | 누적 불 횟수를 저장하는 Redis key |
| `STATS_DAILY_FIRES_PREFIX` | `str` | `stats:daily_fires:` | 일별 불 횟수 Redis key prefix |
| `STATS_DAILY_FIRES_TTL_SEC` | `int` | `172800` (48시간) | 일별 불 횟수 key의 TTL. KST 자정 경계 여유분 포함 |
| `STATS_DAILY_RANKING_PREFIX` | `str` | `stats:daily_ranking:` | 일별 행정동 랭킹 Redis key prefix |
| `STATS_DAILY_RANKING_TTL_SEC` | `int` | `172800` (48시간) | 일별 랭킹 key의 TTL |

### Dockerfile

| 항목 | 값 | 설명 |
|------|-----|------|
| Base image | `golang:1.25-alpine` (build) / `alpine:3.20` (runtime) | Multi-stage build |
| `EXPOSE` | `8000` | 컨테이너 노출 포트 |
| `CMD` | `./hwarr-server` | Go 바이너리 실행 |
| Build arg `GEOJSON_URL` | GitHub raw URL (ver20230701) | 행정동 GeoJSON 다운로드 URL. 빌드 시 override 가능 |

---

## 클라이언트

### 환경변수

| 변수명 | 타입 | 기본값 | 설명 |
|--------|------|--------|------|
| `VITE_API_URL` | `string` | `https://hwarr.com` | API 서버 base URL |
| `VITE_SOCKET_URL` | `string` | `https://hwarr.com` | Socket.IO 서버 URL |

### 상수 (config.ts)

| 상수명 | 타입 | 값 | 설명 |
|--------|------|-----|------|
| `API_URL` | `string` | `VITE_API_URL` fallback `https://hwarr.com` | API 요청 base URL |
| `SOCKET_URL` | `string` | `VITE_SOCKET_URL` fallback `https://hwarr.com` | Socket.IO 연결 URL |
| `LAT_UNIT` | `number` | `0.0009` | 격자 위도 단위 (~100m) |
| `LNG_UNIT` | `number` | `0.0011` | 격자 경도 단위 (~100m, 한국 기준 ~37 N) |
| `TTL_SECONDS` | `number` | `1800` (30분) | 클라이언트 측 불 TTL |

### Vite 설정 (vite.config.ts)

| 항목 | 값 | 설명 |
|------|-----|------|
| Path alias `@` | `./src` | `@/` import alias |
| Routes directory | `./src/routes` | TanStack Router 파일 기반 라우팅 경로 |
| Generated route tree | `./src/routeTree.gen.ts` | 자동 생성 라우트 트리 파일 |
| Plugins | `TanStackRouterVite`, `react`, `tailwindcss` | Vite plugin 목록 |

---

## TTL 불일치 경고

> **서버 `FIRE_TTL_SEC` = 43200 (12시간) vs 클라이언트 `TTL_SECONDS` = 1800 (30분)**

서버는 Redis에서 불 데이터를 12시간 유지하지만, 클라이언트는 30분을 기준으로 불의 잔여 수명을 계산한다.
클라이언트는 주기적으로 서버와 동기화(`fires:sync`, `subscribe:viewport`)하므로 실질적인 문제는 제한적이나, 클라이언트 자체 TTL 만료 시점과 서버의 실제 만료 시점에 차이가 있을 수 있다.

- 클라이언트가 새로고침하면 서버에 남아있는 불이 다시 표시됨
- 서버 동기화를 통해 실시간 상태는 항상 정확하게 반영됨
