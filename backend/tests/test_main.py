"""Galat tak terduga apa pun harus tercatat (bukan hilang begitu saja) dan
direspons dengan 500 generik, bukan membocorkan detail internal — lihat
app/main.py's unhandled_exception_handler."""

import logging

from fastapi.testclient import TestClient

from app.main import app
from app.routers import quiz as quiz_router


def test_unhandled_exception_becomes_generic_500(sb, monkeypatch, caplog):
    def boom(*args, **kwargs):
        raise ValueError("sesuatu yang tak terduga")

    monkeypatch.setattr(quiz_router, "get_supabase", boom)
    # raise_server_exceptions=False: seperti server sungguhan, biarkan handler
    # galat umum yang menangani, jangan biarkan TestClient melempar ulang.
    no_raise_client = TestClient(app, raise_server_exceptions=False)

    with caplog.at_level(logging.ERROR):
        res = no_raise_client.get("/api/quiz/available")

    assert res.status_code == 500
    assert res.json() == {"detail": "Terjadi kesalahan pada server."}
    assert any(
        "Unhandled error" in r.message and r.exc_info is not None
        for r in caplog.records
    )


def test_http_exceptions_are_unaffected(client, sb):
    """Regresi: handler galat umum tidak boleh menutupi status HTTPException
    yang sudah ada (404/401/dst.)."""
    res = client.get("/api/quiz/tidak-ada")
    assert res.status_code == 404
