"""Bantu bedakan galat PostgREST spesifik (kode error Postgres) dari galat
lain, supaya router bisa mengubahnya jadi respons HTTP yang tepat alih-alih
membiarkannya jadi 500 mentah lewat handler galat umum di main.py."""

from postgrest.exceptions import APIError

UNIQUE_VIOLATION = "23505"
FOREIGN_KEY_VIOLATION = "23503"


def is_unique_violation(exc: Exception) -> bool:
    return isinstance(exc, APIError) and exc.code == UNIQUE_VIOLATION


def is_fk_violation(exc: Exception) -> bool:
    return isinstance(exc, APIError) and exc.code == FOREIGN_KEY_VIOLATION
