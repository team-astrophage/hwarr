"""ConnectionManager — tracks active Socket.IO connections.

Provides add/remove/broadcast methods for managing WebSocket clients.
Uses python-socketio's room system for viewport-based broadcasting.
Includes heartbeat tracking and stale connection reaping.
"""

from __future__ import annotations

import asyncio
import logging
import time
from dataclasses import dataclass, field
from typing import Any

import socketio

logger = logging.getLogger(__name__)

# Heartbeat configuration
HEARTBEAT_INTERVAL_SEC = 10  # Server expects a heartbeat every 10s
HEARTBEAT_TIMEOUT_SEC = 30  # Client considered stale after 30s without heartbeat
REAPER_INTERVAL_SEC = 15  # How often the reaper checks for stale connections


@dataclass
class ConnectionInfo:
    """Metadata for a single connected client."""

    sid: str
    connected_at: float
    last_heartbeat: float = 0.0  # last app-level heartbeat timestamp
    rooms: set[str] = field(default_factory=set)
    user_id: str | None = None  # optional stable ID for reconnection
    reconnect_count: int = 0

    def __post_init__(self):
        if self.last_heartbeat == 0.0:
            self.last_heartbeat = self.connected_at


class ConnectionManager:
    """Manages active Socket.IO connections.

    Thread-safe for asyncio — all mutations happen in the event loop.

    Features:
    - Connection add/remove lifecycle
    - Heartbeat tracking with stale connection reaping
    - Room-based viewport subscriptions
    - Reconnection state restoration via user_id
    - Broadcast to all, to rooms, or to individual clients

    Usage:
        sio = socketio.AsyncServer(async_mode="asgi")
        manager = ConnectionManager(sio)

        # On connect
        await manager.add(sid)

        # On subscribe:viewport — join grid rooms
        await manager.join_rooms(sid, grid_ids)

        # Broadcast fire update to a specific room
        await manager.broadcast_to_room("fire:update", data, room=grid_id)

        # Broadcast to all connected clients
        await manager.broadcast("fire:update", data)

        # On disconnect
        await manager.remove(sid)
    """

    def __init__(self, sio: socketio.AsyncServer) -> None:
        self._sio = sio
        self._connections: dict[str, ConnectionInfo] = {}
        # Map user_id → last known ConnectionInfo (for reconnection)
        self._user_sessions: dict[str, ConnectionInfo] = {}
        self._reaper_task: asyncio.Task | None = None

    @property
    def active_count(self) -> int:
        """Number of currently active connections."""
        return len(self._connections)

    @property
    def active_sids(self) -> list[str]:
        """List of all active session IDs."""
        return list(self._connections.keys())

    def get(self, sid: str) -> ConnectionInfo | None:
        """Get connection info for a given sid."""
        return self._connections.get(sid)

    async def add(
        self, sid: str, user_id: str | None = None
    ) -> ConnectionInfo:
        """Register a new connection.

        Args:
            sid: Socket.IO session ID.
            user_id: Optional stable user identifier for reconnection support.

        Returns:
            ConnectionInfo for the newly added connection.
        """
        now = time.time()
        reconnect_count = 0

        # Check if this user_id has a previous session (reconnection)
        if user_id and user_id in self._user_sessions:
            prev = self._user_sessions[user_id]
            reconnect_count = prev.reconnect_count + 1
            logger.info(
                "Reconnection detected: user=%s prev_sid=%s new_sid=%s count=%d",
                user_id, prev.sid, sid, reconnect_count,
            )

        info = ConnectionInfo(
            sid=sid,
            connected_at=now,
            last_heartbeat=now,
            user_id=user_id,
            reconnect_count=reconnect_count,
        )
        self._connections[sid] = info

        if user_id:
            self._user_sessions[user_id] = info

        logger.info("Connection added: %s (total: %d)", sid, self.active_count)
        return info

    async def remove(self, sid: str) -> ConnectionInfo | None:
        """Unregister a connection and leave all rooms.

        Args:
            sid: Socket.IO session ID.

        Returns:
            ConnectionInfo of the removed connection, or None if not found.
        """
        info = self._connections.pop(sid, None)
        if info is None:
            logger.warning("Attempted to remove unknown sid: %s", sid)
            return None

        # Leave all tracked rooms
        for room in info.rooms:
            try:
                await self._sio.leave_room(sid, room)
            except Exception:
                pass  # sid may already be disconnected

        logger.info("Connection removed: %s (total: %d)", sid, self.active_count)
        return info

    async def join_rooms(self, sid: str, rooms: list[str]) -> None:
        """Subscribe a client to one or more rooms (grid IDs).

        Replaces any previously subscribed rooms — a client only watches
        one viewport at a time.

        Args:
            sid: Socket.IO session ID.
            rooms: List of room names (typically grid IDs) to join.
        """
        info = self._connections.get(sid)
        if info is None:
            logger.warning("join_rooms called for unknown sid: %s", sid)
            return

        # Leave old rooms
        for old_room in info.rooms:
            try:
                await self._sio.leave_room(sid, old_room)
            except Exception:
                pass

        # Join new rooms
        new_rooms: set[str] = set()
        for room in rooms:
            await self._sio.enter_room(sid, room)
            new_rooms.add(room)

        info.rooms = new_rooms
        logger.debug("sid %s joined %d rooms", sid, len(new_rooms))

    async def broadcast(self, event: str, data: Any) -> None:
        """Broadcast an event to ALL connected clients.

        Args:
            event: Socket.IO event name (e.g. "fire:update").
            data: Payload to send.
        """
        await self._sio.emit(event, data)
        logger.debug("Broadcast '%s' to all (%d clients)", event, self.active_count)

    async def broadcast_to_room(
        self, event: str, data: Any, room: str
    ) -> None:
        """Broadcast an event to all clients in a specific room.

        Args:
            event: Socket.IO event name.
            data: Payload to send.
            room: Room name (typically a grid ID).
        """
        await self._sio.emit(event, data, room=room)
        logger.debug("Broadcast '%s' to room '%s'", event, room)

    async def send_to(self, event: str, data: Any, sid: str) -> None:
        """Send an event to a specific client.

        Args:
            event: Socket.IO event name.
            data: Payload to send.
            sid: Target client's session ID.
        """
        await self._sio.emit(event, data, to=sid)

    # ------------------------------------------------------------------
    # Heartbeat
    # ------------------------------------------------------------------

    def record_heartbeat(self, sid: str) -> bool:
        """Record a heartbeat from the client.

        Args:
            sid: Socket.IO session ID.

        Returns:
            True if the connection exists and heartbeat was recorded.
        """
        info = self._connections.get(sid)
        if info is None:
            return False
        info.last_heartbeat = time.time()
        return True

    def is_stale(self, sid: str, now: float | None = None) -> bool:
        """Check if a connection has missed heartbeats.

        Args:
            sid: Socket.IO session ID.
            now: Current timestamp (defaults to time.time()).

        Returns:
            True if the connection is stale (no heartbeat within timeout).
        """
        info = self._connections.get(sid)
        if info is None:
            return True
        if now is None:
            now = time.time()
        return (now - info.last_heartbeat) > HEARTBEAT_TIMEOUT_SEC

    def get_stale_sids(self, now: float | None = None) -> list[str]:
        """Get all sids that have exceeded the heartbeat timeout.

        Args:
            now: Current timestamp (defaults to time.time()).

        Returns:
            List of stale session IDs.
        """
        if now is None:
            now = time.time()
        return [
            sid
            for sid, info in self._connections.items()
            if (now - info.last_heartbeat) > HEARTBEAT_TIMEOUT_SEC
        ]

    # ------------------------------------------------------------------
    # Stale connection reaper
    # ------------------------------------------------------------------

    def start_reaper(self) -> None:
        """Start the background task that disconnects stale clients."""
        if self._reaper_task is not None:
            return
        self._reaper_task = asyncio.create_task(self._reaper_loop())
        logger.info("Stale connection reaper started (interval=%ds)", REAPER_INTERVAL_SEC)

    async def stop_reaper(self) -> None:
        """Stop the stale connection reaper."""
        if self._reaper_task is None:
            return
        self._reaper_task.cancel()
        try:
            await self._reaper_task
        except asyncio.CancelledError:
            pass
        self._reaper_task = None
        logger.info("Stale connection reaper stopped")

    async def _reaper_loop(self) -> None:
        """Periodically check and disconnect stale connections."""
        while True:
            try:
                await asyncio.sleep(REAPER_INTERVAL_SEC)
                stale = self.get_stale_sids()
                for sid in stale:
                    logger.warning(
                        "Reaping stale connection: %s", sid
                    )
                    try:
                        await self._sio.disconnect(sid)
                    except Exception:
                        # Force-remove from our tracking if disconnect fails
                        await self.remove(sid)
                if stale:
                    logger.info("Reaped %d stale connection(s)", len(stale))
            except asyncio.CancelledError:
                raise
            except Exception:
                logger.exception("Error in reaper loop")

    # ------------------------------------------------------------------
    # Reconnection support
    # ------------------------------------------------------------------

    def get_previous_session(self, user_id: str) -> ConnectionInfo | None:
        """Get the previous session info for a user_id.

        Used during reconnection to restore room subscriptions.

        Args:
            user_id: Stable user identifier.

        Returns:
            Previous ConnectionInfo or None.
        """
        return self._user_sessions.get(user_id)

    async def restore_rooms(self, sid: str, rooms: set[str]) -> None:
        """Restore room subscriptions for a reconnected client.

        Args:
            sid: New Socket.IO session ID.
            rooms: Set of room names to rejoin.
        """
        if not rooms:
            return
        await self.join_rooms(sid, list(rooms))
