"""Centralized configuration for 전국불판 server.

All tunable constants in one place, with environment variable overrides.
"""

import os

# ---------------------------------------------------------------------------
# TTL settings (seconds)
# ---------------------------------------------------------------------------

FIRE_TTL_SEC = int(os.getenv("FIRE_TTL_SEC", 86400))              # 1 day
FIREFIGHTER_TTL_SEC = int(os.getenv("FIREFIGHTER_TTL_SEC", 86400))  # 1 day
NEWS_TTL_SEC = int(os.getenv("NEWS_TTL_SEC", 86400))              # 1 day

# ---------------------------------------------------------------------------
# Redis keys for global stats
# ---------------------------------------------------------------------------

STATS_TOTAL_FIRES_KEY = "stats:total_fires"
