import base64
import logging

import httpx

from .config import settings

logger = logging.getLogger(__name__)


class ImageGenError(Exception):
    pass


async def generate_image(prompt: str) -> tuple[bytes, str]:
    """Panggil model image-output di OpenRouter dan kembalikan (bytes, content_type)."""
    if not settings.openrouter_api_key:
        raise ImageGenError("OPENROUTER_API_KEY belum diatur di server")
    headers = {
        "Authorization": f"Bearer {settings.openrouter_api_key}",
        "Content-Type": "application/json",
    }
    payload = {
        "model": settings.openrouter_image_model,
        "messages": [{"role": "user", "content": prompt}],
        "modalities": ["image", "text"],
    }
    async with httpx.AsyncClient(timeout=120) as client:
        resp = await client.post(settings.openrouter_url, headers=headers, json=payload)
        if resp.status_code != 200:
            logger.warning("OpenRouter image error %s: %s", resp.status_code, resp.text[:500])
            raise ImageGenError("Gagal membuat gambar dari layanan AI.")
        try:
            message = resp.json()["choices"][0]["message"]
            image_url = message["images"][0]["image_url"]["url"]
            header, b64data = image_url.split(",", 1)
            content_type = header.split(":")[1].split(";")[0]
            data = base64.b64decode(b64data)
        except (KeyError, IndexError, ValueError) as e:
            logger.warning("Respons image OpenRouter tidak terduga: %s", e)
            raise ImageGenError("Respons gambar AI tidak valid.") from e
        return data, content_type
