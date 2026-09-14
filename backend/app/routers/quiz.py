import logging
import random
import uuid
from datetime import datetime, timedelta, timezone

from fastapi import APIRouter, Depends, HTTPException, Request, Response
from pydantic import BaseModel, Field, field_validator

from ..db_errors import is_unique_violation
from ..grade_config import get_config
from ..image_gen import ImageGenError, generate_image
from ..image_search import search_stock_image
from ..image_store import upload_image_bytes
from ..llm import LLMError, _split_counts, generate_quiz, grade_short_answers
from ..text_match import grade_isian
from ..subjects import VALID_GRADES
from ..supabase_client import get_supabase
from .materials import require_admin

logger = logging.getLogger(__name__)

router = APIRouter(prefix="/api/quiz", tags=["quiz"])

CLIENT_COOKIE = "client_id"
IMAGE_CAP_PER_PAKET = 3


async def _resolve_images(sb, questions: list[dict]) -> None:
    """Ubah penanda gambar_tipe/gambar_prompt/gambar_cari dari LLM menjadi
    field 'gambar' berisi URL nyata. Gagal per soal tidak boleh menggagalkan
    seluruh batch — soal tersebut cukup tidak punya gambar."""
    resolved = 0
    for q in questions:
        tipe = q.pop("gambar_tipe", None)
        prompt = q.pop("gambar_prompt", None)
        query = q.pop("gambar_cari", None)
        if tipe is None or resolved >= IMAGE_CAP_PER_PAKET:
            continue
        try:
            if tipe == "generated" and prompt:
                data, content_type = await generate_image(prompt)
                q["gambar"] = upload_image_bytes(sb, data, content_type)
                resolved += 1
            elif tipe == "stock" and query:
                data = await search_stock_image(query)
                if data:
                    q["gambar"] = upload_image_bytes(sb, data, "image/jpeg")
                    resolved += 1
        except ImageGenError as e:
            logger.warning("Gagal membuat gambar soal: %s", e)


class GenerateRequest(BaseModel):
    subject: str
    grade: int
    exam_type_id: str
    jumlah_paket: int = Field(default=3, ge=1, le=5)


class QuizRequest(BaseModel):
    subject: str
    grade: int
    exam_type_id: str
    served_ids: list[str] = Field(default_factory=list, max_length=100)


# Batas wajar untuk jawaban dari endpoint anonim ini — jumlah soal maksimum
# sebuah tipe ujian sendiri dibatasi 50 (lihat exam_types.py), dan nilai isian
# ikut masuk mentah ke prompt koreksi AI (llm.py grade_short_answers), jadi
# payload tak berbatas bisa membengkakkan biaya/waktu panggilan AI.
MAX_ANSWER_KEYS = 100
MAX_ANSWER_VALUE_LENGTH = 5000


class SubmitRequest(BaseModel):
    answers: dict[str, object]

    @field_validator("answers")
    @classmethod
    def _bound_answers(cls, v: dict[str, object]) -> dict[str, object]:
        if len(v) > MAX_ANSWER_KEYS:
            raise ValueError(f"Jumlah jawaban melebihi batas ({MAX_ANSWER_KEYS}).")
        for value in v.values():
            if value is None:
                continue
            if isinstance(value, str):
                if len(value) > MAX_ANSWER_VALUE_LENGTH:
                    raise ValueError("Salah satu jawaban terlalu panjang.")
            elif not isinstance(value, (int, float, bool)):
                raise ValueError("Tipe jawaban tidak valid.")
        return v


class PoolResetRequest(BaseModel):
    subject: str
    grade: int
    exam_type_id: str


def _validate_subject_grade(sb, subject: str, grade: int):
    res = sb.table("subjects").select("id").eq("name", subject).execute()
    if not res.data:
        raise HTTPException(status_code=422, detail="Mata pelajaran tidak valid")
    if grade not in VALID_GRADES:
        raise HTTPException(status_code=422, detail="Kelas tidak valid")


def _get_exam_type(sb, exam_type_id: str) -> dict:
    res = sb.table("exam_types").select("*").eq("id", exam_type_id).execute()
    if not res.data:
        raise HTTPException(status_code=422, detail="Tipe ujian tidak valid")
    return res.data[0]


