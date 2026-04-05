"""User feedback API — forwards user opinion to the Discord dev channel.

POST /api/feedback — accept bug/idea/etc text, forward as Discord embed.

Abuse protection: Redis-backed per-IP rate limit (10 min / N requests).
Messages are sent via Discord incoming webhook.
"""

from __future__ import annotations

import logging
from datetime import datetime, timezone
from typing import Literal, Optional

from fastapi import APIRouter, HTTPException, Request
from pydantic import BaseModel, Field

from config import FEEDBACK_DISCORD_WEBHOOK_URL, FEEDBACK_RATE_LIMIT_PER_10MIN
from services.notifier import send_discord_webhook

logger = logging.getLogger(__name__)

router = APIRouter(prefix="/api", tags=["feedback"])


# ---------------------------------------------------------------------------
# Request / Response models
# ---------------------------------------------------------------------------


class FeedbackRequest(BaseModel):
    category: Literal["bug", "idea", "etc"] = Field(description="Feedback category")
    message: str = Field(min_length=1, max_length=500, description="User message body")
    email: Optional[str] = Field(default=None, max_length=200, description="Optional reply-to email")
    page: Optional[str] = Field(default=None, max_length=200, description="Current page path")


class FeedbackResponse(BaseModel):
    ok: bool


# ---------------------------------------------------------------------------
# Engine / Redis accessor
# ---------------------------------------------------------------------------


def _get_redis():
    """Reuse the Redis client owned by the FireProgressionEngine.

    We avoid opening a second connection pool; the engine's client is
    initialised on server startup (see main.py startup_event).
    """
    from main import engine
    if engine is None:
        return None
    return getattr(engine, "_redis", None)


# ---------------------------------------------------------------------------
# POST /api/feedback
# ---------------------------------------------------------------------------

_RL_WINDOW_SEC = 600  # 10 minutes
_CATEGORY_LABELS = {
    "bug": "🐛 버그 제보",
    "idea": "💡 기능 제안",
    "etc": "💬 기타 의견",
}
_CATEGORY_COLORS = {
    "bug": 0xF3727F,   # negative red
    "idea": 0x1ED760,  # accent green
    "etc": 0xFFA42B,   # warning orange
}
_UA_MAX_LEN = 80


def _short_ua(ua: str) -> str:
    """Shorten a User-Agent string for compact display in the embed footer."""
    if not ua or ua == "-":
        return "unknown client"
    # Prefer the trailing product token (browser/os summary) which is usually
    # more informative than the leading "Mozilla/5.0 (...)" block.
    # Fall back to a simple head-truncate.
    if len(ua) <= _UA_MAX_LEN:
        return ua
    return ua[: _UA_MAX_LEN - 1] + "…"


def _quote_lines(text: str) -> str:
    """Prefix each line with a Discord blockquote marker for readability."""
    return "\n".join(f"> {line}" if line else ">" for line in text.splitlines())


@router.post(
    "/feedback",
    response_model=FeedbackResponse,
    summary="Submit user feedback to the Discord channel",
    status_code=201,
)
async def submit_feedback(body: FeedbackRequest, request: Request) -> FeedbackResponse:
    """Receive feedback, rate-limit by IP, forward to Discord webhook."""
    if not FEEDBACK_DISCORD_WEBHOOK_URL:
        logger.error("FEEDBACK_DISCORD_WEBHOOK_URL not configured — rejecting feedback")
        raise HTTPException(status_code=503, detail="feedback_disabled")

    # ---- resolve real client IP & UA (CloudFront/ALB-aware) ----
    # CloudFront prepends the viewer IP to X-Forwarded-For; ALB appends its
    # own hop. The first entry is the original client.
    xff = request.headers.get("x-forwarded-for", "")
    if xff:
        ip = xff.split(",")[0].strip()
    else:
        ip = request.client.host if request.client else "unknown"

    # CloudFront overwrites the incoming User-Agent with "Amazon CloudFront"
    # unless we explicitly forward it. When forwarded, it is also available
    # as `cloudfront-viewer-user-agent` (managed origin request policy).
    ua = (
        request.headers.get("cloudfront-viewer-user-agent")
        or request.headers.get("user-agent")
        or "-"
    )

    # ---- rate limit (soft fail if Redis is unavailable) ----
    redis = _get_redis()
    if redis is not None:
        key = f"feedback:rl:{ip}"
        try:
            count = await redis.incr(key)
            if count == 1:
                await redis.expire(key, _RL_WINDOW_SEC)
            if count > FEEDBACK_RATE_LIMIT_PER_10MIN:
                raise HTTPException(status_code=429, detail="rate_limited")
        except HTTPException:
            raise
        except Exception:
            logger.exception("feedback rate-limit check failed — allowing through")

    # ---- compose discord embed ----
    label = _CATEGORY_LABELS.get(body.category, body.category)
    color = _CATEGORY_COLORS.get(body.category, 0x888888)

    # Only include optional fields when they actually have content — avoids
    # visual noise from rows of "-" placeholders.
    fields: list[dict] = []
    if body.page:
        fields.append({"name": "페이지", "value": f"`{body.page}`", "inline": True})
    if body.email:
        fields.append({"name": "답변 이메일", "value": body.email, "inline": True})

    embed = {
        "author": {"name": label},
        "description": _quote_lines(body.message),
        "color": color,
        "fields": fields,
        "footer": {"text": f"{ip} · {_short_ua(ua)}"},
        "timestamp": datetime.now(timezone.utc).isoformat(),
    }
    payload = {
        "embeds": [embed],
        # Never ping — user input must not be able to trigger @everyone etc.
        "allowed_mentions": {"parse": []},
    }

    # ---- send ----
    try:
        await send_discord_webhook(FEEDBACK_DISCORD_WEBHOOK_URL, payload)
    except Exception:
        logger.exception("failed to deliver feedback to discord (category=%s)", body.category)
        raise HTTPException(status_code=502, detail="webhook_failed")

    logger.info(
        "feedback received category=%s ip=%s len=%d has_email=%s",
        body.category, ip, len(body.message), bool(body.email),
    )
    return FeedbackResponse(ok=True)
