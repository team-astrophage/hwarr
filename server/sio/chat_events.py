"""Socket.IO event handlers for the global anonymous chat room.

All connected users share a single room `chat:global`. Messages are broadcast
to everyone in the room and persisted to Redis as a capped list with a 1h TTL.

Client → server events:
- chat:join   — enter the global room, returns current presence count
- chat:send   — send a message (validated, broadcast, persisted)
- chat:leave  — leave the global room (presence rebroadcast)

Server → client events:
- chat:message  — a new message was posted in the room
- chat:presence — the number of users currently in the room changed
"""

from __future__ import annotations

import json
import logging
import time
import uuid
from collections.abc import Awaitable, Callable
from typing import TYPE_CHECKING, Any

if TYPE_CHECKING:
    import socketio
    from redis.asyncio import Redis

    from sio.connection_manager import ConnectionManager

logger = logging.getLogger(__name__)

CHAT_ROOM = "chat:global"
CHAT_REDIS_KEY = "chat:global:messages"
CHAT_TTL_SEC = 3600
CHAT_HISTORY_MAX = 100
MSG_MIN_LEN = 1
MSG_MAX_LEN = 300


def _room_member_count(manager: ConnectionManager, room: str) -> int:
    """Count the number of clients currently tracked as members of a room.

    Uses ConnectionManager's own room tracking (info.rooms) rather than
    python-socketio's internal manager, since we already maintain this
    set symmetrically via enter/leave calls.
    """
    count = 0
    for sid in manager.active_sids:
        info = manager.get(sid)
        if info is not None and room in info.rooms:
            count += 1
    return count


def register_chat_events(
    sio: socketio.AsyncServer,
    manager: ConnectionManager,
    redis_client: Redis,
) -> Callable[[set[str]], Awaitable[None]]:
    """Register chat Socket.IO event handlers.

    Returns:
        A coroutine `rebroadcast_presence_on_disconnect(room_set)` that the
        server's global disconnect handler should await when a client drops,
        so the remaining chat participants see the updated presence count.
    """

    async def _broadcast_presence() -> None:
        count = _room_member_count(manager, CHAT_ROOM)
        await manager.broadcast_to_room(
            "chat:presence", {"count": count}, room=CHAT_ROOM,
        )

    @sio.on("chat:join")
    async def handle_chat_join(sid: str, data: dict[str, Any] | None = None) -> dict[str, Any]:
        """Enter the global chat room and announce presence."""
        await sio.enter_room(sid, CHAT_ROOM)
        # Track in ConnectionManager's room set so disconnect cleans up properly.
        info = manager.get(sid)
        if info is not None:
            info.rooms.add(CHAT_ROOM)

        count = _room_member_count(manager, CHAT_ROOM)
        await manager.broadcast_to_room(
            "chat:presence", {"count": count}, room=CHAT_ROOM,
        )
        logger.info("chat:join sid=%s presence=%d", sid, count)
        return {"status": "ok", "count": count}

    @sio.on("chat:leave")
    async def handle_chat_leave(sid: str, data: dict[str, Any] | None = None) -> dict[str, Any]:
        """Leave the global chat room and announce presence."""
        try:
            await sio.leave_room(sid, CHAT_ROOM)
        except Exception:
            pass
        info = manager.get(sid)
        if info is not None:
            info.rooms.discard(CHAT_ROOM)
        await _broadcast_presence()
        logger.info("chat:leave sid=%s", sid)
        return {"status": "ok"}

    @sio.on("chat:send")
    async def handle_chat_send(sid: str, data: dict[str, Any]) -> dict[str, Any]:
        """Validate, persist, and broadcast a chat message.

        Client payload: {
            text: str,
            user_id: str,          # client-stable anonymous identity
            nickname: str,         # "익명의 여우"
            avatar: str,           # emoji "🦊"
            avatar_bg: str,        # hex color
            name_color: str        # hex color
        }
        """
        if not isinstance(data, dict):
            return {"error": "invalid payload"}

        text = data.get("text")
        if not isinstance(text, str):
            return {"error": "text is required"}
        text = text.strip()
        if len(text) < MSG_MIN_LEN:
            return {"error": "text is empty"}
        if len(text) > MSG_MAX_LEN:
            return {"error": f"text exceeds {MSG_MAX_LEN} chars"}

        user_id = data.get("user_id") or ""
        nickname = data.get("nickname") or "익명"
        avatar = data.get("avatar") or "👤"
        avatar_bg = data.get("avatar_bg") or "#333"
        name_color = data.get("name_color") or "#aaa"

        payload = {
            "id": f"msg-{uuid.uuid4().hex[:12]}",
            "user_id": str(user_id)[:64],
            "nickname": str(nickname)[:32],
            "avatar": str(avatar)[:8],
            "avatar_bg": str(avatar_bg)[:16],
            "name_color": str(name_color)[:16],
            "text": text,
            "timestamp": time.time(),
        }

        # Persist to Redis (capped list + 1h TTL). Failure here should not
        # block broadcasting to currently connected users.
        try:
            await redis_client.lpush(CHAT_REDIS_KEY, json.dumps(payload))
            await redis_client.ltrim(CHAT_REDIS_KEY, 0, CHAT_HISTORY_MAX - 1)
            await redis_client.expire(CHAT_REDIS_KEY, CHAT_TTL_SEC)
        except Exception:
            logger.exception("chat:send failed to persist message to Redis")

        await manager.broadcast_to_room("chat:message", payload, room=CHAT_ROOM)
        logger.info(
            "chat:send sid=%s user=%s len=%d", sid, payload["user_id"], len(text),
        )
        return {"status": "ok", "id": payload["id"]}

    async def rebroadcast_presence_on_disconnect(room_set: set[str]) -> None:
        if CHAT_ROOM in room_set:
            await _broadcast_presence()

    return rebroadcast_presence_on_disconnect
