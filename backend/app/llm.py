import asyncio
import json
import logging
from collections import Counter

import httpx

from .config import settings

logger = logging.getLogger(__name__)

QTYPE_LABELS = {
    "pilihan_ganda": "pilihan ganda (4 opsi: A, B, C, D)",
    "benar_salah": "benar/salah",
    "isian": "isian singkat",
    "deskripsi": "uraian/deskripsi (jawaban berupa paragraf)",
}

VALID_TYPES = {"pilihan_ganda", "benar_salah", "isian", "deskripsi"}

# Status yang layak dicoba ulang (gangguan sesaat) — bukan mis. 400/401 yang
# tak akan berhasil walau diulang.
_RETRYABLE_STATUS = {429, 500, 502, 503, 504}
_CHAT_ATTEMPTS = 2
_CHAT_RETRY_DELAY_SECONDS = 1.0


class LLMError(Exception):
    pass


def _split_counts(total: int) -> dict[str, int]:
    pg = round(total * 0.4)
    bs = round(total * 0.2)
    isian = total - pg - bs
    return {"pilihan_ganda": pg, "benar_salah": bs, "isian": isian}


def _build_prompt(
    subject: str, grade: int, counts: dict[str, int], material: str | None
) -> str:
    counts_text = ", ".join(
        f"{count} soal {QTYPE_LABELS[qtype]}"
        for qtype, count in counts.items()
        if count > 0
    )
    if material:
        materi_text = (
            "Gunakan materi ajar berikut sebagai sumber utama soal:\n\n"
            f"{material[:4000]}\n\n"
        )
    else:
        materi_text = (
            "Tidak ada materi khusus. Gunakan kurikulum sekolah Indonesia umum "
            f"untuk mata pelajaran {subject} kelas {grade}.\n\n"
        )
    return (
        f"Buat soal ujian mata pelajaran {subject} untuk kelas {grade} sekolah "
        f"Indonesia. Komposisi soal: {counts_text}.\n\n"
        f"{materi_text}"
        "Semua teks soal, opsi, jawaban, dan pembahasan HARUS dalam Bahasa Indonesia "
        "yang sesuai untuk jenjang kelas tersebut. Jika materi memuat kutipan ayat "
        "Al-Qur'an, Hadis, atau istilah/frasa berbahasa Arab, PERTAHANKAN teks "
        "Arabnya persis apa adanya (jangan ditransliterasi ke huruf Latin, jangan "
        "hanya diterjemahkan tanpa teks aslinya); sertakan juga arti/terjemahannya "
        "dalam Bahasa Indonesia bila relevan.\n\n"
        "Jika soal memuat rumus, persamaan, pecahan, pangkat, akar, atau notasi "
        "matematika lain, tulis menggunakan LaTeX: gunakan $...$ untuk notasi "
        "sebaris (contoh: $x^2 + 1$) dan $$...$$ untuk persamaan berdiri sendiri "
        "(contoh: $$\\frac{a}{b} = c$$). Ini boleh muncul di pertanyaan, opsi, "
        "maupun pembahasan. Selain notasi matematika ini, jangan gunakan format "
        "markdown lain (tanpa bold, tanpa list, tanpa heading) — teks biasa saja.\n\n"
        "Untuk SEBAGIAN KECIL soal saja (jangan berlebihan) di mana gambar benar-benar "
        "diperlukan agar soal bisa dipahami/dijawab (mis. diagram geometri, peta, grafik, "
        "atau mengenali objek/hewan/tumbuhan/tempat nyata), tambahkan dua field berikut "
        "pada objek soal itu:\n"
        '- "gambar_tipe": "generated" jika gambar berupa ilustrasi/diagram yang perlu '
        "dibuat (sertakan juga \"gambar_prompt\": deskripsi singkat gambar yang harus dibuat), "
        'atau "gambar_tipe": "stock" jika yang dibutuhkan adalah foto benda/tempat/makhluk '
        'nyata (sertakan juga "gambar_cari": kata kunci pencarian foto singkat dalam Bahasa '
        "Inggris).\n"
        "Soal lain yang tidak butuh gambar TIDAK PERLU menyertakan field ini sama sekali.\n\n"
        "Balas HANYA dengan JSON valid (tanpa teks lain) dengan format:\n"
        '{"questions": [\n'
        '  {"tipe": "pilihan_ganda", "pertanyaan": "...", "opsi": ["...", "...", "...", "..."], '
        '"jawaban": 0, "pembahasan": "..."},\n'
        '  {"tipe": "benar_salah", "pertanyaan": "...", "jawaban": "benar" atau "salah", "pembahasan": "..."},\n'
        '  {"tipe": "isian", "pertanyaan": "...", "jawaban": "jawaban singkat", "pembahasan": "...", '
        '"gambar_tipe": "stock", "gambar_cari": "..."}\n'
        '  {"tipe": "deskripsi", "pertanyaan": "...", "jawaban": "uraian jawaban model berupa beberapa kalimat", "pembahasan": "..."}\n'
        "]}\n\n"
        "Untuk pilihan_ganda, jawaban adalah indeks opsi yang benar (0-3). "
        "Untuk deskripsi, jawaban adalah jawaban model berupa uraian lengkap "
        "(beberapa kalimat) yang memuat seluruh poin penting yang diharapkan dari siswa. "
        "Pembahasan harus menjelaskan mengapa jawaban tersebut benar."
    )


