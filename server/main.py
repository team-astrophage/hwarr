"""전국불판 — FastAPI + Socket.IO application entry point.

Creates the FastAPI app and mounts the Socket.IO ASGI app.
Socket.IO event handlers for connect/disconnect are registered here.
Fire event handlers are registered via sio.events module.
"""

from __future__ import annotations

import logging
import os
import time

import socketio
from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from routes.demo import router as demo_router
from routes.fire import router as fire_router
from routes.map_config import router as map_config_router
from routes.news import router as news_router
from routes.qr import router as qr_router
from routes.ranking import router as ranking_router
from routes.stats import router as stats_router
from sio.chat_events import CHAT_ROOM, register_chat_events
from sio.connection_manager import (
    ConnectionManager,
    HEARTBEAT_INTERVAL_SEC,
)
from sio.events import register_fire_events

# ---------------------------------------------------------------------------
# Logging
# ---------------------------------------------------------------------------
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
)
logger = logging.getLogger(__name__)

# ---------------------------------------------------------------------------
# Socket.IO server
# ---------------------------------------------------------------------------
sio = socketio.AsyncServer(
    async_mode="asgi",
    cors_allowed_origins="*",  # hackathon — open CORS
    logger=False,
    engineio_logger=False,
    # Tight ping intervals for fast disconnect detection (1-2s delivery target)
    ping_interval=10,  # send ping every 10s
    ping_timeout=5,  # client must respond within 5s
)

# ---------------------------------------------------------------------------
# Connection manager (singleton)
# ---------------------------------------------------------------------------
manager = ConnectionManager(sio)

# ---------------------------------------------------------------------------
# Fire progression engine (lazy-initialized on startup)
# ---------------------------------------------------------------------------
# The engine is created at startup when Redis is available.
# For tests, it can be None until explicitly set.
engine = None

# ---------------------------------------------------------------------------
# FastAPI app
# ---------------------------------------------------------------------------
app = FastAPI(
    title="전국불판 API",
    description="Real-time fire map hackathon backend",
    version="0.1.0",
)

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# ---------------------------------------------------------------------------
# REST API routers
# ---------------------------------------------------------------------------
app.include_router(fire_router)
app.include_router(news_router)
app.include_router(qr_router)
app.include_router(map_config_router)
app.include_router(demo_router)
app.include_router(stats_router)
app.include_router(ranking_router)


@app.get("/health")
async def health_check():
    """Health check endpoint for ALB."""
    return {
        "status": "ok",
        "connections": manager.active_count,
        "engine_running": engine.is_running if engine else False,
    }


@app.on_event("startup")
async def startup_event():
    """Initialize Redis connection and fire progression engine on startup."""
    global engine

    redis_url = os.getenv("REDIS_URL", "redis://localhost:6379/0")

    try:
        import redis.asyncio as aioredis

        redis_client = aioredis.from_url(
            redis_url,
            decode_responses=False,
        )
        # Test connection
        await redis_client.ping()
        logger.info("Connected to Redis: %s", redis_url)

        from jobs.fire_progression import FireProgressionEngine

        engine = FireProgressionEngine(
            redis=redis_client,
            broadcaster=manager,
            scan_interval=2.0,
            cleanup_interval=60.0,
        )

        # Register fire event handlers with the engine
        register_fire_events(sio, manager, engine)

        # Register global chat event handlers (uses the same Redis client)
        register_chat_events(sio, manager, redis_client)

        # Load admin region resolver for daily ranking (optional)
        from pathlib import Path as _Path

        from config import ADMIN_GEOJSON_PATH
        from services.admin_region import AdminRegionResolver

        resolver = AdminRegionResolver.load_or_none(_Path(ADMIN_GEOJSON_PATH))
        engine.set_region_resolver(resolver)

        # Start background tasks
        engine.start()
        logger.info("FireProgressionEngine started")

        # Start stale connection reaper
        manager.start_reaper()

    except Exception:
        logger.exception(
            "Failed to connect to Redis at %s — running without fire engine",
            redis_url,
        )


