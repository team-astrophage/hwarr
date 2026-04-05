"""User feedback API — forwards user opinion to the developer inbox.

POST /api/feedback — accept bug/idea/etc text, forward as email.

Abuse protection: Redis-backed per-IP rate limit (10 min / N requests).
Emails are sent via aiosmtplib using configured SMTP credentials.
"""

from __future__ import annotations

import html as html_lib
import logging
from typing import Literal, Optional

from fastapi import APIRouter, HTTPException, Request
from pydantic import BaseModel, EmailStr, Field

from config import DEVELOPER_EMAIL, FEEDBACK_RATE_LIMIT_PER_10MIN
from services.mailer import send_mail

logger = logging.getLogger(__name__)

router = APIRouter(prefix="/api", tags=["feedback"])


# ---------------------------------------------------------------------------
# Request / Response models
# ---------------------------------------------------------------------------


class FeedbackRequest(BaseModel):
    category: Literal["bug", "idea", "etc"] = Field(description="Feedback category")
    message: str = Field(min_length=1, max_length=500, description="User message body")
    email: Optional[EmailStr] = Field(default=None, description="Optional reply-to email")
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
_CATEGORY_LABELS = {"bug": "버그", "idea": "제안", "etc": "기타"}


@router.post(
    "/feedback",
    response_model=FeedbackResponse,
    summary="Submit user feedback to the developer",
    status_code=201,
)
async def submit_feedback(body: FeedbackRequest, request: Request) -> FeedbackResponse:
    """Receive feedback, rate-limit by IP, forward to developer via email."""
    if not DEVELOPER_EMAIL:
        logger.error("DEVELOPER_EMAIL not configured — rejecting feedback")
        raise HTTPException(status_code=503, detail="feedback_disabled")

    # ---- rate limit (soft fail if Redis is unavailable) ----
    ip = request.client.host if request.client else "unknown"
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

    # ---- compose mail ----
    label = _CATEGORY_LABELS.get(body.category, body.category)
    ua = request.headers.get("user-agent", "-")
    # first 20 chars for subject preview (no newlines)
    preview = body.message.replace("\n", " ").strip()[:20]
    subject = f"[화르르 피드백/{label}] {preview}"

    safe_msg = html_lib.escape(body.message)
    safe_page = html_lib.escape(body.page or "-")
    safe_email = html_lib.escape(body.email or "-")
    safe_ua = html_lib.escape(ua)
    safe_ip = html_lib.escape(ip)

    html_body = (
        f"<div style='font-family:system-ui,-apple-system,sans-serif;font-size:14px'>"
        f"<p><b>카테고리:</b> {label}</p>"
        f"<p><b>페이지:</b> {safe_page}</p>"
        f"<p><b>답변 이메일:</b> {safe_email}</p>"
        f"<p><b>IP:</b> {safe_ip}</p>"
        f"<p><b>User-Agent:</b> {safe_ua}</p>"
        f"<hr>"
        f"<pre style='white-space:pre-wrap;font-family:inherit;font-size:14px'>"
        f"{safe_msg}"
        f"</pre>"
        f"</div>"
    )

    # ---- send ----
    try:
        await send_mail(to=DEVELOPER_EMAIL, subject=subject, html=html_body)
    except Exception:
        logger.exception("failed to send feedback mail (category=%s)", body.category)
        raise HTTPException(status_code=502, detail="mail_send_failed")

    logger.info(
        "feedback received category=%s ip=%s len=%d has_email=%s",
        body.category, ip, len(body.message), bool(body.email),
    )
    return FeedbackResponse(ok=True)