def _has_balanced_math_delimiters(text: str) -> bool:
    """Count unescaped '$' (LaTeX math delimiters) — must be even to be balanced."""
    count = 0
    escaped = False
    for ch in text:
        if escaped:
            escaped = False
            continue
        if ch == "\\":
            escaped = True
        elif ch == "$":
            count += 1
    return count % 2 == 0


def _validate_questions(data: dict, counts: dict[str, int]) -> list[dict]:
    if not isinstance(data, dict) or not isinstance(data.get("questions"), list):
        raise ValueError("JSON tidak berisi daftar questions")
    questions = data["questions"]
    expected = {t: c for t, c in counts.items() if c > 0}
    total = sum(expected.values())
    actual = Counter(
        q.get("tipe") for q in questions if isinstance(q, dict) and q.get("tipe")
    )
    if len(questions) != total or dict(actual) != expected:
        raise ValueError(
            f"Jumlah/komposisi soal tidak sesuai: dapat {dict(actual) or len(questions)} "
            f"soal, seharusnya {expected} (total {total})"
        )
    for i, q in enumerate(questions):
        tipe = q.get("tipe")
        if tipe not in VALID_TYPES:
            raise ValueError(f"Soal {i}: tipe tidak valid: {tipe}")
        if not q.get("pertanyaan") or not q.get("pembahasan"):
            raise ValueError(f"Soal {i}: pertanyaan/pembahasan kosong")
        text_fields = [q["pertanyaan"], q["pembahasan"]]
        if tipe == "pilihan_ganda":
            opsi = q.get("opsi")
            if not isinstance(opsi, list) or len(opsi) != 4:
                raise ValueError(f"Soal {i}: opsi harus 4 item")
            if not isinstance(q.get("jawaban"), int) or not 0 <= q["jawaban"] <= 3:
                raise ValueError(f"Soal {i}: jawaban harus indeks 0-3")
            text_fields.extend(opsi)
        elif tipe == "benar_salah":
            if q.get("jawaban") not in ("benar", "salah"):
                raise ValueError(f"Soal {i}: jawaban harus 'benar' atau 'salah'")
        elif tipe == "isian":
            if not isinstance(q.get("jawaban"), str) or not q.get("jawaban").strip():
                raise ValueError(f"Soal {i}: jawaban isian kosong")
        elif tipe == "deskripsi":
            if not isinstance(q.get("jawaban"), str) or not q.get("jawaban").strip():
                raise ValueError(f"Soal {i}: jawaban deskripsi kosong")
        for field in text_fields:
            if isinstance(field, str) and not _has_balanced_math_delimiters(field):
                raise ValueError(f"Soal {i}: delimiter $ tidak seimbang")
        gambar_tipe = q.get("gambar_tipe")
        if gambar_tipe is not None:
            if gambar_tipe not in ("generated", "stock"):
                raise ValueError(f"Soal {i}: gambar_tipe tidak valid: {gambar_tipe}")
            field_name = "gambar_prompt" if gambar_tipe == "generated" else "gambar_cari"
            if not isinstance(q.get(field_name), str) or not q[field_name].strip():
                raise ValueError(f"Soal {i}: {field_name} kosong")
    return questions


