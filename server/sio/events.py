"""Socket.IO event handlers for fire state-change broadcasting.

Handles client-initiated events:
- fire:ignite   — user taps to ignite a fire at GPS coordinates
- subscribe:viewport — user subscribes to a map viewport (joins grid rooms)
- fire:state    — user requests current fire state for visible grids

Each event that mutates state triggers broadcasts to all connected clients
via the ConnectionManager and FireProgressionEngine.
"""

from __future__ import annotations

import logging
import random
import time
import uuid
from typing import TYPE_CHECKING, Any

from config import FIRE_TTL_SEC
from grid import get_grids_in_viewport, to_grid_id
from models.fire import build_grid_state, get_stage
from routes.map_config import _PREDEFINED_LOCATIONS

if TYPE_CHECKING:
    import socketio

    from jobs.fire_progression import FireProgressionEngine
    from sio.connection_manager import ConnectionManager

logger = logging.getLogger(__name__)


def register_fire_events(
    sio: socketio.AsyncServer,
    manager: ConnectionManager,
    engine: FireProgressionEngine,
) -> None:
    """Register all fire-related Socket.IO event handlers.

    This function wires up the Socket.IO server with the connection manager
    and fire progression engine so that client events trigger state changes
    and broadcasts.

    Args:
        sio: The Socket.IO async server instance.
        manager: ConnectionManager for tracking connections and rooms.
        engine: FireProgressionEngine for fire state management.
    """

    @sio.on("fire:ignite")
    async def handle_fire_ignite(sid: str, data: dict[str, Any]) -> dict[str, Any]:
        """Handle fire ignition from a client.

        Client sends: { "lat": float, "lng": float }
        Server:
          1. Converts GPS → grid ID
          2. Registers fire event in Redis (via engine)
          3. Engine broadcasts fire:update to room on stage change
          4. Returns ack with grid state to the igniting client

        Args:
            sid: Socket.IO session ID of the igniting client.
            data: Payload with 'lat' and 'lng' fields.

        Returns:
            Acknowledgement dict with fire event details.
        """
        lat = data.get("lat")
        lng = data.get("lng")
        demo = data.get("demo", False)

        # Demo mode fallback: pick a random predefined location when GPS
        # is unavailable (client sends { "demo": true } without lat/lng)
        demo_location_name = None
        if demo and (lat is None or lng is None):
            loc = random.choice(_PREDEFINED_LOCATIONS)
            lat = loc["lat"]
            lng = loc["lng"]
            demo_location_name = loc["name"]
            logger.info(
                "fire:ignite demo fallback sid=%s → %s (%s)",
                sid, loc["id"], loc["name"],
            )
        elif lat is None or lng is None:
            logger.warning("fire:ignite from %s missing lat/lng: %s", sid, data)
            return {"error": "lat and lng are required"}

        try:
            lat = float(lat)
            lng = float(lng)
        except (TypeError, ValueError):
            logger.warning("fire:ignite from %s invalid coords: %s", sid, data)
            return {"error": "lat and lng must be numbers"}

        requested_grid_id = to_grid_id(lat, lng)
        event_id = f"fire-{uuid.uuid4().hex[:12]}"
        expire_at = time.time() + FIRE_TTL_SEC

        # Register fire — engine handles neighbor-spreading, stage detection,
        # and broadcasting. The landing grid may differ from the requested one.
        registration = await engine.register_fire(requested_grid_id, event_id, expire_at)
        grid_id = registration.grid_id
        active_count = registration.active_count
        spread_path = [
            {"from": src, "to": dst} for src, dst in registration.spread_path
        ]

        # Build response for the igniting client
        state = build_grid_state(
            grid_id=grid_id,
            active_count=active_count,
            lat=lat,
            lng=lng,
        )

        # Also broadcast fire:ignite to the grid room so all watchers
        # see the new fire immediately (even if stage didn't change)
        ignite_payload = {
            "grid_id": grid_id,
            "event_id": event_id,
            "lat": lat,
            "lng": lng,
            "active_count": active_count,
            "stage": state.stage,
            "stage_info": state.stage_info.model_dump(),
            "ignited_by": sid,
            "timestamp": time.time(),
        }
        await manager.broadcast_to_room("fire:ignite", ignite_payload, room=grid_id)

        # Also broadcast globally for map overview updates
        await manager.broadcast("fire:global_update", {
            "grid_id": grid_id,
            "active_count": active_count,
            "stage": state.stage,
            "stage_info": state.stage_info.model_dump(),
            "timestamp": time.time(),
        })

        logger.info(
            "fire:ignite sid=%s requested=%s landed=%s count=%d stage=%d spread=%d",
            sid, requested_grid_id, grid_id, active_count, state.stage, len(spread_path),
        )

        result = {
            "status": "ok",
            "grid_id": grid_id,
            "requested_grid_id": requested_grid_id,
            "event_id": event_id,
            "active_count": active_count,
            "stage": state.stage,
            "stage_info": state.stage_info.model_dump(),
            "spread_path": spread_path,
        }
        if demo:
            result["demo"] = True
            result["lat"] = lat
            result["lng"] = lng
            if demo_location_name:
                result["demo_location"] = demo_location_name
        return result

    @sio.on("subscribe:viewport")
    async def handle_subscribe_viewport(sid: str, data: dict[str, Any]) -> dict[str, Any]:
        """Subscribe client to fire updates within a map viewport.

        Client sends: { "ne_lat": float, "ne_lng": float, "sw_lat": float, "sw_lng": float }
        Server:
          1. Computes all grid IDs within the bounding box
          2. Joins the client to those grid rooms (replacing previous rooms)
          3. Sends current fire state for all visible grids

        Args:
            sid: Socket.IO session ID.
            data: Viewport bounding box coordinates.

        Returns:
            Acknowledgement with subscribed grid count and current states.
        """
        try:
            ne_lat = float(data["ne_lat"])
            ne_lng = float(data["ne_lng"])
            sw_lat = float(data["sw_lat"])
            sw_lng = float(data["sw_lng"])
        except (KeyError, TypeError, ValueError) as e:
            logger.warning("subscribe:viewport from %s invalid data: %s (%s)", sid, data, e)
            return {"error": "ne_lat, ne_lng, sw_lat, sw_lng are required numbers"}

        grid_ids = get_grids_in_viewport(ne_lat, ne_lng, sw_lat, sw_lng)

        # Join rooms for all grids in the viewport
        await manager.join_rooms(sid, grid_ids)

        # Collect current state for visible grids that have fires
        now = time.time()
        grid_states = []
        for grid_id in grid_ids:
            active_count = await engine._get_active_count(grid_id, now)
            if active_count > 0:
                state = build_grid_state(grid_id=grid_id, active_count=active_count)
                grid_states.append(state.model_dump())

        logger.info(
            "subscribe:viewport sid=%s grids=%d active=%d",
            sid, len(grid_ids), len(grid_states),
        )

        return {
            "status": "ok",
            "subscribed_grids": len(grid_ids),
            "active_fires": grid_states,
        }

    @sio.on("fire:state")
    async def handle_fire_state(sid: str, data: dict[str, Any]) -> dict[str, Any]:
        """Request current fire state for specific grids or all active grids.

        Client sends: { "grid_ids": [str, ...] }  (optional — all active if omitted)
        Server returns current state for requested grids.

        Args:
            sid: Socket.IO session ID.
            data: Optional grid_ids list.

        Returns:
            Dict with grid states.
        """
        now = time.time()
        requested_ids = data.get("grid_ids")

        if requested_ids:
            grid_ids = requested_ids
        else:
            # Return all active grids
            grid_ids = await engine._get_active_grid_ids()

        grid_states = []
        for grid_id in grid_ids:
            active_count = await engine._get_active_count(grid_id, now)
            if active_count > 0:
                state = build_grid_state(grid_id=grid_id, active_count=active_count)
                grid_states.append(state.model_dump())

        return {
            "status": "ok",
            "grids": grid_states,
            "total_active_grids": len(grid_states),
        }

    # ------------------------------------------------------------------
    # Mock server compatibility handlers
    # ------------------------------------------------------------------

    @sio.on("fire")
    async def handle_fire_compat(sid: str, data: dict[str, Any]) -> None:
        """Handle 'fire' event (mock server compat).

        Frontend sends: { "lat": float, "lng": float }
        Delegates to the same logic as fire:ignite, then broadcasts
        fire:update in camelCase format as the mock server did.
        """
        lat = data.get("lat")
        lng = data.get("lng")

        if lat is None or lng is None:
            return

        try:
            lat = float(lat)
            lng = float(lng)
        except (TypeError, ValueError):
            return

        requested_grid_id = to_grid_id(lat, lng)
        event_id = f"fire-{uuid.uuid4().hex[:12]}"
        expire_at = time.time() + FIRE_TTL_SEC

        registration = await engine.register_fire(requested_grid_id, event_id, expire_at)
        grid_id = registration.grid_id
        active_count = registration.active_count
        # Use the landing grid's center for lat/lng when the fire spread to a
        # neighbor; this keeps the camelCase broadcast consistent with grid_id.
        if grid_id != requested_grid_id:
            from grid import grid_id_to_center
            lat, lng = grid_id_to_center(grid_id)
        state = build_grid_state(
            grid_id=grid_id, active_count=active_count, lat=lat, lng=lng,
        )

        # Broadcast in camelCase format (mock server compat)
        await manager.broadcast("fire:update", {
            "gridId": grid_id,
            "activeCount": active_count,
            "stage": state.stage,
            "lat": lat,
            "lng": lng,
        })

        logger.info(
            "fire (compat) sid=%s requested=%s landed=%s count=%d stage=%d spread=%d",
            sid, requested_grid_id, grid_id, active_count, state.stage,
            len(registration.spread_path),
        )

    @sio.on("get_fires")
    async def handle_get_fires_compat(sid: str, data: Any = None) -> None:
        """Handle 'get_fires' event (mock server compat).

        Returns all active fires via 'fires:sync' event to the requester,
        using camelCase keys as the mock server did.
        """
        now = time.time()
        grid_ids = await engine._get_active_grid_ids()

        fires_list = []
        for grid_id in grid_ids:
            active_count = await engine._get_active_count(grid_id, now)
            if active_count > 0:
                stage = get_stage(active_count)
                fires_list.append({
                    "gridId": grid_id,
                    "activeCount": active_count,
                    "stage": int(stage),
                })

        await manager.send_to("fires:sync", fires_list, sid)
