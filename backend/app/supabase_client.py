import httpx
from supabase import Client, ClientOptions, create_client

from .config import settings

_client: Client | None = None

QUESTION_IMAGES_BUCKET = "question-images"
_bucket_ready = False

# Satu httpx.Client bersama, dipakai oleh SEMUA sub-klien Supabase (postgrest,
# storage, auth) di kedua fungsi di bawah — termasuk setiap panggilan
# get_fresh_client() (yang sebelumnya membuat pool koneksi baru tiap login).
# Proyek ini memakai Supabase tier gratis (maks 60 koneksi Postgres saat
# puncak); membatasi & memakai ulang koneksi keluar kita sendiri membatasi
# seberapa besar beban bersamaan yang bisa kita berikan ke pool PostgREST,
# bukan angka ajaib — sesuaikan bila lalu lintas sah mulai antre.
_http_client = httpx.Client(
    http2=True,  # meniru default klien internal postgrest-py: multiplexing di atas lebih sedikit socket
    limits=httpx.Limits(max_connections=20, max_keepalive_connections=10),
)


def _require_config() -> None:
    if not settings.supabase_url or not settings.supabase_service_key:
        raise RuntimeError(
            "SUPABASE_URL dan SUPABASE_SERVICE_KEY belum diatur di server"
        )


def close_http_client() -> None:
    """Tutup koneksi bersama saat proses berhenti (dipanggil dari main.py)."""
    _http_client.close()


def get_supabase() -> Client:
    """Klien service key untuk semua operasi tabel — JANGAN pernah
    memanggil auth.sign_in_* pada instance ini, karena supabase-py akan
    mengganti header auth ke token user dan kehilangan akses service_role."""
    global _client
    if _client is None:
        _require_config()
        _client = create_client(
            settings.supabase_url,
            settings.supabase_service_key,
            options=ClientOptions(httpx_client=_http_client),
        )
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
    """Klien baru khusus untuk operasi login (sign_in_with_password).

    Wrapper Client-nya baru setiap panggilan — supaya header auth token user
    dari satu login tidak bocor ke pemanggil lain (lihat docstring di atas) —
    tapi transport httpx di baliknya DIBAGI (lihat _http_client) supaya login
    yang sering/berulang memakai ulang koneksi yang sudah ada alih-alih
    membuka pool koneksi baru tiap kali."""
    _require_config()
    return create_client(
        settings.supabase_url,
        settings.supabase_service_key,
        options=ClientOptions(httpx_client=_http_client),
    )
