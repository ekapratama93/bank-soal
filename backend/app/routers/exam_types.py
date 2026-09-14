from fastapi import APIRouter, Depends, HTTPException
from pydantic import BaseModel

from ..db_errors import is_fk_violation
from ..supabase_client import get_supabase
from .materials import require_admin

router = APIRouter(prefix="/api/exam-types", tags=["exam-types"])

VALID_QTYPES = ("pilihan_ganda", "benar_salah", "isian", "deskripsi")


class ExamTypeIn(BaseModel):
    name: str
    jumlah_soal: int | None = None
    durasi_menit: int | None = None
    tipe_soal: dict[str, int] | None = None
    poin_per_tipe: dict[str, int] | None = None


def _validate_tipe_soal(tipe_soal: dict[str, int] | None) -> dict[str, int] | None:
    """Komposisi tipe soal: {"pilihan_ganda": 10, "isian": 5, ...}.

    None = komposisi otomatis. Hasil dinormalisasi: tipe berjumlah 0 dibuang.
    """
    if tipe_soal is None:
        return None
    total = 0
    for tipe, jumlah in tipe_soal.items():
        if tipe not in VALID_QTYPES:
            raise HTTPException(
                status_code=422,
                detail=f"Tipe soal tidak valid: {tipe}. Pilihan: {', '.join(VALID_QTYPES)}",
            )
        if not isinstance(jumlah, int) or not 0 <= jumlah <= 50:
            raise HTTPException(
                status_code=422, detail=f"Jumlah soal '{tipe}' harus 0-50"
            )
        total += jumlah
    if not 5 <= total <= 50:
        raise HTTPException(
            status_code=422,
            detail="Total jumlah soal dari komposisi tipe soal harus 5-50",
        )
    return {t: j for t, j in tipe_soal.items() if j > 0}


def _validate_poin_per_tipe(poin: dict[str, int] | None) -> dict[str, int] | None:
    """Bobot poin per tipe soal: {"pilihan_ganda": 2, "deskripsi": 10, ...}.

    None = semua soal bernilai 1 poin. Tipe yang tidak disebut mengikuti default 1.
    """
    if poin is None:
        return None
    out: dict[str, int] = {}
    for tipe, nilai_poin in poin.items():
        if tipe not in VALID_QTYPES:
            raise HTTPException(
                status_code=422,
                detail=f"Tipe soal tidak valid: {tipe}. Pilihan: {', '.join(VALID_QTYPES)}",
            )
        if not isinstance(nilai_poin, int) or not 1 <= nilai_poin <= 100:
            raise HTTPException(
                status_code=422, detail=f"Poin untuk '{tipe}' harus 1-100"
            )
        out[tipe] = nilai_poin
    return out or None


@router.get("")
def list_exam_types():
    sb = get_supabase()
    res = sb.table("exam_types").select("*").order("name").execute()
    return res.data


@router.post("", status_code=201)
def create_exam_type(body: ExamTypeIn, admin: dict = Depends(require_admin)):
    if body.jumlah_soal is not None and not 5 <= body.jumlah_soal <= 50:
        raise HTTPException(status_code=422, detail="Jumlah soal harus 5-50")
    if body.durasi_menit is not None and not 10 <= body.durasi_menit <= 180:
        raise HTTPException(status_code=422, detail="Durasi harus 10-180 menit")
    sb = get_supabase()
    data = {"name": body.name.strip()}
    if body.jumlah_soal is not None:
        data["jumlah_soal"] = body.jumlah_soal
    if body.durasi_menit is not None:
        data["durasi_menit"] = body.durasi_menit
    tipe_soal = _validate_tipe_soal(body.tipe_soal)
    if tipe_soal is not None:
        data["tipe_soal"] = tipe_soal
    poin_per_tipe = _validate_poin_per_tipe(body.poin_per_tipe)
    if poin_per_tipe is not None:
        data["poin_per_tipe"] = poin_per_tipe
    res = sb.table("exam_types").insert(data).execute()
    return res.data[0]


@router.patch("/{exam_type_id}")
def update_exam_type(
    exam_type_id: str, body: ExamTypeIn, admin: dict = Depends(require_admin)
):
    sb = get_supabase()
    existing = sb.table("exam_types").select("id").eq("id", exam_type_id).execute()
    if not existing.data:
        raise HTTPException(status_code=404, detail="Tipe ujian tidak ditemukan")
    data = {"name": body.name.strip()}
    if body.jumlah_soal is not None:
        if not 5 <= body.jumlah_soal <= 50:
            raise HTTPException(status_code=422, detail="Jumlah soal harus 5-50")
        data["jumlah_soal"] = body.jumlah_soal
    if body.durasi_menit is not None:
        if not 10 <= body.durasi_menit <= 180:
            raise HTTPException(status_code=422, detail="Durasi harus 10-180")
        data["durasi_menit"] = body.durasi_menit
    tipe_soal = _validate_tipe_soal(body.tipe_soal)
    if tipe_soal is not None:
        data["tipe_soal"] = tipe_soal
    poin_per_tipe = _validate_poin_per_tipe(body.poin_per_tipe)
    if poin_per_tipe is not None:
        data["poin_per_tipe"] = poin_per_tipe
    res = sb.table("exam_types").update(data).eq("id", exam_type_id).execute()
    return res.data[0] if res.data else None


@router.delete("/{exam_type_id}", status_code=204)
def delete_exam_type(exam_type_id: str, admin: dict = Depends(require_admin)):
    sb = get_supabase()
    existing = sb.table("exam_types").select("id").eq("id", exam_type_id).execute()
    if not existing.data:
        raise HTTPException(status_code=404, detail="Tipe ujian tidak ditemukan")
    used_materials = (
        sb.table("materials").select("id").eq("exam_type_id", exam_type_id).execute()
    )
    if used_materials.data:
        raise HTTPException(
            status_code=409,
            detail="Tipe ujian masih dipakai materi. Pindahkan atau hapus materinya dulu.",
        )
    used_quizzes = (
        sb.table("quizzes").select("id").eq("exam_type_id", exam_type_id).execute()
    )
    if used_quizzes.data:
        raise HTTPException(
            status_code=409,
            detail="Tipe ujian masih dipakai kuis. Reset pool dulu sebelum menghapus.",
        )
    try:
        sb.table("exam_types").delete().eq("id", exam_type_id).execute()
    except Exception as e:
        # Sesuatu terpakai di celah antara pengecekan di atas dan delete ini —
        # DB menolak lewat foreign key, bukan 500 mentah.
        if not is_fk_violation(e):
            raise
        raise HTTPException(
            status_code=409,
            detail="Tipe ujian masih dipakai materi atau kuis. Coba lagi.",
        )
    return None