@app.on_event("shutdown")
async def shutdown_event():
    """Gracefully stop the fire progression engine and reaper."""
    await manager.stop_reaper()
    if engine and engine.is_running:
        await engine.stop()
        logger.info("FireProgressionEngine stopped")


# ---------------------------------------------------------------------------
# Socket.IO event handlers
# ---------------------------------------------------------------------------

@sio.event
async def connect(sid: str, environ: dict, auth: dict | None = None):
    """Handle new WebSocket connection.

    Registers the client with ConnectionManager and sends a welcome ack.
    Supports reconnection via auth.user_id — restores previous room subs.
    """
    logger.info("Client connecting: %s", sid)

    # Extract user_id from auth for reconnection support
    user_id = None
    if auth and isinstance(auth, dict):
        user_id = auth.get("user_id")

    # Check for previous session to restore rooms on reconnect
    previous_rooms: set[str] = set()
    if user_id:
        prev_session = manager.get_previous_session(user_id)
        if prev_session:
            previous_rooms = prev_session.rooms.copy()

    info = await manager.add(sid, user_id=user_id)

    # Restore room subscriptions on reconnect
    if previous_rooms:
        await manager.restore_rooms(sid, previous_rooms)
        logger.info(
            "Restored %d rooms for reconnected user=%s sid=%s",
            len(previous_rooms), user_id, sid,
        )

    # Send connection acknowledgement to the client
    await sio.emit("connected", {
        "sid": sid,
        "connected_at": info.connected_at,
        "active_connections": manager.active_count,
        "heartbeat_interval": HEARTBEAT_INTERVAL_SEC,
        "reconnect_count": info.reconnect_count,
        "restored_rooms": len(previous_rooms),
    }, to=sid)
    # Broadcast users:count to all clients (mock server compat)
    await sio.emit("users:count", {"count": manager.active_count})

    logger.info(
        "Client connected: %s (total: %d)", sid, manager.active_count
    )


@sio.event
async def disconnect(sid: str):
    """Handle WebSocket disconnection.

    Removes the client from ConnectionManager, cleans up rooms.
    Preserves session info in _user_sessions for potential reconnection.
    """
    logger.info("Client disconnecting: %s", sid)
    info = manager.get(sid)
    was_in_chat = info is not None and CHAT_ROOM in info.rooms
    await manager.remove(sid)

    # Broadcast users:count to all clients (mock server compat)
    await sio.emit("users:count", {"count": manager.active_count})

    # If the disconnecting client was in the global chat room, rebroadcast
    # the updated presence count to remaining participants.
    if was_in_chat:
        try:
            participants = sio.manager.get_participants("/", CHAT_ROOM)
            count = sum(1 for _ in participants)
            await sio.emit("chat:presence", {"count": count}, room=CHAT_ROOM)
        except Exception:
            logger.exception("Failed to rebroadcast chat presence on disconnect")

    logger.info(
        "Client disconnected: %s (total: %d)", sid, manager.active_count
    )


@sio.on("heartbeat")
async def handle_heartbeat(sid: str, data: dict | None = None):
    """Handle client heartbeat ping.

    Client sends: { "ts": <client_timestamp> } (optional)
    Server responds with heartbeat:ack containing server timestamp
    and round-trip info for latency monitoring.
    """
    recorded = manager.record_heartbeat(sid)
    if not recorded:
        logger.warning("Heartbeat from unknown sid: %s", sid)
        return {"error": "unknown_sid"}

    server_ts = time.time()
    return {
        "status": "ok",
        "server_ts": server_ts,
        "client_ts": data.get("ts") if data else None,
    }


# ---------------------------------------------------------------------------
# Mount Socket.IO onto FastAPI as ASGI sub-app
# ---------------------------------------------------------------------------
sio_asgi_app = socketio.ASGIApp(sio, other_asgi_app=app)

# The combined ASGI app: Socket.IO handles /socket.io/*, FastAPI handles rest
combined_app = sio_asgi_app


if __name__ == "__main__":
    import uvicorn

    host = os.getenv("HOST", "0.0.0.0")
    port = int(os.getenv("PORT", "8000"))
    uvicorn.run(
        "main:combined_app",
        host=host,
        port=port,
        reload=True,
        log_level="info",
    )
