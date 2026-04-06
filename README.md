# 화르르 (hwarr)

GPS 기반 전국 실시간 스트레스 해소 웹 서비스. 내 위치에서 불을 지르고, 전국이 타오르는 걸 실시간으로 본다.

## 기술 스택

| 영역 | 기술 |
|------|------|
| 프론트엔드 | React 19, Vite, TanStack Router/Query, Zustand, Tailwind CSS, Leaflet |
| 백엔드 | FastAPI, Python-SocketIO, Redis, Shapely |
| 인프라 | Terraform, Docker |
| 실시간 통신 | Socket.IO |

## 빠른 시작

### 사전 준비

- **Node.js** 18+
- **Python** 3.12+
- **Redis** 7+

### 1. 서버 실행

```bash
cd server
python -m venv .venv
source .venv/bin/activate    # Windows: .venv\Scripts\activate
pip install -r requirements.txt
python main.py
```

서버가 `http://localhost:8000`에서 실행된다.

### 2. 클라이언트 실행

```bash
cd client
npm install
```

> **필수:** 로컬 서버에 연결하려면 `client/.env.local` 파일을 생성해야 한다. 없으면 프로덕션 서버에 연결된다.

```bash
# client/.env.local
VITE_API_URL=http://localhost:8000
VITE_SOCKET_URL=http://localhost:8000
```

```bash
npm run dev
```

브라우저에서 `http://localhost:5173` 접속.

> 상세 가이드: [로컬 개발 환경 구축](docs/how-to/local-dev-setup.md)

## 프로젝트 구조

```
├── client/     # React 19 + Vite 프론트엔드
├── server/     # FastAPI 백엔드
├── infra/      # Terraform (AWS)
├── docs/       # 프로젝트 문서
└── scripts/    # 유틸리티 스크립트
```

## 문서

전체 문서는 [docs/README.md](docs/README.md)에서 확인할 수 있다.

- [아키텍처 개요](docs/explanation/architecture-overview.md)
- [REST API 레퍼런스](docs/reference/api-rest.md)
- [Socket.IO 이벤트 레퍼런스](docs/reference/api-socketio.md)
- [로컬 개발 환경 구축](docs/how-to/local-dev-setup.md)
- [배포](docs/how-to/deploy.md)
- [첫 기여 가이드](docs/tutorial/first-contribution.md)
