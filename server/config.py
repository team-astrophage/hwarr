"""Centralized configuration for 전국불판 server.

All tunable constants in one place, with environment variable overrides.
"""

import os
from pathlib import Path
from zoneinfo import ZoneInfo

KST = ZoneInfo("Asia/Seoul")

# ---------------------------------------------------------------------------
# TTL settings (seconds)
# ---------------------------------------------------------------------------

FIRE_TTL_SEC = int(os.getenv("FIRE_TTL_SEC", 2400))               # 40 minutes
NEWS_TTL_SEC = int(os.getenv("NEWS_TTL_SEC", 86400))              # 1 day

# ---------------------------------------------------------------------------
# Redis keys for global stats
# ---------------------------------------------------------------------------

STATS_TOTAL_FIRES_KEY = "stats:total_fires"
STATS_DAILY_FIRES_PREFIX = "stats:daily_fires:"
STATS_DAILY_FIRES_TTL_SEC = 60 * 60 * 48  # 48h TTL (KST 자정 경계 여유)
STATS_DAILY_RANKING_PREFIX = "stats:daily_ranking:"
STATS_DAILY_RANKING_TTL_SEC = 60 * 60 * 48  # 48h TTL

# ---------------------------------------------------------------------------
# Admin region GeoJSON (for daily ranking)
# ---------------------------------------------------------------------------

ADMIN_GEOJSON_PATH = os.getenv(
    "ADMIN_GEOJSON_PATH",
    str(Path(__file__).parent / "data" / "admin_dong.geojson"),
)

# ---------------------------------------------------------------------------
# SMTP / feedback email
# ---------------------------------------------------------------------------

SMTP_HOST = os.getenv("SMTP_HOST", "smtp.gmail.com")
SMTP_PORT = int(os.getenv("SMTP_PORT", "587"))
SMTP_USER = os.getenv("SMTP_USER", "")
SMTP_PASSWORD = os.getenv("SMTP_PASSWORD", "")
DEVELOPER_EMAIL = os.getenv("DEVELOPER_EMAIL", "")
FEEDBACK_RATE_LIMIT_PER_10MIN = int(os.getenv("FEEDBACK_RATE_LIMIT_PER_10MIN", "3"))
