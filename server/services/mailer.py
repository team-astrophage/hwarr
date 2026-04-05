"""SMTP email sender (aiosmtplib).

Lightweight async wrapper used for outbound transactional emails
(e.g. user feedback forwarding to developer inbox).
"""

from __future__ import annotations

import logging

import aiosmtplib
from email.message import EmailMessage

from config import SMTP_HOST, SMTP_PASSWORD, SMTP_PORT, SMTP_USER

logger = logging.getLogger(__name__)


async def send_mail(*, to: str, subject: str, html: str) -> None:
    """Send an HTML email via configured SMTP server.

    Raises aiosmtplib.SMTPException on failure — caller decides whether
    to surface this as a 5xx response.
    """
    if not (SMTP_USER and SMTP_PASSWORD and to):
        raise RuntimeError("SMTP not configured (SMTP_USER/SMTP_PASSWORD/to missing)")

    msg = EmailMessage()
    msg["From"] = SMTP_USER
    msg["To"] = to
    msg["Subject"] = subject
    msg.set_content("이 메일은 HTML 형식입니다.")
    msg.add_alternative(html, subtype="html")

    await aiosmtplib.send(
        msg,
        hostname=SMTP_HOST,
        port=SMTP_PORT,
        username=SMTP_USER,
        password=SMTP_PASSWORD,
        start_tls=True,
    )
    logger.info("mail sent to=%s subject=%s", to, subject)
