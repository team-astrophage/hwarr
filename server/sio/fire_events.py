"""
Socket.IO 불 이벤트 핸들러

이벤트 플로우:
1. 클라이언트 → "fire" 이벤트 (lat, lng)
2. 서버: gridId 계산 → Redis Sorted Set에 ZADD
3. 서버: 해당 격자 활성 카운트 조회 → 단계 계산
4. 서버 → 전체 클라이언트에 "fire:update" 브로드캐스트

브로드캐스트 범위 선택 이유:
- MVP에서는 전체 브로드캐스트 (구현 단순)
- 프로덕션에서는 뷰포트 기반 room 분리로 최적화 필요
"""

import math
import time
import uuid

import socketio

from .redis_client import redis_client

GRID_SIZE = 0.001  # ~100m
TTL_SECONDS = 1800  # 30분


def get_grid_id(lat: float, lng: float) -> str:
    grid_lat = math.floor(lat / GRID_SIZE)
    grid_lng = math.floor(lng / GRID_SIZE)
    return f"{grid_lat}:{grid_lng}"


def get_fire_stage(active_count: int) -> int:
    """개발용 임계값 (프로덕션: 1/6/21/51/100)"""
    if active_count >= 15:
        return 5  # 전소
    if active_count >= 10:
        return 4  # 대형화재
    if active_count >= 6:
        return 3  # 화재
    if active_count >= 3:
        return 2  # 모닥불
    if active_count >= 1:
        return 1  # 불씨
    return 0


connected_clients: set[str] = set()


def register_fire_events(sio: socketio.AsyncServer):
    async def broadcast_user_count():
        """접속자 수를 전체 클라이언트에 브로드캐스트"""
        await sio.emit("users:count", {"count": len(connected_clients)})

    @sio.event
    async def connect(sid, environ):
        connected_clients.add(sid)
        print(f"[Socket.IO] Client connected: {sid} (total: {len(connected_clients)})")
        await broadcast_user_count()

    @sio.event
    async def disconnect(sid):
        connected_clients.discard(sid)
        print(f"[Socket.IO] Client disconnected: {sid} (total: {len(connected_clients)})")
        await broadcast_user_count()

    @sio.event
    async def fire(sid, data):
        """
        클라이언트가 불을 지르는 이벤트

        data: { "lat": float, "lng": float }
        """
        lat = data.get("lat")
        lng = data.get("lng")

        if lat is None or lng is None:
            return

        r = redis_client.get()
        grid_id = get_grid_id(lat, lng)
        now = time.time()
        expire_at = now + TTL_SECONDS
        event_id = str(uuid.uuid4())

        # Redis Sorted Set에 불 이벤트 추가
        # key: fire:{gridId}, score: 만료 시각, value: eventId
        key = f"fire:{grid_id}"
        await r.zadd(key, {event_id: expire_at})

        # 만료된 이벤트 정리
        await r.zremrangebyscore(key, 0, now)

        # 활성 카운트 조회
        active_count = await r.zcount(key, now, "+inf")
        stage = get_fire_stage(active_count)

        # 전체 브로드캐스트
        await sio.emit(
            "fire:update",
            {
                "gridId": grid_id,
                "activeCount": active_count,
                "stage": stage,
                "lat": lat,
                "lng": lng,
            },
        )

    @sio.event
    async def get_fires(sid, data):
        """
        클라이언트가 현재 활성 불 목록을 요청

        뷰포트 내 격자만 반환하는 것이 이상적이지만,
        MVP에서는 전체 활성 격자를 반환한다.
        """
        r = redis_client.get()
        now = time.time()

        # fire:* 키 패턴으로 모든 활성 격자 조회
        fires = {}
        async for key in r.scan_iter(match="fire:*"):
            # 만료 정리
            await r.zremrangebyscore(key, 0, now)
            active_count = await r.zcount(key, now, "+inf")
            if active_count > 0:
                grid_id = key.replace("fire:", "")
                fires[grid_id] = {
                    "gridId": grid_id,
                    "activeCount": active_count,
                    "stage": get_fire_stage(active_count),
                }

        await sio.emit("fires:sync", list(fires.values()), to=sid)