def _strip_questions(questions: list[dict]) -> list[dict]:
    """Remove answers/explanations — these never leave the backend."""
    stripped = []
    for i, q in enumerate(questions):
        out = {
            "nomor": i + 1,
            "tipe": q["tipe"],
            "pertanyaan": q["pertanyaan"],
        }
        if q["tipe"] == "pilihan_ganda":
            out["opsi"] = q["opsi"]
        if q.get("gambar"):
            out["gambar"] = q["gambar"]
        stripped.append(out)
    return stripped


def _exam_type_row(sb, quiz: dict) -> dict | None:
    res = (
        sb.table("exam_types")
        .select("*")
        .eq("id", quiz["exam_type_id"])
        .execute()
    )
    return res.data[0] if res.data else None


def _quiz_public(sb, quiz: dict, attempt: dict) -> dict:
    row = _exam_type_row(sb, quiz)
    return {
        "quiz_id": quiz["id"],
        "questions": _strip_questions(quiz["questions"]),
        "subject": quiz["subject"],
        "grade": quiz["grade"],
        "exam_type": (row or {}).get("name", ""),
        "durasi_menit": quiz.get("durasi_menit"),
        "expires_at": attempt.get("expires_at"),
    }


def _get_attempt(sb, quiz_id: str, client_id: str) -> dict | None:
    res = (
        sb.table("attempts")
        .select("*")
        .eq("quiz_id", quiz_id)
        .eq("client_id", client_id)
        .execute()
    )
    return res.data[0] if res.data else None


def _create_attempt(sb, quiz: dict, client_id: str, expires_at: datetime | None = None) -> dict:
    """Buat attempt (dan mulai timer) untuk pasangan (quiz, client) ini saja —
    paket yang sama tetap tersedia untuk siswa lain karena timer tidak
    disimpan di baris quiz."""
    if expires_at is None:
        durasi = quiz.get("durasi_menit") or get_config(quiz["grade"])["durasi_menit"]
        expires_at = datetime.now(timezone.utc) + timedelta(minutes=durasi)
    if not quiz.get("started"):
        sb.table("quizzes").update({"started": True}).eq("id", quiz["id"]).execute()
        quiz["started"] = True
    try:
        res = (
            sb.table("attempts")
            .insert(
                {
                    "quiz_id": quiz["id"],
                    "client_id": client_id,
                    "expires_at": expires_at.isoformat(),
                    "expired": False,
                }
            )
            .execute()
        )
        return res.data[0]
    except Exception as e:
        # Dua request nyaris bersamaan (double-click, dua tab, retry klien)
        # bisa lolos _get_attempt() yang sama-sama tidak menemukan baris, lalu
        # berlomba insert — idx_attempts_quiz_client di DB menolak yang kalah.
        # Alih-alih 500 mentah, ambil baris milik pemenangnya.
        if not is_unique_violation(e):
            raise
        winner = _get_attempt(sb, quiz["id"], client_id)
        if winner is None:
            raise
        return winner


def _get_or_create_attempt(sb, quiz: dict, client_id: str) -> dict:
    return _get_attempt(sb, quiz["id"], client_id) or _create_attempt(sb, quiz, client_id)


def _ensure_client_id(request: Request, response: Response) -> str:
    """ID anonim per perangkat: dibuat sekali, disimpan sebagai httpOnly cookie."""
    client_id = request.cookies.get(CLIENT_COOKIE)
    if not client_id:
        client_id = str(uuid.uuid4())
    response.set_cookie(
        CLIENT_COOKIE,
        client_id,
        max_age=60 * 60 * 24 * 365,  # 1 tahun (maks browser umumnya 400 hari)
        httponly=True,
        samesite="lax",
    )
    return client_id


