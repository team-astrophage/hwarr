"""Discord webhook notifier.

Lightweight async wrapper to POST messages to a Discord channel
via an incoming webhook URL. Used to forward user feedback to
the dev team channel.
"""

from __future__ import annotations

import logging
from typing import Any

import httpx

logger = logging.getLogger(__name__)

_TIMEOUT_SEC = 5.0


async def send_discord_webhook(webhook_url: str, payload: dict[str, Any]) -> None:
    """POST a message payload to a Discord webhook.

    Discord expects either a plain `content` string or `embeds` array.
    Raises httpx.HTTPError on network/HTTP failures — caller decides
    whether to surface this as a 5xx response.
    """
    if not webhook_url:
        raise RuntimeError("discord webhook url is empty")

    async with httpx.AsyncClient(timeout=_TIMEOUT_SEC) as client:
        resp = await client.post(webhook_url, json=payload)
        resp.raise_for_status()
    logger.info("discord webhook delivered (status=%d)", resp.status_code)
