"""SUPABASE_URL/OPENROUTER_URL yang salah bentuk harus gagal cepat saat
proses dimulai, bukan diam-diam sampai request pertama yang memakainya —
lihat app/config.py."""

import pytest
from pydantic import ValidationError

from app.config import Settings


def test_malformed_supabase_url_fails_at_construction():
    with pytest.raises(ValidationError, match="supabase_url"):
        Settings(supabase_url="rkmoxbv.supabase.co", supabase_service_key="k")


def test_malformed_openrouter_url_fails_at_construction():
    with pytest.raises(ValidationError, match="openrouter_url"):
        Settings(openrouter_url="not-a-url")


def test_empty_supabase_url_is_allowed_at_construction():
    """Kosong ditangani terpisah (RuntimeError → 503 saat dipakai) — bukan
    galat konfigurasi, tapi 'belum dikonfigurasi'."""
    Settings(supabase_url="")


def test_valid_https_url_is_accepted():
    s = Settings(supabase_url="https://example.supabase.co", supabase_service_key="k")
    assert s.supabase_url == "https://example.supabase.co"
