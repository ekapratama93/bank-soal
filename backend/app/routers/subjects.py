from fastapi import APIRouter, Depends, HTTPException
from pydantic import BaseModel

from ..db_errors import is_fk_violation, is_unique_violation
from ..supabase_client import get_supabase
from .materials import require_admin

router = APIRouter(prefix="/api/subjects", tags=["subjects"])


class SubjectIn(BaseModel):
    name: str


@router.get("")
def list_subjects():
    sb = get_supabase()
    res = sb.table("subjects").select("*").order("name").execute()
    return res.data


@router.post("", status_code=201)
def create_subject(body: SubjectIn, admin: dict = Depends(require_admin)):
    sb = get_supabase()
    name = body.name.strip()
    if not name:
        raise HTTPException(status_code=422, detail="Nama mata pelajaran kosong")
    existing = sb.table("subjects").select("id").eq("name", name).execute()
    if existing.data:
        raise HTTPException(status_code=409, detail="Mata pelajaran sudah ada")
    res = sb.table("subjects").insert({"name": name}).execute()
    return res.data[0]


@router.patch("/{subject_id}")
def rename_subject(subject_id: str, body: SubjectIn, admin: dict = Depends(require_admin)):
    """Admin: ganti nama mata pelajaran. materials/quizzes merujuk lewat
    subject_id, jadi rename tidak menyentuh data lain."""
    sb = get_supabase()
    existing = sb.table("subjects").select("id").eq("id", subject_id).execute()
    if not existing.data:
        raise HTTPException(status_code=404, detail="Mata pelajaran tidak ditemukan")
    name = body.name.strip()
    if not name:
        raise HTTPException(status_code=422, detail="Nama mata pelajaran kosong")
    duplicate = sb.table("subjects").select("id").eq("name", name).execute()
    if duplicate.data and duplicate.data[0]["id"] != subject_id:
        raise HTTPException(status_code=409, detail="Mata pelajaran sudah ada")
    try:
        res = sb.table("subjects").update({"name": name}).eq("id", subject_id).execute()
    except Exception as e:
        # Balapan antara pre-check di atas dan update ini — DB menolak lewat
        # unique constraint, bukan 500 mentah.
        if not is_unique_violation(e):
            raise
        raise HTTPException(status_code=409, detail="Mata pelajaran sudah ada")
    return res.data[0] if res.data else None


@router.delete("/{subject_id}", status_code=204)
def delete_subject(subject_id: str, admin: dict = Depends(require_admin)):
    sb = get_supabase()
    existing = sb.table("subjects").select("name").eq("id", subject_id).execute()
    if not existing.data:
        raise HTTPException(status_code=404, detail="Mata pelajaran tidak ditemukan")
    name = existing.data[0]["name"]

    # Cek pemakaian lewat subject_id (baru) DAN teks nama (baris yang ditulis
    # backend lama selama jendela deploy sebelum backfill).
    used_materials = (
        sb.table("materials").select("id").eq("subject_id", subject_id).execute()
    )
    if not used_materials.data:
        used_materials = sb.table("materials").select("id").eq("subject", name).execute()
    if used_materials.data:
        raise HTTPException(
            status_code=409,
            detail="Mata pelajaran masih dipakai materi. Hapus materinya dulu.",
        )
    used_quizzes = (
        sb.table("quizzes").select("id").eq("subject_id", subject_id).execute()
    )
    if not used_quizzes.data:
        used_quizzes = sb.table("quizzes").select("id").eq("subject", name).execute()
    if used_quizzes.data:
        raise HTTPException(
            status_code=409,
            detail="Mata pelajaran masih dipakai kuis. Reset pool dulu sebelum menghapus.",
        )
    try:
        sb.table("subjects").delete().eq("id", subject_id).execute()
    except Exception as e:
        # Sesuatu mulai memakai mapel di celah antara pengecekan di atas dan
        # delete ini — DB menolak lewat foreign key, bukan 500 mentah.
        if not is_fk_violation(e):
            raise
        raise HTTPException(
            status_code=409,
            detail="Mata pelajaran masih dipakai materi atau kuis. Coba lagi.",
        )
    return None