@router.post("/generate")
async def generate(body: GenerateRequest, admin: dict = Depends(require_admin)):
    """Admin: buat batch paket soal untuk kombinasi mapel+kelas+tipe ujian."""
    sb = get_supabase()
    _validate_subject_grade(sb, body.subject, body.grade)
    exam_type = _get_exam_type(sb, body.exam_type_id)
    cfg = get_config(body.grade)
    total = exam_type.get("jumlah_soal") or cfg["jumlah_soal"]
    durasi = exam_type.get("durasi_menit") or cfg["durasi_menit"]

    # Komposisi tipe soal dari tipe ujian; NULL = komposisi otomatis
    tipe_soal = exam_type.get("tipe_soal") or None
    counts: dict[str, int]
    if tipe_soal:
        counts = {t: int(n) for t, n in tipe_soal.items() if n}
        if counts:
            total = sum(counts.values())
        else:
            counts = _split_counts(total)
    else:
        counts = _split_counts(total)

    materials = (
        sb.table("materials")
        .select("title, content")
        .eq("subject", body.subject)
        .eq("grade", body.grade)
        .eq("exam_type_id", body.exam_type_id)
        .execute()
    )
    material_text = None
    if materials.data:
        material_text = "\n\n".join(
            f"{m['title']}:\n{m['content']}" for m in materials.data
        )

    batch_id = str(uuid.uuid4())
    created = []
    for _ in range(body.jumlah_paket):
        try:
            questions = await generate_quiz(
                body.subject, body.grade, counts, material_text
            )
        except LLMError as e:
            if not created:
                raise HTTPException(status_code=502, detail=str(e))
            break  # paket yang sudah jadi tetap tersimpan sebagai pool
        await _resolve_images(sb, questions)
        res = (
            sb.table("quizzes")
            .insert(
                {
                    "subject": body.subject,
                    "grade": body.grade,
                    "exam_type_id": body.exam_type_id,
                    "questions": questions,
                    "batch_id": batch_id,
                    "durasi_menit": durasi,
                    "started": False,
                    "expires_at": None,
                }
            )
            .execute()
        )
        created.append(res.data[0])

    return {
        "generated": len(created),
        "quiz_ids": [q["id"] for q in created],
    }


@router.post("/request")
async def request_quiz(body: QuizRequest, response: Response, request: Request):
    """Siswa: terima satu paket acak dari pool yang sudah dibuat admin."""
    sb = get_supabase()
    client_id = _ensure_client_id(request, response)
    _validate_subject_grade(sb, body.subject, body.grade)
    _get_exam_type(sb, body.exam_type_id)
    served = set(body.served_ids)

    pool = (
        sb.table("quizzes")
        .select("*")
        .eq("subject", body.subject)
        .eq("grade", body.grade)
        .eq("exam_type_id", body.exam_type_id)
        .execute()
        .data
        or []
    )
    if not pool:
        raise HTTPException(
            status_code=404,
            detail=(
                "Belum ada paket soal tersedia untuk kombinasi ini. "
                "Hubungi guru/admin."
            ),
        )
    # Paket yang sudah dibuka siswa lain tetap boleh diberikan ke siswa ini —
    # yang tidak boleh diulang hanya paket yang SUDAH pernah dibuka siswa ini.
    own_attempts = (
        sb.table("attempts").select("quiz_id").eq("client_id", client_id).execute().data
        or []
    )
    taken = served | {a["quiz_id"] for a in own_attempts}
    available = [q for q in pool if q["id"] not in taken]
    chosen = random.choice(available or pool)
    attempt = _get_or_create_attempt(sb, chosen, client_id)
    return {**_quiz_public(sb, chosen, attempt), "repeat": not bool(available)}


@router.get("/available")
def available():
    """Kombinasi mapel+kelas+tipe ujian yang punya paket soal ter-generate."""
    sb = get_supabase()
    quizzes = (
        sb.table("quizzes")
        .select("subject, grade, exam_type_id, started")
        .execute()
        .data
        or []
    )
    types = sb.table("exam_types").select("id, name").execute().data or []
    names = {t["id"]: t["name"] for t in types}

    agg: dict[tuple, dict] = {}
    for q in quizzes:
        key = (q["subject"], q["grade"], q["exam_type_id"])
        entry = agg.setdefault(key, {"unstarted": 0, "total": 0})
        entry["total"] += 1
        if not q.get("started"):
            entry["unstarted"] += 1

    return [
        {
            "subject": subject,
            "grade": grade,
            "exam_type_id": exam_type_id,
            "exam_type": names.get(exam_type_id, ""),
            "unstarted": entry["unstarted"],
            "total": entry["total"],
        }
        for (subject, grade, exam_type_id), entry in sorted(agg.items())
    ]