async def _chat(messages: list[dict]) -> str:
    """Panggil OpenRouter. Percobaan ulang (retry) hanya untuk gangguan
    sesaat — koneksi terputus/timeout, atau status 429/5xx — bukan untuk
    galat yang pasti akan gagal lagi (mis. API key salah)."""
    if not settings.openrouter_api_key:
        raise LLMError("OPENROUTER_API_KEY belum diatur di server")
    headers = {
        "Authorization": f"Bearer {settings.openrouter_api_key}",
        "Content-Type": "application/json",
    }
    payload = {
        "model": settings.openrouter_model,
        "messages": messages,
        "temperature": 0.7,
        "response_format": {"type": "json_object"},
    }
    async with httpx.AsyncClient(timeout=180) as client:
        for attempt in range(_CHAT_ATTEMPTS):
            is_last = attempt == _CHAT_ATTEMPTS - 1
            try:
                resp = await client.post(
                    settings.openrouter_url, headers=headers, json=payload
                )
            except httpx.RequestError as e:
                logger.warning(
                    "Koneksi ke OpenRouter gagal (percobaan %d): %s", attempt + 1, e
                )
                if is_last:
                    raise LLMError(
                        "Gagal terhubung ke layanan AI. Coba lagi nanti."
                    ) from e
                await asyncio.sleep(_CHAT_RETRY_DELAY_SECONDS)
                continue

            if resp.status_code == 200:
                break

            logger.error("OpenRouter error %s: %s", resp.status_code, resp.text[:500])
            if resp.status_code in _RETRYABLE_STATUS and not is_last:
                await asyncio.sleep(_CHAT_RETRY_DELAY_SECONDS)
                continue
            raise LLMError("Gagal menghubungi layanan AI. Coba lagi nanti.")

        try:
            content = resp.json()["choices"][0]["message"]["content"]
        except (KeyError, IndexError, ValueError) as e:
            logger.error("Respons OpenRouter tidak terduga: %s", e)
            raise LLMError("Respons AI tidak valid. Coba lagi nanti.") from e
        if not content:
            raise LLMError("Respons AI kosong. Coba lagi nanti.")
        return content


async def generate_quiz(
    subject: str, grade: int, counts: dict[str, int], material: str | None
) -> list[dict]:
    prompt = _build_prompt(subject, grade, counts, material)
    total = sum(c for c in counts.values() if c > 0)
    messages = [
        {
            "role": "system",
            "content": (
                "Anda pembuat soal ujian sekolah Indonesia. "
                "Anda selalu menjawab dengan JSON valid saja."
            ),
        },
        {"role": "user", "content": prompt},
    ]
    last_error = None
    for attempt in range(2):
        content = await _chat(messages)
        try:
            data = json.loads(content)
            return _validate_questions(data, counts)
        except (json.JSONDecodeError, ValueError) as e:
            last_error = e
            logger.warning("Validasi JSON soal gagal (percobaan %d): %s", attempt + 1, e)
            messages = messages[:2] + [
                {"role": "assistant", "content": content},
                {
                    "role": "user",
                    "content": (
                        f"JSON Anda tidak valid: {e}. "
                        "Perbaiki dan balas HANYA JSON valid sesuai format, "
                        f"tepat {total} soal."
                    ),
                },
            ]
    raise LLMError(
        "Gagal membuat soal setelah beberapa percobaan. Coba lagi beberapa saat lagi."
    ) from last_error


