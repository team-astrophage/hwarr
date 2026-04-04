"""Firefighter NPC model and spawn logic.

When a grid cell reaches fire stage 4+ (대화재), the system automatically
spawns a firefighter NPC assigned to suppress that cell. Each NPC tracks:
  - which grid cell it's assigned to
  - when it was dispatched
  - how many fires it removes per sweep
  - its current status (dispatched → active → done)

NPCs are stored in Redis hashes with a 30-minute TTL.
"""

from __future__ import annotations

import time
import uuid
from enum import StrEnum
from typing import Optional

from pydantic import BaseModel, Field

from config import FIREFIGHTER_TTL_SEC
from models.fire import (
    FireStage,
    STAGE_CONFIGS,
    get_stage,
    should_dispatch_firefighter,
    get_firefighter_remove_count,
)


# Redis key prefix for firefighter NPCs
FIREFIGHTER_KEY_PREFIX = "firefighter:"
# Redis set tracking all active firefighter IDs
ACTIVE_FIREFIGHTERS_KEY = "active_firefighters"
# Redis set tracking which grids already have a firefighter assigned
FIREFIGHTER_GRID_INDEX_KEY = "firefighter_grids"


class FirefighterStatus(StrEnum):
    """Lifecycle states for a firefighter NPC."""

    DISPATCHED = "dispatched"   # Just spawned, heading to the cell
    ACTIVE = "active"           # Currently suppressing fires
    DONE = "done"               # Finished suppression, cell is safe


class FirefighterNPC(BaseModel):
    """A firefighter NPC assigned to suppress a grid cell."""

    npc_id: str = Field(description="Unique firefighter NPC identifier")
    grid_id: str = Field(description="Grid cell this firefighter is assigned to")
    status: FirefighterStatus = Field(default=FirefighterStatus.DISPATCHED)
    dispatched_at: float = Field(description="Unix timestamp of dispatch")
    fires_removed: int = Field(default=0, description="Total fires suppressed so far")
    remove_per_sweep: int = Field(
        description="Number of fires removed each sweep cycle"
    )
    target_stage: int = Field(
        description="Fire stage that triggered this firefighter"
    )

    def to_broadcast_payload(self) -> dict:
        """Serialize for Socket.IO broadcast."""
        return {
            "npc_id": self.npc_id,
            "grid_id": self.grid_id,
            "status": self.status.value,
            "dispatched_at": self.dispatched_at,
            "fires_removed": self.fires_removed,
            "remove_per_sweep": self.remove_per_sweep,
            "target_stage": self.target_stage,
        }


def create_firefighter_npc(
    grid_id: str,
    active_count: int,
) -> FirefighterNPC:
    """Factory: create a new firefighter NPC for a grid cell.

    Args:
        grid_id: The grid cell that triggered the firefighter.
        active_count: Current active fire count in the cell.

    Returns:
        A new FirefighterNPC ready for dispatch.
    """
    stage = get_stage(active_count)
    remove_count = get_firefighter_remove_count(active_count)

    return FirefighterNPC(
        npc_id=f"ff-{uuid.uuid4().hex[:8]}",
        grid_id=grid_id,
        status=FirefighterStatus.DISPATCHED,
        dispatched_at=time.time(),
        fires_removed=0,
        remove_per_sweep=max(remove_count, 1),
        target_stage=stage.value,
    )


def should_spawn_firefighter(active_count: int) -> bool:
    """Check if a firefighter NPC should be spawned for this fire count.

    This is a convenience wrapper around should_dispatch_firefighter
    that also serves as the single entry point for spawn detection.

    Args:
        active_count: Number of active fires in a grid cell.

    Returns:
        True if the cell has reached stage 4+ (대화재) and needs a firefighter.
    """
    return should_dispatch_firefighter(active_count)
