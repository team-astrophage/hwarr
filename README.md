# MELTTOWN

GPS 기반 전국 실시간 스트레스 해소 웹 서비스. 내 위치에서 불을 지르고, 전국이 타오르는 걸 실시간으로 본다.

## 로컬 실행 가이드

### 사전 준비

- **Node.js** 18+
- **Python** 3.12+
- **Redis** 7+

```bash
# macOS
brew install node python redis
```

### 1. 레포 클론

```bash
git clone https://github.com/kimzeze/CURSOR-Astrophage.git
cd CURSOR-Astrophage/davao
git checkout dev
```

### 2. Redis 실행

```bash
redis-server
```

별도 터미널에서 실행하거나, 백그라운드로:

```bash
redis-server --daemonize yes
```

### 3. 서버 실행

```bash
cd server
python -m venv .venv
source .venv/bin/activate    # Windows: .venv\Scripts\activate
pip install -r requirements.txt
python main.py
```

서버가 `http://localhost:8000` 에서 실행된다.
확인: `curl http://localhost:8000/api/stats`

### 4. 클라이언트 실행

```bash
cd client
npm install
npm run dev -- --port 3000
```

브라우저에서 `http://localhost:3000` 접속.

### 5. GPS 백도어 (데스크탑 테스트용)

데스크탑에서는 GPS 권한이 안 뜰 수 있다. 브라우저 콘솔에서:

```js
// 페이지 이동 전에 실행
window.__MELTTOWN_GPS = { lat: 37.5665, lng: 126.9780 }  // 서울 시청
```

또는 `/map` 접속 전에 콘솔에 입력 후 새로고침.

## 기술 스택

| 영역 | 스택 |
|------|------|
| Frontend | React, TanStack Router, Tailwind CSS, Zustand, Socket.IO Client, Leaflet |
| Backend | FastAPI, python-socketio, Redis (Sorted Set), uvicorn |
| 지도 | CARTO Dark 타일, react-leaflet |

## 프로젝트 구조

```
davao/
├── client/                  # React 프론트엔드
│   └── src/
│       ├── features/
│       │   ├── fire-map/    # 지도 + 불 기능
│       │   └── landing/     # 랜딩 페이지
│       ├── components/      # 공유 컴포넌트
│       ├── lib/             # config, socket
│       └── routes/          # TanStack 파일 라우팅
├── server/                  # FastAPI 백엔드
│   ├── sio/                 # Socket.IO 이벤트, Redis 클라이언트
│   └── routes/              # REST API
└── docs/                    # PRD, 아키텍처 문서
```
