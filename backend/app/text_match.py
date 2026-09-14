"""Pencocokan teks sederhana untuk menilai jawaban isian singkat tanpa AI.

Soal isian dirancang punya SATU jawaban model pendek (lihat _build_prompt di
llm.py: "jawaban singkat"), dan koreksinya memang dimaksud hanya bertoleransi
kecil pada salah ketik/ejaan — bukan menilai kelengkapan/parafrase seperti
uraian/deskripsi. Perbandingan string ternormalisasi mengerjakan itu secara
instan dan konsisten, tanpa perlu memanggil AI sama sekali. Ini yang membuat
grade_short_answers() di llm.py cuma perlu menilai soal deskripsi — batch
yang dikirim ke AI jadi jauh lebih kecil dan pengumpulan jauh lebih cepat.
"""

import math
import re
import unicodedata
from difflib import SequenceMatcher

# Rasio kemiripan (0-1) minimum supaya dianggap benar — cukup longgar untuk
# menoleransi 1-2 salah ketik pada jawaban pendek, cukup ketat untuk tetap
# menolak jawaban yang sungguh berbeda.
MATCH_THRESHOLD = 0.85

# "1/2", "-3/4" — pecahan sederhana.
_FRACTION_RE = re.compile(r"-?\d+\s*/\s*\d+")
# Hanya digit, titik, koma, minus — kalau tidak murni ini, jangan dipaksa
# diperlakukan sebagai angka (mis. "sekitar 100 orang" tetap lewat jalur teks).
_NUMERIC_RE = re.compile(r"-?[\d.,]+")

# Tanda baca ASCII biasa + varian "pintar" yang sering disisipkan otomatis
# oleh keyboard HP (kutip miring ‘’“”, garis pisah – —) — semuanya harus
# gugur sebelum dibandingkan, bukan cuma bentuk ASCII-nya.
_PUNCTUATION_RE = re.compile("[.,!?;:'\"()\\[\\]{}‘’“”\\-–—]")
_WHITESPACE_RE = re.compile(r"\s+")


def _normalize(text: str) -> str:
    # NFKC dulu: dua string yang tampak identik bisa berbeda secara byte
    # (huruf beraksen sebagai satu kode vs huruf+aksen terpisah, angka/huruf
    # lebar-penuh dari sebagian keyboard mobile) — disamakan SEBELUM
    # dibandingkan, supaya jawaban yang sebenarnya sama tidak dianggap beda
    # hanya karena representasi Unicode-nya berbeda.
    text = unicodedata.normalize("NFKC", text)
    text = text.strip().casefold()
    text = _PUNCTUATION_RE.sub("", text)
    text = _WHITESPACE_RE.sub(" ", text)
    return text.strip()


def _try_parse_number(text: str) -> float | None:
    """Coba baca teks sebagai angka — pecahan ("1/2") atau desimal, dengan
    notasi Indonesia (titik ribuan, koma desimal: "1.000", "-0,2") maupun
    notasi internasional ("0.25") — atau None kalau bukan murni angka.

    PENTING: ini harus dicoba SEBELUM _normalize()/pencocokan string, karena
    _normalize() membuang tanda titik/koma/minus (dianggap tanda baca biasa)
    — itu benar untuk teks, tapi merusak makna angka ("-0,2" jadi "02",
    kehilangan tanda minus DAN titik desimalnya sekaligus).
    """
    text = unicodedata.normalize("NFKC", text).strip()
    if not text:
        return None

    frac = _FRACTION_RE.fullmatch(text)
    if frac:
        num_str, den_str = frac.group().split("/")
        den = int(den_str)
        return int(num_str) / den if den != 0 else None

    if not _NUMERIC_RE.fullmatch(text):
        return None  # bukan murni angka (mis. ada huruf) — lewat jalur teks

    has_comma, has_dot = "," in text, "." in text
    if has_comma and has_dot:
        # Pemisah yang muncul TERAKHIR adalah desimal; sisanya pemisah ribuan.
        # "1.234,56" (ID) -> koma terakhir -> desimal. "1,234.56" (US) -> titik.
        if text.rfind(",") > text.rfind("."):
            normalized = text.replace(".", "").replace(",", ".")
        else:
            normalized = text.replace(",", "")
    elif has_comma:
        # Cuma koma: konvensi Indonesia untuk ribuan adalah titik, bukan
        # koma — jadi koma di sini hampir selalu desimal ("-0,2" -> -0.2).
        normalized = text.replace(",", ".")
    elif has_dot:
        # Cuma titik: ambigu antara ribuan ("1.000" ala Indonesia) dan
        # desimal ("0.25" ala internasional). Heuristik: diawali "0." atau
        # "-0." (tak seorang pun menulis ribuan berawalan nol), atau bagian
        # setelah titik BUKAN persis 3 digit (pemisah ribuan grup 3 digit),
        # atau ada lebih dari satu titik yang bukan grup ribuan — dianggap
        # desimal; selain itu (mis. "1.000", "12.345") dianggap ribuan.
        last_group = text.rsplit(".", 1)[-1]
        looks_decimal = (
            text.startswith("0.")
            or text.startswith("-0.")
            or len(last_group) != 3
        )
        normalized = text if looks_decimal else text.replace(".", "")
    else:
        normalized = text

    try:
        return float(normalized)
    except ValueError:
        return None


def grade_isian(jawaban_siswa: str, jawaban_benar: str) -> tuple[str, float]:
    """Bandingkan jawaban isian siswa dengan jawaban model.

    Mengembalikan (verdict, skor). Tidak ada nilai parsial di sini — isian
    adalah ingatan faktual singkat, bukan sesuatu yang wajar dinilai
    "separuh benar" dari kemiripan string semata (beda dengan uraian).
    """
    siswa_num = _try_parse_number(jawaban_siswa)
    benar_num = _try_parse_number(str(jawaban_benar))
    if siswa_num is not None and benar_num is not None:
        # Angka dibandingkan sebagai nilai, bukan string — "1/2", "0.5", dan
        # "0,5" harus dianggap sama. Toleransi hanya untuk presisi floating
        # point (mis. hasil bagi pecahan), BUKAN pembulatan siswa — 0.24
        # tetap salah kalau jawaban benarnya 0.25.
        if math.isclose(siswa_num, benar_num, rel_tol=1e-9, abs_tol=1e-9):
            return "benar", 1.0
        return "salah", 0.0

    a = _normalize(jawaban_siswa)
    b = _normalize(str(jawaban_benar))
    if not a:
        return "salah", 0.0
    if a == b:
        return "benar", 1.0
    ratio = SequenceMatcher(None, a, b).ratio()
    if ratio >= MATCH_THRESHOLD:
        return "benar", 1.0
    return "salah", 0.0
