"""Socket.IO event handlers for global anonymous chat.

Single global room (`chat:global`) where all connected clients exchange
ephemeral messages. Messages are persisted to Redis with a 1-hour TTL and
capped at 100 entries.
"""

from __future__ import annotations

import json
import logging
import time
import uuid
from typing import TYPE_CHECKING, Any

if TYPE_CHECKING:
    import socketio

    from sio.connection_manager import ConnectionManager

logger = logging.getLogger(__name__)

CHAT_ROOM = "chat:global"
CHAT_REDIS_KEY = "chat:global:messages"
CHAT_TTL_SEC = 3600  # 1 hour
CHAT_MAX_MESSAGES = 100
CHAT_HISTORY_SIZE = 50
CHAT_TEXT_MIN = 1
CHAT_TEXT_MAX = 300


def register_chat_events(
    sio: socketio.AsyncServer,
    manager: ConnectionManager,
    redis_client: Any,
) -> None:
    """Register all chat-related Socket.IO event handlers.

    Args:
        sio: The Socket.IO async server instance.
        manager: ConnectionManager for tracking connections and rooms.
        redis_client: async Redis client for message persistence.
    """

    async def _room_count() -> int:
        """Return the number of clients currently in the chat room."""
        try:
            # python-socketio manager.get_participants is a sync iterator
            participants = sio.manager.get_participants("/", CHAT_ROOM)
            return sum(1 for _ in participants)
        except Exception:
            return 0

    async def _broadcast_presence() -> None:
        count = await _room_count()
        await sio.emit("chat:presence", {"count": count}, room=CHAT_ROOM)

    @sio.on("chat:join")
    async def handle_chat_join(sid: str, data: dict[str, Any] | None = None) -> dict[str, Any]:
        """Enter the global chat room; return recent history + presence."""
        info = manager.get(sid)
        if info is None:
            return {"error": "unknown_sid"}

        await sio.enter_room(sid, CHAT_ROOM)
        info.rooms.add(CHAT_ROOM)

        # Load recent history from Redis. Stored newest-first via LPUSH,
        # so LRANGE 0 49 returns newest→oldest. Reverse for chronological.
        history: list[dict[str, Any]] = []
        try:
            raw = await redis_client.lrange(CHAT_REDIS_KEY, 0, CHAT_HISTORY_SIZE - 1)
            for item in reversed(raw):
                try:
                    history.append(json.loads(item))
                except (json.JSONDecodeError, TypeError):
                    continue
        except Exception:
            logger.exception("Failed to load chat history from Redis")

        count = await _room_count()
        # Broadcast updated presence to everyone in the room
        await sio.emit("chat:presence", {"count": count}, room=CHAT_ROOM)

        logger.info("chat:join sid=%s presence=%d history=%d", sid, count, len(history))
        return {"status": "ok", "history": history, "presence": count}

    @sio.on("chat:send")
    async def handle_chat_send(sid: str, data: dict[str, Any]) -> dict[str, Any]:
        """Validate, persist, and broadcast a chat message."""
        if not isinstance(data, dict):
            return {"error": "invalid_payload"}

        text = (data.get("text") or "").strip()
        if len(text) < CHAT_TEXT_MIN:
            return {"error": "text_too_short"}
        if len(text) > CHAT_TEXT_MAX:
            return {"error": "text_too_long"}

        user_id = data.get("user_id")
        if not user_id or not isinstance(user_id, str):
            return {"error": "user_id_required"}

        msg = {
            "id": f"msg-{uuid.uuid4().hex[:12]}",
            "user_id": user_id,
            "nickname": str(data.get("nickname") or "익명"),
            "avatar": str(data.get("avatar") or "👤"),
            "avatar_bg": str(data.get("avatar_bg") or "#333"),
            "name_color": str(data.get("name_color") or "#fff"),
            "text": text,
            "timestamp": time.time(),
        }

        # Persist to Redis: newest-first list, cap at CHAT_MAX_MESSAGES, reset TTL.
        try:
            serialized = json.dumps(msg, ensure_ascii=False)
            pipe = redis_client.pipeline()
            pipe.lpush(CHAT_REDIS_KEY, serialized)
            pipe.ltrim(CHAT_REDIS_KEY, 0, CHAT_MAX_MESSAGES - 1)
            pipe.expire(CHAT_REDIS_KEY, CHAT_TTL_SEC)
            await pipe.execute()
        except Exception:
            logger.exception("Failed to persist chat message to Redis")

        await sio.emit("chat:message", msg, room=CHAT_ROOM)
        logger.info("chat:send sid=%s user=%s len=%d", sid, user_id, len(text))
        return {"status": "ok", "id": msg["id"]}

    @sio.on("chat:leave")
    async def handle_chat_leave(sid: str, data: dict[str, Any] | None = None) -> dict[str, Any]:
        """Leave the chat room and rebroadcast presence."""
        info = manager.get(sid)
        try:
            await sio.leave_room(sid, CHAT_ROOM)
        except Exception:
            pass
        if info is not None:
            info.rooms.discard(CHAT_ROOM)

        await _broadcast_presence()
        logger.info("chat:leave sid=%s", sid)
        return {"status": "ok"}

    logger.info("Chat events registered (room=%s)", CHAT_ROOM)
