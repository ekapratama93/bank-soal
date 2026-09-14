"""Klien Supabase harus memakai ulang satu httpx.Client bersama, bukan
membuat pool koneksi baru tiap panggilan (lihat app/supabase_client.py)."""

import app.supabase_client as sc


def _reset_singleton(monkeypatch):
    monkeypatch.setattr(sc, "_client", None)


def test_get_supabase_uses_shared_http_client(monkeypatch):
    _reset_singleton(monkeypatch)
    monkeypatch.setattr(sc.settings, "supabase_url", "https://example.supabase.co")
    monkeypatch.setattr(sc.settings, "supabase_service_key", "fake-key")

    captured = {}

    def fake_create_client(url, key, options=None):
        captured["options"] = options
        return object()

    monkeypatch.setattr(sc, "create_client", fake_create_client)

    sc.get_supabase()

    assert captured["options"].httpx_client is sc._http_client


def test_get_supabase_is_a_singleton(monkeypatch):
    _reset_singleton(monkeypatch)
    monkeypatch.setattr(sc.settings, "supabase_url", "https://example.supabase.co")
    monkeypatch.setattr(sc.settings, "supabase_service_key", "fake-key")

    calls = []
    monkeypatch.setattr(
        sc, "create_client", lambda *a, **k: calls.append(1) or object()
    )

    first = sc.get_supabase()
    second = sc.get_supabase()

    assert first is second
    assert len(calls) == 1


def test_get_fresh_client_uses_shared_http_client_but_is_a_new_wrapper(monkeypatch):
    monkeypatch.setattr(sc.settings, "supabase_url", "https://example.supabase.co")
    monkeypatch.setattr(sc.settings, "supabase_service_key", "fake-key")

    captured_options = []
    monkeypatch.setattr(
        sc,
        "create_client",
        lambda url, key, options=None: captured_options.append(options) or object(),
    )

    a = sc.get_fresh_client()
    b = sc.get_fresh_client()

    assert a is not b  # wrapper baru tiap panggilan (isolasi header auth)
    assert captured_options[0].httpx_client is sc._http_client
    assert captured_options[1].httpx_client is sc._http_client  # transport dibagi


def test_get_supabase_raises_runtime_error_when_unconfigured(monkeypatch):
    _reset_singleton(monkeypatch)
    monkeypatch.setattr(sc.settings, "supabase_url", "")
    monkeypatch.setattr(sc.settings, "supabase_service_key", "")

    import pytest

    with pytest.raises(RuntimeError):
        sc.get_supabase()
