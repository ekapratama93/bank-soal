"""generate_image() harus mengubah kegagalan jaringan jadi ImageGenError,
bukan membiarkan exception httpx merambat mentah — lihat app/image_gen.py."""

import httpx
import pytest

from app.config import settings
from app.image_gen import ImageGenError, generate_image


@pytest.fixture(autouse=True)
def _fake_api_key(monkeypatch):
    monkeypatch.setattr(settings, "openrouter_api_key", "fake-key-for-tests")


async def test_connection_error_becomes_image_gen_error(monkeypatch):
    async def fake_post(self, url, headers=None, json=None):
        raise httpx.ConnectError("koneksi gagal")

    monkeypatch.setattr(httpx.AsyncClient, "post", fake_post)

    with pytest.raises(ImageGenError):
        await generate_image("sebuah prompt")


async def test_timeout_becomes_image_gen_error(monkeypatch):
    async def fake_post(self, url, headers=None, json=None):
        raise httpx.ReadTimeout("timeout")

    monkeypatch.setattr(httpx.AsyncClient, "post", fake_post)

    with pytest.raises(ImageGenError):
        await generate_image("sebuah prompt")
