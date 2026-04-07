# 로컬 개발 환경 설정 가이드

화르르(hwarr) 프로젝트를 로컬에서 실행하기 위한 단계별 가이드입니다.

로컬 개발 환경을 구성하는 방법은 두 가지입니다:

- **[Docker Compose](#docker-compose로-실행-추천)** — 명령어 하나로 전체 환경 구성. 별도 설치 불필요.
- **[직접 설치](#직접-설치)** — Node.js, Python, Redis를 직접 설치하여 실행.

---

## Docker Compose로 실행 (추천)

### 사전 준비

[Docker Desktop](https://www.docker.com/products/docker-desktop/)을 설치한다. Docker Compose는 Docker Desktop에 포함되어 있다.

```bash
docker --version          # Docker 확인
docker compose version    # Compose 확인
```

### 1단계: 전체 서비스 실행

프로젝트 루트에서 아래 명령을 실행한다.

```bash
docker compose up --build
```

최초 실행 시 이미지 빌드에 약 30초가 소요된다. 이후에는 캐시를 사용하므로 빠르게 시작된다.

실행 완료 시 3개 컨테이너가 올라간다:

| 컨테이너 | 포트 | 역할 |
|----------|------|------|
| `hwarr-redis-1` | `6379` | Redis 데이터 저장소 |
| `hwarr-backend-1` | `8000` | FastAPI 서버 |
| `hwarr-frontend-1` | `5173` | Vite 개발 서버 |

브라우저에서 `http://localhost:5173`으로 접속한다.

### 2단계: 백그라운드 실행

터미널을 점유하고 싶지 않다면 `-d` 플래그를 사용한다.

```bash
docker compose up -d
```

로그를 확인하고 싶을 때:

```bash
docker compose logs -f           # 전체 로그
docker compose logs -f backend   # 백엔드만
```

### 3단계: 종료

```bash
docker compose down
```

Redis 데이터는 `hwarr_redis-data` 볼륨에 보존된다. 데이터까지 완전히 삭제하려면:

```bash
docker compose down -v
```

### Docker 개발 환경 특징

- **핫 리로드**: 소스 코드가 볼륨 마운트되어 있어 파일 수정 시 서버/클라이언트가 자동 재시작된다.
- **환경변수 자동 설정**: `VITE_API_URL`, `REDIS_URL` 등이 `docker-compose.yml`에 미리 설정되어 있다. `.env.local` 파일을 만들 필요 없다.
- **GeoJSON 자동 다운로드**: 백엔드 컨테이너 시작 시 행정동 GeoJSON 파일이 없으면 자동으로 다운로드한다.
- **Redis 헬스체크**: Redis가 준비된 후에 백엔드가 시작되도록 `depends_on` + `healthcheck`가 설정되어 있다.

### 자주 쓰는 Docker 명령어

```bash
# 이미지 재빌드 (의존성 변경 시)
docker compose up --build

# 특정 서비스만 재시작
docker compose restart backend

# 컨테이너 안에서 명령 실행
docker compose exec backend bash
docker compose exec redis redis-cli

# 실행 상태 확인
docker compose ps
```

---

## 직접 설치

Docker를 사용하지 않고 각 도구를 직접 설치하여 실행하는 방법이다.

### 1. 필요 도구

| 도구 | 최소 버전 | 설치 (macOS) |
|------|----------|-------------|
| Node.js | 18+ | `brew install node` |
| Python | 3.12+ | `brew install python` |
| Redis | 7+ | `brew install redis` |

Windows 사용자는 [Redis for Windows](https://github.com/microsoftarchive/redis/releases) 또는 WSL2 환경을 권장합니다.

버전 확인:

```bash
node -v        # v18 이상
python3 -V     # 3.12 이상
redis-server -v # 7 이상
```

---

## 2. 서버 세팅

### 2-1. Redis 시작

별도 터미널에서 실행하거나 백그라운드로 띄웁니다.

```bash
# 포그라운드 실행
redis-server

# 또는 백그라운드 실행
redis-server --daemonize yes
```

### 2-2. Python 가상환경 및 의존성 설치

```bash
cd server
python3 -m venv .venv
source .venv/bin/activate    # Windows: .venv\Scripts\activate
pip install -r requirements.txt
```

주요 패키지: `fastapi`, `uvicorn[standard]`, `python-socketio`, `redis`, `shapely`, `httpx`

### 2-3. 행정동 GeoJSON 다운로드 (일일 랭킹용, 선택)

서버는 시작 시 `data/admin_dong.geojson` 파일을 찾습니다. 없으면 랭킹 기능 없이 정상 동작하지만, 필요하다면 수동으로 받아둡니다.

```bash
mkdir -p data
curl -fsSL -o data/admin_dong.geojson \
  "https://raw.githubusercontent.com/vuski/admdongkor/master/ver20230701/HangJeongDong_ver20230701.geojson"
```

### 2-4. 서버 실행

```bash
python main.py
```

서버가 `http://localhost:8000`에서 실행됩니다. `--reload` 옵션이 기본 활성화되어 코드 변경 시 자동 재시작됩니다.

동작 확인:

```bash
curl http://localhost:8000/health
curl http://localhost:8000/api/stats
```

### 2-5. 환경변수 (선택)

서버는 아래 환경변수를 지원합니다. 설정하지 않으면 기본값을 사용합니다.

| 환경변수 | 기본값 | 설명 |
|---------|--------|------|
| `REDIS_URL` | `redis://localhost:6379/0` | Redis 접속 URL |
| `HOST` | `0.0.0.0` | 서버 바인드 주소 |
| `PORT` | `8000` | 서버 포트 |
| `FIRE_TTL_SEC` | `43200` (12시간) | 불 지속 시간(초) |
| `ADMIN_GEOJSON_PATH` | `data/admin_dong.geojson` | 행정동 GeoJSON 경로 |
| `FEEDBACK_DISCORD_WEBHOOK_URL` | (빈 문자열) | 피드백 전송용 Discord webhook URL |

---

## 3. 클라이언트 세팅

### 3-1. 의존성 설치

```bash
cd client
npm install
```

### 3-2. 환경변수 설정

클라이언트는 기본적으로 프로덕션 URL(`https://hwarr.com`)을 바라봅니다. 로컬 서버에 연결하려면 `.env.local` 파일을 만들어야 합니다.

```bash
# client/.env.local
VITE_API_URL=http://localhost:8000
VITE_SOCKET_URL=http://localhost:8000
```

> `.env.local` 파일이 없으면 로컬 서버가 아닌 프로덕션 서버에 연결됩니다. 반드시 설정하세요.

### 3-3. 개발 서버 실행

```bash
npm run dev
```

Vite 개발 서버가 기본적으로 `http://localhost:5173`에서 실행됩니다. 포트를 지정하고 싶다면:

```bash
npm run dev -- --port 3000
```

브라우저에서 해당 주소로 접속하면 됩니다.

---

## 4. GPS 테스트 방법

데스크탑 환경에서는 GPS가 동작하지 않거나 권한이 차단될 수 있습니다. 아래 방법으로 위치를 가짜로 주입할 수 있습니다.

### 방법 1: 브라우저 콘솔 백도어

브라우저 개발자 도구(F12) 콘솔에서 아래 코드를 입력한 뒤 페이지를 새로고침합니다.

```js
// 서울 시청 좌표
window.__MELTTOWN_GPS = { lat: 37.5665, lng: 126.9780 };
```

다른 지역 예시:

```js
// 부산역
window.__MELTTOWN_GPS = { lat: 35.1152, lng: 129.0422 };

// 제주 시청
window.__MELTTOWN_GPS = { lat: 33.4996, lng: 126.5312 };
```

### 방법 2: Chrome DevTools Sensors 패널

1. Chrome DevTools를 엽니다 (F12 또는 Cmd+Option+I).
2. 우측 상단 점 세 개 메뉴 > **More tools** > **Sensors**를 선택합니다.
3. **Location** 드롭다운에서 미리 정의된 도시를 선택하거나, **Other**를 선택해 위도/경도를 직접 입력합니다.
4. 이 상태에서 페이지를 새로고침하면 `navigator.geolocation`이 해당 좌표를 반환합니다.

> Sensors 패널의 Location 설정은 `navigator.geolocation` API를 덮어쓰므로, 백도어 방식보다 더 정확하게 실제 GPS 동작을 시뮬레이션합니다.

### 방법 3: Demo API

서버에 demo 라우터(`/api/demo`)가 등록되어 있습니다. 테스트 데이터를 일괄 생성할 때 유용합니다.

---

## 5. Redis CLI 유용한 명령어

```bash
# Redis CLI 접속
redis-cli

# 현재 저장된 모든 키 조회
KEYS *

# 활성 불 데이터 확인 (Sorted Set)
ZCARD fires
ZRANGE fires 0 -1 WITHSCORES

# 오늘 날짜 기준 통계 확인
GET stats:total_fires
GET stats:daily_fires:2026-04-06

# 일일 랭킹 확인
ZREVRANGE stats:daily_ranking:2026-04-06 0 9 WITHSCORES

# 특정 키의 TTL 확인
TTL stats:daily_fires:2026-04-06

# 모든 데이터 초기화 (개발 환경에서만!)
FLUSHDB

# 실시간 명령어 모니터링 (디버깅용)
MONITOR
```

> `MONITOR` 명령은 Redis에 들어오는 모든 명령을 실시간으로 출력합니다. Socket.IO 이벤트가 Redis에 어떤 명령을 보내는지 확인할 때 유용합니다. 부하가 있으므로 디버깅 후 종료하세요.

---

## 6. 자주 하는 실수와 해결법

### "Redis connection refused" 에러

**원인**: Redis 서버가 실행되지 않은 상태에서 서버를 시작했습니다.

**해결**: 서버 시작 전에 Redis를 먼저 실행하세요.

```bash
redis-server --daemonize yes
redis-cli ping  # PONG이 나오면 정상
```

서버는 Redis 연결 실패 시에도 시작되지만, 불 기능(FireProgressionEngine)이 비활성화됩니다.

---

### 클라이언트에서 서버에 연결이 안 될 때

**원인**: `client/.env.local` 파일이 없거나 잘못 설정되어 프로덕션 URL로 요청이 갑니다.

**해결**: `client/.env.local`에 아래 내용이 있는지 확인하세요.

```
VITE_API_URL=http://localhost:8000
VITE_SOCKET_URL=http://localhost:8000
```

> `.env.local` 파일을 수정한 뒤에는 Vite 개발 서버를 재시작해야 반영됩니다.

---

### CORS 에러

**원인**: 보통 클라이언트와 서버의 포트가 다를 때 발생합니다.

**해결**: 서버의 CORS 설정은 기본적으로 `allow_origins=["*"]`로 열려 있습니다. CORS 에러가 발생한다면 서버가 실제로 실행 중인지, 주소가 맞는지 확인하세요.

---

### `ModuleNotFoundError` (Python)

**원인**: 가상환경이 활성화되지 않았거나, `pip install`을 실행하지 않았습니다.

**해결**:

```bash
cd server
source .venv/bin/activate
pip install -r requirements.txt
```

---

### Socket.IO 연결이 끊겼다가 다시 붙을 때

**정상 동작입니다.** 서버는 `ping_interval=10`, `ping_timeout=5`로 설정되어 있어, 15초 이상 응답이 없으면 연결을 끊습니다. 클라이언트는 자동으로 재연결을 시도합니다.

---

### 행정동 GeoJSON 파일이 없다는 경고

**원인**: `data/admin_dong.geojson` 파일이 없습니다.

**해결**: 2-3 단계의 `curl` 명령으로 파일을 다운로드하세요. 이 파일이 없어도 서버는 정상 동작하며, 일일 지역 랭킹 기능만 비활성화됩니다.

---

### 포트 충돌 ("Address already in use")

**원인**: 이전에 실행한 서버 프로세스가 아직 살아있습니다.

**해결**:

```bash
# 8000번 포트를 사용 중인 프로세스 찾기
lsof -i :8000

# 해당 프로세스 종료
kill -9 <PID>
```

또는 `PORT` 환경변수로 다른 포트를 지정합니다.

```bash
PORT=8001 python main.py
```
