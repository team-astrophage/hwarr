"""
MELTTOWN Server — FastAPI + python-socketio + Redis

기술 선택 이유:
- FastAPI: async 네이티브, 자동 Swagger, Pydantic 타입 검증
- python-socketio AsyncServer: Socket.IO 프로토콜 호환, ASGI 통합
- redis-py async: 공식 클라이언트, asyncio 네이티브
- ASGI mount: FastAPI와 Socket.IO를 하나의 uvicorn 프로세스로 통합
  (별도 프로세스 불필요, 해커톤에서 운영 부담 최소화)
"""

import socketio
import uvicorn
from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from routes.stats import router as stats_router
from sio.fire_events import register_fire_events
from sio.redis_client import redis_client

# --- FastAPI ---
app = FastAPI(title="MELTTOWN API")

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],  # 개발용. 프로덕션에서는 도메인 제한
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

app.include_router(stats_router, prefix="/api")


@app.on_event("startup")
async def startup():
    await redis_client.initialize()


@app.on_event("shutdown")
async def shutdown():
    await redis_client.close()


# --- Socket.IO ---
# async_mode="asgi" 선택 이유: FastAPI(ASGI)와 같은 이벤트 루프 공유
# cors_allowed_origins="*" 선택 이유: 개발 시 CORS 이슈 방지
sio = socketio.AsyncServer(
    async_mode="asgi",
    cors_allowed_origins="*",
)

register_fire_events(sio)

# Socket.IO를 ASGI app으로 감싸서 FastAPI에 마운트
# socketio_path 선택 이유: 기본 "/socket.io" 경로 유지, 클라이언트 호환
socket_app = socketio.ASGIApp(sio, other_asgi_app=app, socketio_path="/socket.io")

if __name__ == "__main__":
    uvicorn.run(
        "main:socket_app",
        host="0.0.0.0",
        port=8000,
        reload=True,
    )
