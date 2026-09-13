import logging

import httpx

logger = logging.getLogger(__name__)

OPENVERSE_URL = "https://api.openverse.org/v1/images/"


async def search_stock_image(query: str) -> bytes | None:
    """Cari foto berlisensi terbuka di Openverse. Kembalikan None jika tidak ada
    hasil yang cocok atau layanan gagal — ini kondisi wajar, bukan galat."""
    async with httpx.AsyncClient(timeout=30) as client:
        try:
            resp = await client.get(
                OPENVERSE_URL, params={"q": query, "page_size": 1}
            )
            if resp.status_code != 200:
                return None
            results = resp.json().get("results") or []
            if not results:
                return None
            image_url = results[0].get("url")
            if not image_url:
                return None
            image_resp = await client.get(image_url)
            if image_resp.status_code != 200:
                return None
            return image_resp.content
        except httpx.HTTPError as e:
            logger.warning("Pencarian gambar stok gagal: %s", e)
            return None