@router.get("/attempts")
def list_attempts(response: Response, request: Request):
    """Riwayat attempt per perangkat (identifikasi lewat cookie client_id)."""
    sb = get_supabase()
    client_id = _ensure_client_id(request, response)
    attempts = (
        sb.table("attempts")
        .select("*")
        .eq("client_id", client_id)
        .order("submitted_at", desc=True)
        .execute()
        .data
        or []
    )
    attempts = [a for a in attempts if a.get("submitted_at")]
    quizzes = (
        sb.table("quizzes").select("id, subject, grade, exam_type_id").execute().data
        or []
    )
    quiz_map = {q["id"]: q for q in quizzes}
    exam_names = {
        t["id"]: t["name"]
        for t in (sb.table("exam_types").select("id, name").execute().data or [])
    }
    return [
        {
            "quiz_id": a["quiz_id"],
            "subject": quiz_map.get(a["quiz_id"], {}).get("subject", ""),
            "grade": quiz_map.get(a["quiz_id"], {}).get("grade", 0),
            "exam_type": exam_names.get(
                quiz_map.get(a["quiz_id"], {}).get("exam_type_id", ""), ""
            ),
            "nilai": (a.get("score") or {}).get("nilai", 0),
            "poin": (a.get("score") or {}).get("poin"),
            "poin_maks": (a.get("score") or {}).get("poin_maks"),
            "per_question": (a.get("score") or {}).get("per_question", []),
            "expired": a.get("expired", False),
            "submitted_at": a.get("submitted_at"),
        }
        for a in attempts
    ]


# ============================================================
# Manajemen kuis (admin) — daftar, detail lengkap, hapus paket
# ============================================================


@router.get("/admin/list")
def admin_list_quizzes(
    subject: str | None = None,
    grade: int | None = None,
    exam_type_id: str | None = None,
    admin: dict = Depends(require_admin),
):
    """Admin: daftar semua paket soal (metadata saja, tanpa isi soal)."""
    sb = get_supabase()
    query = sb.table("quizzes").select("*").order("created_at", desc=True)
    if subject:
        query = query.eq("subject", subject)
    if grade is not None:
        query = query.eq("grade", grade)
    if exam_type_id:
        query = query.eq("exam_type_id", exam_type_id)
    quizzes = query.execute().data or []
    names = {
        t["id"]: t["name"]
        for t in (sb.table("exam_types").select("id, name").execute().data or [])
    }
    return [
        {
            "id": q["id"],
            "subject": q["subject"],
            "grade": q["grade"],
            "exam_type_id": q["exam_type_id"],
            "exam_type": names.get(q["exam_type_id"], ""),
            "jumlah_soal": len(q.get("questions") or []),
            "started": bool(q.get("started")),
            "durasi_menit": q.get("durasi_menit"),
            "batch_id": q.get("batch_id"),
            "created_at": q.get("created_at"),
        }
        for q in quizzes
    ]


@router.get("/admin/quizzes/{quiz_id}")
def admin_get_quiz(quiz_id: str, admin: dict = Depends(require_admin)):
    """Admin: detail paket lengkap — termasuk kunci jawaban & pembahasan."""
    sb = get_supabase()
    res = sb.table("quizzes").select("*").eq("id", quiz_id).execute()
    if not res.data:
        raise HTTPException(status_code=404, detail="Paket soal tidak ditemukan")
    return _admin_quiz_payload(sb, res.data[0])


VALID_TIPE_SOAL = ("pilihan_ganda", "benar_salah", "isian", "deskripsi")


class AdminQuestionIn(BaseModel):
    """Satu soal dari admin — jawaban diterima apa adanya (int/str), divalidasi manual."""

    tipe: str
    pertanyaan: str
    opsi: list[str] | None = None
    jawaban: object = None
    pembahasan: str = ""
    gambar: str | None = None


class QuizUpdateRequest(BaseModel):
    questions: list[AdminQuestionIn]


