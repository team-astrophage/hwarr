"""Tests for the QR code generation endpoint."""

from __future__ import annotations

import os
from unittest.mock import patch

import pytest
from httpx import ASGITransport, AsyncClient

from routes.qr import _generate_qr_png, _get_map_url


# ---------------------------------------------------------------------------
# Unit tests for helper functions
# ---------------------------------------------------------------------------


class TestGetMapUrl:
    def test_default_url(self):
        with patch.dict(os.environ, {}, clear=True):
            # Remove FRONTEND_URL if set
            os.environ.pop("FRONTEND_URL", None)
            url = _get_map_url()
            assert url == "https://bulpan.example.com"

    def test_custom_url(self):
        with patch.dict(os.environ, {"FRONTEND_URL": "https://my-app.com/"}):
            url = _get_map_url()
            assert url == "https://my-app.com"  # trailing slash stripped

    def test_demo_mode(self):
        with patch.dict(os.environ, {"FRONTEND_URL": "https://my-app.com"}):
            url = _get_map_url(demo=True)
            assert url == "https://my-app.com?demo=true"


class TestGenerateQrPng:
    def test_returns_png_bytes(self):
        result = _generate_qr_png("https://example.com")
        assert isinstance(result, bytes)
        # PNG magic bytes
        assert result[:4] == b"\x89PNG"

    def test_custom_size(self):
        small = _generate_qr_png("https://example.com", box_size=4)
        large = _generate_qr_png("https://example.com", box_size=20)
        # Larger box_size should produce larger image
        assert len(large) > len(small)


# ---------------------------------------------------------------------------
# Integration test for the endpoint
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_qr_endpoint_returns_png():
    """GET /api/qr should return a PNG image."""
    from main import app

    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.get("/api/qr")

    assert resp.status_code == 200
    assert resp.headers["content-type"] == "image/png"
    assert resp.content[:4] == b"\x89PNG"
    assert "X-QR-URL" in resp.headers


@pytest.mark.asyncio
async def test_qr_endpoint_demo_mode():
    """GET /api/qr?demo=true should include demo=true in the URL."""
    from main import app

    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.get("/api/qr", params={"demo": "true"})

    assert resp.status_code == 200
    assert "demo=true" in resp.headers["X-QR-URL"]