async def grade_short_answers(
    items: list[dict],
) -> dict[int, dict]:
    """items: [{index, tipe, pertanyaan, jawaban_model, jawaban_siswa}]

    Returns {index: {verdict, skor, umpan_balik}}.
    """
    if not items:
        return {}
    prompt = (
        "Anda guru yang mengoreksi jawaban siswa sekolah Indonesia. "
        "Untuk setiap item, nilai jawaban siswa dibanding jawaban model:\n"
        '- "benar" (skor 1.0): inti jawaban tepat\n'
        '- "parsial" (skor 0.1-0.9): sebagian benar\n'
        '- "salah" (skor 0.0): salah atau kosong\n'
        "Untuk soal isian singkat, toleransi kecil pada salah ketik/ejaan. "
        "Untuk soal uraian/deskripsi, nilai kelengkapan dan ketepatan isi — "
        "jawaban siswa tidak harus sama kata per kata dengan jawaban model; "
        "jawaban kosong atau tidak relevan bernilai salah.\n"
        "Umpan balik dalam Bahasa Indonesia: singkat untuk isian singkat; "
        "untuk uraian/deskripsi berisi koreksi AI — sebutkan poin/ide yang sudah "
        "tepat dari jawaban siswa dan poin penting dari jawaban model yang belum "
        "atau kurang dicantumkan (1-3 kalimat).\n\n"
        "Item jawaban:\n"
        f"{json.dumps(items, ensure_ascii=False)}\n\n"
        "Balas HANYA JSON valid:\n"
        '{"hasil": [{"index": 0, "verdict": "benar|parsial|salah", "skor": 0.0, '
        '"umpan_balik": "..."}]}'
    )
    messages = [
        {
            "role": "system",
            "content": (
                "Anda korektor jawaban siswa. Anda selalu menjawab dengan JSON valid saja."
            ),
        },
        {"role": "user", "content": prompt},
    ]
    last_error = None
    for attempt in range(2):
        content = await _chat(messages)
        try:
            data = json.loads(content)
            hasil = data["hasil"]
            if not isinstance(hasil, list):
                raise ValueError("hasil bukan daftar")
            result: dict[int, dict] = {}
            for h in hasil:
                index = h.get("index")
                verdict = h.get("verdict")
                skor = h.get("skor")
                if (
                    not isinstance(index, int)
                    or verdict not in ("benar", "parsial", "salah")
                    or not isinstance(skor, (int, float))
                ):
                    raise ValueError(f"Item hasil tidak valid: {h}")
                result[index] = {
                    "verdict": verdict,
                    "skor": max(0.0, min(1.0, float(skor))),
                    "umpan_balik": h.get("umpan_balik", ""),
                }
            missing = [it["index"] for it in items if it["index"] not in result]
            if missing:
                raise ValueError(f"Hasil kurang untuk index: {missing}")
            return result
        except (json.JSONDecodeError, ValueError, KeyError, TypeError) as e:
            last_error = e
            logger.warning("Validasi JSON penilaian gagal (percobaan %d): %s", attempt + 1, e)
            messages = messages[:2] + [
                {"role": "assistant", "content": content},
                {
                    "role": "user",
                    "content": (
                        f"JSON Anda tidak valid: {e}. Balas HANYA JSON valid "
                        "dengan hasil untuk SEMUA index jawaban."
                    ),
                },
            ]
    raise LLMError(
        "Gagal mengoreksi jawaban isian. Coba kumpulkan ulang beberapa saat lagi."
    ) from last_error