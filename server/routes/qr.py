"""QR code generation endpoint.

Returns a QR code image (PNG) that encodes the frontend map page URL.
Used at the hackathon so ~30 users can scan and join instantly.
"""

from __future__ import annotations

import io
import os

import qrcode
from fastapi import APIRouter, Query
from fastapi.responses import StreamingResponse

router = APIRouter(prefix="/api", tags=["qr"])

# Default frontend URL — overridable via FRONTEND_URL env var
_DEFAULT_FRONTEND_URL = "https://bulpan.example.com"


def _get_map_url(demo: bool = False) -> str:
    """Build the map page URL, optionally with ?demo=true for GPS fallback."""
    base = os.getenv("FRONTEND_URL", _DEFAULT_FRONTEND_URL)
    # Strip trailing slash for consistency
    base = base.rstrip("/")
    if demo:
        return f"{base}?demo=true"
    return base


def _generate_qr_png(data: str, box_size: int = 10, border: int = 2) -> bytes:
    """Generate a QR code PNG image as bytes."""
    qr = qrcode.QRCode(
        version=None,  # auto-size
        error_correction=qrcode.constants.ERROR_CORRECT_M,
        box_size=box_size,
        border=border,
    )
    qr.add_data(data)
    qr.make(fit=True)

    img = qr.make_image(fill_color="black", back_color="white")
    buf = io.BytesIO()
    img.save(buf, format="PNG")
    buf.seek(0)
    return buf.getvalue()


@router.get(
    "/qr",
    summary="Generate QR code for map page",
    responses={
        200: {
            "content": {"image/png": {}},
            "description": "QR code PNG image linking to the map page",
        }
    },
)
async def generate_qr(
    demo: bool = Query(False, description="Include ?demo=true for GPS fallback"),
    size: int = Query(10, ge=4, le=40, description="QR box size in pixels (4-40)"),
):
    """Return a QR code image (PNG) that links to the frontend map page.

    Scan this QR at the hackathon venue to instantly open the fire map.
    """
    url = _get_map_url(demo=demo)
    png_bytes = _generate_qr_png(url, box_size=size)

    return StreamingResponse(
        io.BytesIO(png_bytes),
        media_type="image/png",
        headers={
            "Cache-Control": "public, max-age=3600",
            "X-QR-URL": url,
        },
    )
