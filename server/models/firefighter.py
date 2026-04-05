"""Firefighter NPC spawn payload generator (visual effect only).

When a grid cell reaches fire stage 4+ (대화재), the server broadcasts a
``firefighter:spawn`` event so the frontend can render a firetruck overlay.
No server-side suppression or state tracking is performed — the frontend
handles the visual lifecycle on its own.
"""

from __future__ import annotations

import time
import uuid

from pydantic import BaseModel, Field

from models.fire import (
    get_stage,
    should_dispatch_firefighter,
    get_firefighter_remove_count,
)


class FirefighterNPC(BaseModel):
    """Spawn payload for a firefighter visual effect on a grid cell."""

    npc_id: str = Field(description="Unique firefighter NPC identifier")
    grid_id: str = Field(description="Grid cell this firefighter is assigned to")
    status: str = Field(default="dispatched")
    dispatched_at: float = Field(description="Unix timestamp of dispatch")
    fires_removed: int = Field(default=0, description="Always 0 (no suppression)")
    remove_per_sweep: int = Field(
        description="Nominal fires-per-sweep (informational only)"
    )
    target_stage: int = Field(
        description="Fire stage that triggered this firefighter"
    )

    def to_broadcast_payload(self) -> dict:
        """Serialize for Socket.IO broadcast."""
        return {
            "npc_id": self.npc_id,
            "grid_id": self.grid_id,
            "status": self.status,
            "dispatched_at": self.dispatched_at,
            "fires_removed": self.fires_removed,
            "remove_per_sweep": self.remove_per_sweep,
            "target_stage": self.target_stage,
        }


def create_firefighter_npc(
    grid_id: str,
    active_count: int,
) -> FirefighterNPC:
    """Factory: create a new firefighter NPC payload for a grid cell.

    Args:
        grid_id: The grid cell that triggered the firefighter.
        active_count: Current active fire count in the cell.

    Returns:
        A new FirefighterNPC ready for broadcast.
    """
    stage = get_stage(active_count)
    remove_count = get_firefighter_remove_count(active_count)

    return FirefighterNPC(
        npc_id=f"ff-{uuid.uuid4().hex[:8]}",
        grid_id=grid_id,
        status="dispatched",
        dispatched_at=time.time(),
        fires_removed=0,
        remove_per_sweep=max(remove_count, 1),
        target_stage=stage.value,
    )


def should_spawn_firefighter(active_count: int) -> bool:
    """Check if a firefighter NPC should be spawned for this fire count.

    Args:
        active_count: Number of active fires in a grid cell.

    Returns:
        True if the cell has reached stage 4+ (대화재).
    """
    return should_dispatch_firefighter(active_count)