def _validate_admin_questions(items: list[AdminQuestionIn]) -> list[dict]:
    """Validasi soal hasil edit admin — aturan sama dengan generator AI (llm.py).
    Mengembalikan daftar soal bersih siap simpan (tanpa nomor/gambar_tipe)."""
    if not items:
        raise HTTPException(
            status_code=422, detail="Paket soal harus berisi minimal 1 soal"
        )
    cleaned: list[dict] = []
    for i, item in enumerate(items, start=1):
        tipe = item.tipe
        if tipe not in VALID_TIPE_SOAL:
            raise HTTPException(
                status_code=422, detail=f"Soal {i}: tipe tidak valid: {tipe}"
            )
        pertanyaan = item.pertanyaan.strip()
        pembahasan = item.pembahasan.strip()
        if not pertanyaan or not pembahasan:
            raise HTTPException(
                status_code=422, detail=f"Soal {i}: pertanyaan/pembahasan kosong"
            )
        q: dict = {
            "tipe": tipe,
            "pertanyaan": pertanyaan,
            "pembahasan": pembahasan,
        }
        if tipe == "pilihan_ganda":
            opsi = item.opsi
            if not isinstance(opsi, list) or len(opsi) != 4 or any(
                not str(o).strip() for o in opsi
            ):
                raise HTTPException(
                    status_code=422, detail=f"Soal {i}: opsi harus 4 item dan tidak kosong"
                )
            jawaban = item.jawaban
            if isinstance(jawaban, bool) or not isinstance(jawaban, int):
                raise HTTPException(
                    status_code=422,
                    detail=f"Soal {i}: jawaban pilihan ganda harus indeks 0-3",
                )
            if not 0 <= jawaban <= 3:
                raise HTTPException(
                    status_code=422,
                    detail=f"Soal {i}: jawaban pilihan ganda harus indeks 0-3",
                )
            q["opsi"] = [str(o) for o in opsi]
            q["jawaban"] = jawaban
        elif tipe == "benar_salah":
            if item.jawaban not in ("benar", "salah"):
                raise HTTPException(
                    status_code=422,
                    detail=f"Soal {i}: jawaban harus 'benar' atau 'salah'",
                )
            q["jawaban"] = item.jawaban
        else:
            jawaban = item.jawaban
            if not isinstance(jawaban, str) or not jawaban.strip():
                raise HTTPException(
                    status_code=422, detail=f"Soal {i}: jawaban {tipe} kosong"
                )
            q["jawaban"] = jawaban.strip()
        if item.gambar and isinstance(item.gambar, str):
            q["gambar"] = item.gambar
        cleaned.append(q)
    return cleaned


def _admin_quiz_payload(sb, quiz: dict) -> dict:
    row = _exam_type_row(sb, quiz)
    questions = []
    for i, q in enumerate(quiz["questions"]):
        item = {
            "nomor": i + 1,
            "tipe": q["tipe"],
            "pertanyaan": q["pertanyaan"],
            "jawaban": q["jawaban"],
            "pembahasan": q.get("pembahasan", ""),
        }
        if q["tipe"] == "pilihan_ganda":
            item["opsi"] = q["opsi"]
        if q.get("gambar"):
            item["gambar"] = q["gambar"]
        questions.append(item)
    return {
        "id": quiz["id"],
        "subject": quiz["subject"],
        "grade": quiz["grade"],
        "exam_type_id": quiz["exam_type_id"],
        "exam_type": (row or {}).get("name", ""),
        "durasi_menit": quiz.get("durasi_menit"),
        "started": bool(quiz.get("started")),
        "batch_id": quiz.get("batch_id"),
        "created_at": quiz.get("created_at"),
        "questions": questions,
    }


@router.patch("/admin/quizzes/{quiz_id}")
def admin_update_quiz(
    quiz_id: str, body: QuizUpdateRequest, admin: dict = Depends(require_admin)
):
    """Admin: perbaiki isi paket — ganti seluruh daftar soal dengan versi baru."""
    sb = get_supabase()
    res = sb.table("quizzes").select("*").eq("id", quiz_id).execute()
    if not res.data:
        raise HTTPException(status_code=404, detail="Paket soal tidak ditemukan")
    quiz = res.data[0]
    questions = _validate_admin_questions(body.questions)
    sb.table("quizzes").update({"questions": questions}).eq("id", quiz_id).execute()
    quiz["questions"] = questions
    return _admin_quiz_payload(sb, quiz)


