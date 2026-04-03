"""
REST API — 랜딩 페이지 통계

선택 이유:
- 랜딩 페이지는 WebSocket 불필요 (실시간 갱신 불필요, 페이지 로드 시 1회 fetch)
- REST로 제공하면 소켓 연결 전에 가볍게 데이터 로드 가능
"""

import time

from fastapi import APIRouter

from sio.redis_client import redis_client
from sio.fire_events import get_fire_stage, connected_clients

router = APIRouter()


@router.get("/stats")
async def get_stats():
    """전국 화재 통계 — 활성 격자 수, 총 활성 이벤트 수"""
    r = redis_client.get()
    now = time.time()

    active_grids = 0
    total_fires = 0

    async for key in r.scan_iter(match="fire:*"):
        await r.zremrangebyscore(key, 0, now)
        count = await r.zcount(key, now, "+inf")
        if count > 0:
            active_grids += 1
            total_fires += count

    return {
        "activeGrids": active_grids,
        "totalFires": total_fires,
        "onlineUsers": len(connected_clients),
    }
