from supabase import Client, create_client

from .config import settings

_client: Client | None = None

QUESTION_IMAGES_BUCKET = "question-images"
_bucket_ready = False


def get_supabase() -> Client:
    """Klien service key untuk semua operasi tabel — JANGAN pernah
    memanggil auth.sign_in_* pada instance ini, karena supabase-py akan
    mengganti header auth ke token user dan kehilangan akses service_role."""
    global _client
    if _client is None:
        if not settings.supabase_url or not settings.supabase_service_key:
            raise RuntimeError(
                "SUPABASE_URL dan SUPABASE_SERVICE_KEY belum diatur di server"
            )
        _client = create_client(settings.supabase_url, settings.supabase_service_key)
    return _client


def ensure_bucket(sb: Client) -> None:
    """Buat bucket penyimpanan gambar soal sekali saja (aman dipanggil berulang)."""
    global _bucket_ready
    if _bucket_ready:
        return
    try:
        sb.storage.create_bucket(QUESTION_IMAGES_BUCKET, options={"public": True})
    except Exception:
        pass  # bucket sudah ada
    _bucket_ready = True


def get_fresh_client() -> Client:
    """Klien baru khusus untuk operasi login (sign_in_with_password)."""
    if not settings.supabase_url or not settings.supabase_service_key:
        raise RuntimeError(
            "SUPABASE_URL dan SUPABASE_SERVICE_KEY belum diatur di server"
        )
    return create_client(settings.supabase_url, settings.supabase_service_key)