@router.delete("/admin/quizzes/{quiz_id}", status_code=204)
def admin_delete_quiz(quiz_id: str, admin: dict = Depends(require_admin)):
    """Admin: hapus satu paket soal. Riwayat pengerjaan paket ini ikut terhapus
    lewat "on delete cascade" attempts.quiz_id di schema.sql — hanya satu
    panggilan delete di sini, jadi tidak ada jendela di mana quiz sudah
    terhapus tapi attempts-nya tertinggal (atau sebaliknya) akibat panggilan
    kedua yang gagal."""
    sb = get_supabase()
    res = sb.table("quizzes").select("id").eq("id", quiz_id).execute()
    if not res.data:
        raise HTTPException(status_code=404, detail="Paket soal tidak ditemukan")
    sb.table("quizzes").delete().eq("id", quiz_id).execute()
    return None


@router.get("/{quiz_id}")
def get_quiz(quiz_id: str, response: Response, request: Request):
    sb = get_supabase()
    client_id = _ensure_client_id(request, response)
    res = sb.table("quizzes").select("*").eq("id", quiz_id).execute()
    if not res.data:
        raise HTTPException(status_code=404, detail="Kuis tidak ditemukan")
    quiz = res.data[0]
    attempt = _get_or_create_attempt(sb, quiz, client_id)
    return _quiz_public(sb, quiz, attempt)


@router.post("/pool/reset")
def reset_pool(body: PoolResetRequest, admin: dict = Depends(require_admin)):
    sb = get_supabase()
    _validate_subject_grade(sb, body.subject, body.grade)
    _get_exam_type(sb, body.exam_type_id)
    removed = (
        sb.table("quizzes")
        .delete()
        .eq("subject", body.subject)
        .eq("grade", body.grade)
        .eq("exam_type_id", body.exam_type_id)
        .eq("started", False)
        .execute()
    )
    return {"deleted": len(removed.data)}


@router.post("/{quiz_id}/submit")
async def submit(
    quiz_id: str, body: SubmitRequest, response: Response, request: Request
):
    sb = get_supabase()
    client_id = _ensure_client_id(request, response)
    res = sb.table("quizzes").select("*").eq("id", quiz_id).execute()
    if not res.data:
        raise HTTPException(status_code=404, detail="Kuis tidak ditemukan")
    quiz = res.data[0]
    questions: list[dict] = quiz["questions"]
    exam_row = _exam_type_row(sb, quiz)

    attempt = _get_attempt(sb, quiz_id, client_id)
    if attempt is None:
        # Kirim jawaban tanpa pernah membuka kuis: mulai dan langsung kedaluwarsa
        attempt = _create_attempt(sb, quiz, client_id, expires_at=datetime.now(timezone.utc))

    # Idempoten: paket yang sudah dikumpulkan mengembalikan hasil yang tersimpan,
    # tanpa menilai ulang. Tanpa ini, percobaan ulang dari browser (koneksi putus
    # saat respons dikirim) akan memanggil AI lagi dan menimpa nilai yang sudah ada.
    if attempt.get("submitted_at") and attempt.get("score"):
        score = attempt["score"]
        return {
            "quiz_id": quiz_id,
            "subject": quiz["subject"],
            "grade": quiz["grade"],
            "exam_type": (exam_row or {}).get("name", ""),
            "nilai": score.get("nilai", 0),
            "poin": score.get("poin"),
            "poin_maks": score.get("poin_maks"),
            "per_question": score.get("per_question", []),
            "expired": attempt.get("expired", False),
        }

    now = datetime.now(timezone.utc)
    expires_at = datetime.fromisoformat(attempt["expires_at"])
    expired = expires_at < now

    # Isian dinilai lokal (perbandingan string, lihat text_match.py) —
    # cuma soal deskripsi/uraian yang sungguh butuh penilaian AI (kelengkapan
    # & ketepatan isi, bukan sekadar cocok-tidaknya string). Ini yang bikin
    # batch ke AI jauh lebih kecil dan pengumpulan jawaban jauh lebih cepat.
    short_items: list[dict] = []
    for i, q in enumerate(questions):
        if q["tipe"] == "deskripsi":
            jawaban_siswa = str(body.answers.get(str(i), "") or "").strip()
            short_items.append(
                {
                    "index": i,
                    "tipe": q["tipe"],
                    "pertanyaan": q["pertanyaan"],
                    "jawaban_model": q["jawaban"],
                    "jawaban_siswa": jawaban_siswa,
                }
            )

    short_results: dict[int, dict] = {}
    if short_items:
        # Lewati panggilan AI sama sekali kalau tidak ada soal deskripsi —
        # bukan cuma soal performa, juga supaya paket tanpa uraian tidak
        # pernah gagal (502) gara-gara AI, padahal tidak butuh AI sama sekali.
        try:
            short_results = await grade_short_answers(short_items)
        except LLMError as e:
            raise HTTPException(status_code=502, detail=str(e))

    per_question = []
    total_poin_dapat = 0.0
    total_poin_maks = 0
    # Bobot poin per tipe soal dari tipe ujian; tipe tak disebut = 1 poin
    poin_cfg = (exam_row or {}).get("poin_per_tipe") or {}

    def _poin_for(tipe: str) -> int:
        p = poin_cfg.get(tipe)
        return int(p) if isinstance(p, (int, float)) and p > 0 else 1

    for i, q in enumerate(questions):
        jawaban = body.answers.get(str(i))
        tipe = q["tipe"]
        if tipe == "pilihan_ganda":
            benar = jawaban == q["jawaban"]
            verdict = "benar" if benar else "salah"
            skor = 1.0 if benar else 0.0
            umpan_balik = "Jawaban benar." if benar else "Jawaban tidak tepat."
            jawaban_benar = q["opsi"][q["jawaban"]]
            jawaban_siswa = (
                q["opsi"][jawaban] if isinstance(jawaban, int) and 0 <= jawaban <= 3 else "-"
            )
        elif tipe == "benar_salah":
            benar = jawaban == q["jawaban"]
            verdict = "benar" if benar else "salah"
            skor = 1.0 if benar else 0.0
            umpan_balik = "Jawaban benar." if benar else "Jawaban tidak tepat."
            jawaban_benar = q["jawaban"]
            jawaban_siswa = jawaban if jawaban in ("benar", "salah") else "-"
        elif tipe == "isian":
            jawaban_siswa_str = str(jawaban).strip() if jawaban else ""
            verdict, skor = grade_isian(jawaban_siswa_str, q["jawaban"])
            umpan_balik = "Jawaban benar." if verdict == "benar" else "Jawaban tidak tepat."
            jawaban_benar = q["jawaban"]
            jawaban_siswa = jawaban_siswa_str or "-"
        else:  # deskripsi — dinilai AI (lihat short_items di atas)
            hasil = short_results.get(i)
            if hasil:
                verdict = hasil["verdict"]
                skor = hasil["skor"]
                umpan_balik = hasil["umpan_balik"] or "-"
            else:
                verdict = "salah"
                skor = 0.0
                umpan_balik = "-"
            jawaban_benar = q["jawaban"]
            jawaban_siswa = str(jawaban) if jawaban else "-"
        poin_tipe = _poin_for(tipe)
        poin_dapat = round(skor * poin_tipe, 2)
        total_poin_dapat += poin_dapat
        total_poin_maks += poin_tipe
        per_question.append(
            {
                "nomor": i + 1,
                "tipe": tipe,
                "pertanyaan": q["pertanyaan"],
                "jawaban_siswa": jawaban_siswa,
                "jawaban_benar": jawaban_benar,
                "verdict": verdict,
                "skor": skor,
                "poin": poin_dapat,
                "poin_maks": poin_tipe,
                "umpan_balik": umpan_balik,
                "pembahasan": q["pembahasan"],
                "gambar": q.get("gambar"),
                "opsi": q["opsi"] if tipe == "pilihan_ganda" else None,
            }
        )

    nilai = (
        round(total_poin_dapat / total_poin_maks * 100) if total_poin_maks else 0
    )

    sb.table("attempts").update(
        {
            "answers": body.answers,
            "score": {
                "nilai": nilai,
                "poin": round(total_poin_dapat, 2),
                "poin_maks": total_poin_maks,
                "per_question": per_question,
            },
            "expired": expired,
            "submitted_at": now.isoformat(),
        }
    ).eq("id", attempt["id"]).execute()

    return {
        "quiz_id": quiz_id,
        "subject": quiz["subject"],
        "grade": quiz["grade"],
        "exam_type": (exam_row or {}).get("name", ""),
        "nilai": nilai,
        "poin": round(total_poin_dapat, 2),
        "poin_maks": total_poin_maks,
        "per_question": per_question,
        "expired": expired,
    }